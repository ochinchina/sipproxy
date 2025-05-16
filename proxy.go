package main

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"
)

type BackendChangeEvent struct {
	action  string
	backend Backend
	parent  *RoundRobinBackend
}

type ProxyItem struct {
	sync.Mutex
	transports []ServerTransport
	viaConfig  *ViaConfig
	backend    *RoundRobinBackend
	msgHandler MessageHandler
}

type BackendWithParent struct {
	backend Backend
	parent  *RoundRobinBackend
}

type MyName struct {
	names    []string
	patterns []*regexp.Regexp
}

func NewMyName(name string) *MyName {
	tmp := strings.Split(name, ",")
	myName := &MyName{names: make([]string, 0), patterns: make([]*regexp.Regexp, 0)}

	for _, t := range tmp {
		s := strings.TrimSpace(t)
		myName.names = append(myName.names, s)
		p, err := regexp.Compile(s)
		if err == nil {
			myName.patterns = append(myName.patterns, p)
		}
	}
	return myName
}

func (p *MyName) matchAbsoluteURI(absoluteURI string) bool {
	for _, name := range p.names {
		if absoluteURI == name {
			return true
		}
	}

	for _, pattern := range p.patterns {
		if pattern.MatchString(absoluteURI) {
			return true
		}
	}
	return false

}

func (p *MyName) matchSIPURI(user string, hostName string) bool {
	for _, name := range p.names {
		pos := strings.Index(name, "@")
		if pos == -1 {
			if hostName == name {
				return true
			}
		} else {
			if hostName == name[pos+1:] && user == name[0:pos] {
				return true
			}
		}
	}

	userHost := fmt.Sprintf("%s@%s", user, hostName)
	for _, pattern := range p.patterns {
		if pattern.MatchString(userHost) {
			return true
		}

	}
	return false

}

func (p *MyName) isMyMessage(msg *Message) bool {
	requestURI, err := msg.GetRequestURI()
	if err != nil {
		zap.L().Error("Fail to find the requestURI in message", zap.String("message", msg.String()))
		return false
	}
	absoluteURI, err := requestURI.GetAbsoluteURI()
	if err == nil {
		if p.matchAbsoluteURI(absoluteURI.String()) {
			return true
		}
	}

	sipUri, err := requestURI.GetSIPURI()
	if err == nil {
		if msg.ReceivedFrom != nil && sipUri.Host == msg.ReceivedFrom.GetAddress() && sipUri.GetPort() == msg.ReceivedFrom.GetPort() {
			return true
		}
		if p.matchSIPURI(sipUri.User, sipUri.Host) {
			return true
		}
	}
	return false

}

type Proxy struct {
	myName              *MyName
	listenConfigs       []ListenConfig
	receivedSupport     bool
	keepNextHopRoute    bool
	preConfigRoute      *PreConfigRoute
	resolver            *PreConfigHostResolver
	items               []*ProxyItem
	clientTransMgr      *ClientTransportMgr
	selfLearnRoute      *SelfLearnRoute
	mustRecordRoute     bool
	msgChannel          chan *RawMessage
	connAcceptedChannel chan net.Conn
	sessionBackends     *SessionBasedBackend
}

func NewProxy(name string,
	dialogExpire int64,
	listenConfigs []ListenConfig,
	keepNextHopRoute bool,
	preConfigRoute *PreConfigRoute,
	resolver *PreConfigHostResolver,
	selfLearnRoute *SelfLearnRoute,
	receivedSupport bool,
	mustRecordRoute bool) *Proxy {

	proxy := &Proxy{myName: NewMyName(name),
		listenConfigs:       listenConfigs,
		receivedSupport:     receivedSupport,
		keepNextHopRoute:    keepNextHopRoute,
		preConfigRoute:      preConfigRoute,
		resolver:            resolver,
		items:               make([]*ProxyItem, 0),
		clientTransMgr:      nil,
		selfLearnRoute:      selfLearnRoute,
		mustRecordRoute:     mustRecordRoute,
		msgChannel:          make(chan *RawMessage, 10000),
		connAcceptedChannel: make(chan net.Conn),
		sessionBackends:     NewSessionBasedBackend(dialogExpire)}

	for _, listenConf := range listenConfigs {
		item, err := NewProxyItem(listenConf, receivedSupport, proxy, selfLearnRoute, proxy)
		if err == nil {
			proxy.items = append(proxy.items, item)
		}
	}

	connectionEstablished := func(conn net.Conn) {
		serverTransport := NewTCPServerTransportWithConn(conn, proxy.receivedSupport, selfLearnRoute, nil)
		serverTransport.Start(proxy)
	}

	proxy.clientTransMgr = NewClientTransportMgr(selfLearnRoute, connectionEstablished)

	go proxy.receiveAndProcessMessage()
	return proxy
}

func (p *Proxy) Start() error {
	for _, item := range p.items {
		err := item.Start()
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Proxy) HandleRawMessage(msg *RawMessage) {
	p.msgChannel <- msg
}

// ConnectionAccepted implement ConnectionAcceptedListener interface
func (p *Proxy) ConnectionAccepted(conn net.Conn) {
	p.connAcceptedChannel <- conn
}

func (p *Proxy) receiveAndProcessMessage() {
	for {
		select {
		case rawMsg := <-p.msgChannel:
			msg, err := p.handleRawMessage(rawMsg)
			if err == nil {
				p.handleMessage(rawMsg.From.GetProtocol(), msg, rawMsg.Backend, rawMsg.Via)
				p.handleSession(msg)
			}

		case conn := <-p.connAcceptedChannel:
			host, port, err := net.SplitHostPort(conn.RemoteAddr().String())
			if err == nil {
				port_i, err := strconv.Atoi(port)
				if err == nil {
					trans, err := p.clientTransMgr.GetTransport("tcp", host, port_i, "")
					if err == nil {
						trans.primary, _ = NewTCPClientTransportWithConn(conn)
					}
				}
			}
		}
	}
}

func (p *Proxy) handleRawMessage(rawMessage *RawMessage) (*Message, error) {
	msg := rawMessage.Message
	msg.ReceivedFrom = rawMessage.From
	//if msg.IsRequest() && !p.isBackendAddr(rawMessage.PeerAddr) {
	if msg.IsRequest() {
		p.selfLearnRoute.AddRoute(rawMessage.PeerAddr, rawMessage.From)
		msg.ForEachViaParam(func(viaParam *ViaParam) {
			p.selfLearnRoute.AddRoute(viaParam.Host, rawMessage.From)
		})
	}
	// set the received parameters
	if msg.IsRequest() && rawMessage.ReceivedSupport {
		msg.SetReceived(rawMessage.PeerAddr, rawMessage.PeerPort)
	}
	if msg.IsRequest() && rawMessage.TcpConn != nil {
		host, port, _, err := p.getNextReponseHop(msg)
		if strings.HasPrefix(host, "[") {
			host = host[1 : len(host)-1]
		}
		zap.L().Info("receive a message from tcp", zap.String("host", host), zap.Int("port", port))
		if err == nil {
			// create a transport for transaction in tcp connection
			transId, err := msg.GetClientTransaction()
			if err == nil {
				trans, err := p.clientTransMgr.GetTransport("tcp", host, port, transId)
				if err == nil {
					trans.primary, _ = NewTCPClientTransportWithConn(rawMessage.TcpConn)
				}
			}
		}
	}
	// The proxy will inspect the URI in the topmost Route header
	// field value.  If it indicates this proxy, the proxy removes it
	// from the Route header field (this route node has been
	// reached).

	p.tryRemoveTopRoute(rawMessage)
	return msg, nil
}

func (p *Proxy) tryRemoveTopRoute(rawMessage *RawMessage) {
	msg := rawMessage.Message

	route, err := msg.GetRoute()
	if err != nil {
		return
	}
	routeParam, err := route.GetRouteParam(0)
	if err != nil {
		return
	}
	sipUri, err := routeParam.GetAddress().GetAddress().GetSIPURI()

	if err != nil {
		return
	}

	myAddr := rawMessage.From.GetAddress()
	myPort := rawMessage.From.GetPort()

	if sipUri.GetPort() == myPort && p.isSameAddress(sipUri.Host, myAddr) {
		zap.L().Info("remove top route item because the top item is my address", zap.String("route-param", routeParam.String()))
		msg.PopRoute()
	}
}

// isSameAddress check if the two addresses are the same
// if the two addresses are the same, return true, otherwise return false
func (p *Proxy) isSameAddress(addr1 string, addr2 string) bool {
	if addr1 == addr2 {
		return true
	}

	ip1, err := p.resolver.GetIp(addr1)
	if err != nil {
		return false
	}

	ip2, err := p.resolver.GetIp(addr2)

	if err != nil {
		return false
	}

	return ip1 == ip2

}

func (p *Proxy) handleSession(msg *Message) {
	if !msg.IsResponse() {
		return
	}

	if serverTransId, err := msg.GetServerTransaction(); err == nil {
		if backend, ok := p.sessionBackends.backends[serverTransId]; ok {
			addr := backend.backend.GetAddress()
			if msg.IsFinalResponse() {
				zap.L().Info("transaction is finished, remove backend by transId", zap.String("backendAddr", addr), zap.String("serverTransaction", serverTransId))
				p.sessionBackends.RemoveSession(serverTransId)
			}

			if method, err := msg.GetMethod(); err == nil {
				switch method {
				case "INVITE":
					dialog, _ := msg.GetDialog()
					if dialog != "" {
						zap.L().Info("dialog is bind to backend", zap.String("backendAddr", addr), zap.String("dialog", dialog))
						p.sessionBackends.AddBackend(dialog, backend.backend, msg.GetExpires(0))
					}
				case "BYE":
					dialog, _ := msg.GetDialog()
					if dialog != "" {
						zap.L().Info("dialog is closed", zap.String("backendAddr", addr), zap.String("dialog", dialog))
						p.sessionBackends.RemoveSession(dialog)
					}

				}
			}
		}
	}

}

func (p *Proxy) handleMessage(protocol string, msg *Message, backend Backend, viaConfig *ViaConfig) {
	zap.L().Debug("Received a message", zap.String("host", msg.ReceivedFrom.GetAddress()), zap.Int("port", msg.ReceivedFrom.GetPort()), zap.String("message", msg.String()))
	if msg.IsRequest() {
		host, port, transport, err := p.getNextRequestHop(msg)
		if err == nil {
			zap.L().Info("Get next hop", zap.String("host", host), zap.Int("port", port), zap.String("transport", transport))
			serverTrans, ok := p.selfLearnRoute.GetRoute(host, protocol)

			if ok {
				p.addVia(msg, serverTrans)
				p.addRecordRoute(msg, serverTrans)
			}
			p.sendMessage(host, port, transport, msg)
		} else if p.myName.isMyMessage(msg) {
			zap.L().Info("it is my request")
			p.sendToBackend(protocol, msg, backend, viaConfig)
		} else {
			zap.L().Error("Not my message, fail to route the message")
		}
	} else {
		msg.PopVia()
		host, port, transport, err := p.getNextReponseHop(msg)
		// if the response of SUBSCRIBE to the backend
		if method, err := msg.GetMethod(); err == nil && method == "SUBSCRIBE" {
			if serverTransId, err := msg.GetServerTransaction(); err == nil {
				if tmpBackend, ok := p.sessionBackends.backends[serverTransId]; ok {
					if dialog, err := msg.GetDialog(); err == nil {
						zap.L().Info("bind the dialog to the response", zap.String("dialog", dialog), zap.String("backend", tmpBackend.backend.GetAddress()))

						p.sessionBackends.AddBackend(dialog, tmpBackend.backend, msg.GetExpires(0))
					}
				}
			}
		}
		if err != nil {
			zap.L().Error("Fail to find the next hop for response", zap.String("message", msg.String()))
		} else {
			p.sendMessage(host, port, transport, msg)
		}
	}
}

func (p *Proxy) addVia(msg *Message, transport ServerTransport) (*Via, error) {
	via, err := CreateVia(transport.GetProtocol(), transport.GetAddress(), transport.GetPort())
	if err == nil {
		msg.AddVia(via)
	}
	return via, nil
}

func (p *Proxy) addRecordRoute(msg *Message, transport ServerTransport) {
	// if no Record-Route header is found and the mustRecordRoute is false, no need to add Record-Route header
	if _, err := msg.GetHeader("Record-Route"); err != nil && !p.mustRecordRoute {
		return
	}

	msg.AddRecordRoute(CreateRecordRoute(transport.GetAddress(), transport.GetPort()))
}

func (p *Proxy) sendToBackend(protocol string, msg *Message, preferBackend Backend, viaConfig *ViaConfig) {

	backendItem := p.findBackendProxyItem(protocol)
	if backendItem == nil && preferBackend == nil {
		zap.L().Error("Fail to find the backend for my message", zap.String("message", msg.String()))
	} else {
		transId, _ := msg.GetServerTransaction()
		// find the backend by dialog or transaction
		backend, transport, err := p.findBackendByDialog(protocol, msg)

		if err != nil {
			// if no backend is found by dialog, find the backend by transaction
			backend, transport, err = p.findBackendByTransaction(protocol, msg)
		}

		if err != nil && viaConfig != nil {
			transport, _ = p.findTransportByViaConfig(viaConfig)
		}

		if backend == nil && preferBackend != nil {
			backend = preferBackend
			//transport, _ = p.findTransportByBackendAddr(backend.GetAddress(), protocol)
		}

		if transport == nil && backend != nil {
			backendAddrs := getAllBackendAddresses(backend)
			for _, addr := range backendAddrs {
				transport, err = p.findTransportByBackendAddr(addr, protocol)
				if err == nil {
					break
				}
			}
		}
		if backend == nil && backendItem != nil {
			backend = backendItem.backend
			transport = backendItem.transports[0]
		}
		if transport == nil && backendItem != nil {
			transport, _ = backendItem.FindTransport(func(t ServerTransport) bool {
				return t.GetProtocol() == protocol
			})
			if transport == nil {
				transport = backendItem.transports[0]
			}
		}
		if transport != nil {
			p.addVia(msg, transport)
			p.addRecordRoute(msg, transport)
		}
		if backend == nil {
			zap.L().Error("Fail to find backend for my message", zap.String("message", msg.String()))
			return
		}
		usedBackend, err := backend.Send(msg)
		if err == nil {
			zap.L().Debug("succeed to send the message to the backend", zap.String("backend", usedBackend.GetAddress()), zap.String("message", msg.String()))
			if len(transId) > 0 {
				// bind the backend with the transaction
				zap.L().Info("bind server transaction with backend", zap.String("trandId", transId), zap.String("backend", usedBackend.GetAddress()))
				p.sessionBackends.AddBackend(transId, usedBackend, msg.GetExpires(0))
			}
		} else {
			zap.L().Error("Fail to send the message to the backend", zap.String("backend", backend.GetAddress()), zap.String("message", msg.String()))
		}
	}
}

// findBackendByDialog find the backend information by the message dialog
func (p *Proxy) findBackendByDialog(protocol string, msg *Message) (Backend, ServerTransport, error) {
	method, err := msg.GetMethod()
	if err != nil {
		return nil, nil, err
	}

	// no dialog for INVITE and SUBSCRIBE message because they initialize the dialog
	if method == "INVITE" || method == "SUBSCRIBE" {
		return nil, nil, fmt.Errorf("no dialog for request %s", method)
	}
	dialog, err := msg.GetDialog()

	if err != nil {
		return nil, nil, err
	}

	backend, err := p.sessionBackends.GetBackend(dialog)
	var transport ServerTransport = nil
	if err == nil {
		zap.L().Info("succeed to find backend by dialog", zap.String("backendAddr", backend.GetAddress()), zap.String("dialog", dialog))
		transport, _ = p.findTransportByBackendAddr(backend.GetAddress(), protocol)
	}
	// remove the SUBSCRIBE initialized dialog if the Subscription-State is terminated in NOTIFY message
	if method == "NOTIFY" && backend != nil {
		if v, err := msg.GetHeaderValue("Subscription-State"); err == nil {
			if s, ok := v.(string); ok && s == "terminated" {
				zap.L().Info("remove the dialog because the Subscription-State is terminated in NOTIFY message", zap.String("dialog", dialog))
				p.sessionBackends.RemoveSession(dialog)
			}
		}
	}
	return backend, transport, err
}

func (p *Proxy) findBackendByTransaction(protocol string, msg *Message) (Backend, ServerTransport, error) {
	transId, err := msg.GetServerTransaction()
	if err != nil {
		return nil, nil, err
	}

	backend, err := p.sessionBackends.GetBackend(transId)
	if err == nil {
		zap.L().Info("succeed to find backend by server transaction", zap.String("backendAddr", backend.GetAddress()), zap.String("transId", transId))
		transport, _ := p.findTransportByBackendAddr(backend.GetAddress(), protocol)
		return backend, transport, nil
	} else {
		return nil, nil, fmt.Errorf("fail to find backend by transaction %s", transId)
	}
}

func (p *Proxy) findTransportByViaConfig(viaConfig *ViaConfig) (ServerTransport, error) {
	if viaConfig == nil {
		return nil, fmt.Errorf("no via config")
	}
	for _, item := range p.items {
		transport, err := item.FindTransport(func(transport ServerTransport) bool {
			return transport.GetProtocol() == viaConfig.Protocol && transport.GetAddress() == viaConfig.Address && transport.GetPort() == viaConfig.Port
		})
		if err == nil {
			zap.L().Info("succeed to find transport by via config", zap.String("viaConfig", viaConfig.String()))
			return transport, err
		}
	}
	return nil, fmt.Errorf("fail to find transport by via config")
}

func (p *Proxy) findTransportByBackendAddr(addr string, preferProtocol string) (ServerTransport, error) {
	for _, item := range p.items {
		transport, err := item.FindTransport(func(serverTransport ServerTransport) bool {
			return serverTransport.GetAddress() == addr
		})

		if err == nil {
			return transport, err
		}
	}

	for _, item := range p.items {
		transport, err := item.FindTransport(func(serverTransport ServerTransport) bool {
			return serverTransport.GetProtocol() == preferProtocol
		})
		if err == nil {
			return transport, err
		}
	}

	return nil, fmt.Errorf("fail to find backend by %s", addr)
}

func (p *Proxy) findBackendProxyItem(protocol string) *ProxyItem {
	for _, item := range p.items {
		if item.backend != nil && strings.HasPrefix(item.backend.GetAddress(), protocol) {
			return item
		}
	}
	return nil
}

func (p *Proxy) getNextRequestHop(msg *Message) (host string, port int, transport string, err error) {
	host, port, transport, err = p.getNextRequestHopByRoute(msg)
	if err == nil {
		return host, port, transport, err
	}
	return p.getNextRequestHopByConfig(msg)

}

func (p *Proxy) getNextRequestHopByConfig(msg *Message) (host string, port int, transport string, err error) {
	to, err := msg.GetTo()
	if err != nil {
		return "", 0, "", fmt.Errorf("no To header in message")
	}
	destHost, err := to.GetHost()
	if err != nil {
		return "", 0, "", fmt.Errorf("fail to find Host in To header of message")
	}
	transport, host, port, err = p.preConfigRoute.FindRoute(destHost)
	return
}

func (P *Proxy) getNextRequestHopByRoute(msg *Message) (host string, port int, transport string, err error) {

	route, err := msg.GetRoute()
	if err != nil {
		return
	}
	routeParam, err := route.GetRouteParam(0)
	if err != nil {
		return
	}
	if !P.keepNextHopRoute {
		msg.PopRoute()
	}
	addr := routeParam.GetAddress().GetAddress()
	if addr.IsSIPURI() {
		sipUri, _ := addr.GetSIPURI()
		transport = sipUri.GetTransport()
		host = sipUri.Host
		port = sipUri.GetPort()
	} else {
		err = fmt.Errorf("address %v is not a sip URI", addr)
	}
	return
}

func (p *Proxy) getNextReponseHop(msg *Message) (host string, port int, protocol string, err error) {
	via, err := msg.GetVia()
	if err != nil {
		return
	}
	viaParam, err := via.GetParam(0)
	if err != nil {
		return
	}
	protocol = viaParam.Transport
	host, err = viaParam.GetReceived()
	if err == nil {
		port, err = viaParam.GetRPort()
		if err != nil {
			port = viaParam.GetPort()
		}
		err = nil
	} else {
		host = viaParam.Host
		port = viaParam.GetPort()
		err = nil
	}
	return
}

func (p *Proxy) findClientTransport(host string, port int, protocol string, transId string) (ClientTransport, error) {
	if serverTrans, ok := p.selfLearnRoute.GetRoute(host, protocol); ok {
		udpServerTrans, ok := serverTrans.(*UDPServerTransport)
		if ok {
			remoteAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, strconv.Itoa(port)))
			if err == nil {
				return NewUDPClientTransportWithConn(udpServerTrans.conn, remoteAddr)
			}
		}
	}

	return p.clientTransMgr.GetTransport(protocol, host, port, transId)
}

func (p *Proxy) sendMessage(host string, port int, protocol string, msg *Message) {
	ip, err := p.resolver.GetIp(host)
	if err != nil {
		ip = host
	}
	transId, err := msg.GetClientTransaction()
	if err != nil {
		transId = ""
	}
	t, err := p.findClientTransport(ip, port, protocol, transId)
	if err == nil {
		if msg.IsFinalResponse() {
			// remove the transport from the client transaction manager
			p.clientTransMgr.RemoveTransport(protocol, host, port, transId)
		}
		t.Send(msg)
	} else {
		zap.L().Error("Fail to find the transport", zap.String("host", host), zap.Int("port", port), zap.String("transport", protocol), zap.String("message", msg.String()))
	}
}

// NewProxyItem create a sip proxy
func NewProxyItem(listenConfig ListenConfig,
	receivedSupport bool,
	connAcceptedListener ConnectionAcceptedListener,
	selfLearnRoute *SelfLearnRoute,
	msgHandler MessageHandler) (*ProxyItem, error) {
	zap.L().Info("NewProxyItem", zap.Any("listenConfig", listenConfig), zap.Bool("receivedSupport", receivedSupport))

	proxyItem := &ProxyItem{transports: make([]ServerTransport, 0),
		viaConfig:  createViaConfig(listenConfig.Via),
		backend:    nil,
		msgHandler: msgHandler,
	}

	connectionEstablished := func(conn net.Conn) {
		zap.L().Info("tcp connection established", zap.String("remoteAddr", conn.RemoteAddr().String()), zap.String("localAddr", conn.LocalAddr().String()))
		proxyItem.connectionEstablished(conn, receivedSupport, selfLearnRoute)
	}

	proxyItem.backend, _ = CreateRoundRobinBackend(listenConfig.Backends, connectionEstablished)

	if listenConfig.UdpPort > 0 {
		udpServerTrans, err := NewUDPServerTransport(listenConfig.Address, listenConfig.UdpPort, receivedSupport, selfLearnRoute, proxyItem.viaConfig, proxyItem.backend)
		if err == nil {
			proxyItem.transports = append(proxyItem.transports, udpServerTrans)
		}
	}

	if listenConfig.TcpPort > 0 {
		proxyItem.transports = append(proxyItem.transports, NewTCPServerTransport(listenConfig.Address, listenConfig.TcpPort, receivedSupport, connAcceptedListener, selfLearnRoute, proxyItem.viaConfig, proxyItem.backend))
	}

	return proxyItem, nil
}

func (p *ProxyItem) FindTransport(cond func(serverTransport ServerTransport) bool) (ServerTransport, error) {
	for _, transport := range p.transports {
		if cond(transport) {
			return transport, nil
		}
	}
	return nil, fmt.Errorf("fail to find transport")

}

func (p *ProxyItem) connectionEstablished(conn net.Conn, receivedSupport bool, selfLearnRoute *SelfLearnRoute) {
	p.Lock()
	defer p.Unlock()

	p.removeExitServerTransports()
	trans := NewTCPServerTransportWithConn(conn, receivedSupport, selfLearnRoute, p.backend)
	p.transports = append(p.transports, trans)
	trans.Start(p.msgHandler)

}

func (p *ProxyItem) removeExitServerTransports() {
	ok_transports := make([]ServerTransport, 0)

	for _, transport := range p.transports {
		if !transport.IsExit() {
			ok_transports = append(ok_transports, transport)
		}
	}

	if len(ok_transports) != len(p.transports) {
		p.transports = ok_transports
	}

}

func (p *ProxyItem) Start() error {
	for _, trans := range p.transports {
		err := trans.Start(p.msgHandler)
		if err != nil {
			return err
		}
	}
	return nil
}

