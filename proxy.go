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
	backend    *RoundRobinBackend
	dests      []string
	defRoute   bool
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
	myName               *MyName
	keepNextHopRoute     bool
	preConfigRoute       *PreConfigRoute
	resolver             *PreConfigHostResolver
	items                []*ProxyItem
	clientTransMgr       *ClientTransportMgr
	selfLearnRoute       *SelfLearnRoute
	mustRecordRoute      bool
	msgChannel           chan *RawMessage
	backendChangeChannel chan *BackendChangeEvent
	connAcceptedChannel  chan net.Conn
	backends             map[string]*BackendWithParent
	dialogBasedBackends  *DialogBasedBackend
}

func NewProxy(name string,
	dialogExpire int64,
	keepNextHopRoute bool,
	preConfigRoute *PreConfigRoute,
	resolver *PreConfigHostResolver,
	selfLearnRoute *SelfLearnRoute,
	receivedSupport bool,
	mustRecordRoute bool) *Proxy {

	proxy := &Proxy{myName: NewMyName(name),
		keepNextHopRoute:     keepNextHopRoute,
		preConfigRoute:       preConfigRoute,
		resolver:             resolver,
		items:                make([]*ProxyItem, 0),
		clientTransMgr:       nil,
		selfLearnRoute:       selfLearnRoute,
		mustRecordRoute:      mustRecordRoute,
		msgChannel:           make(chan *RawMessage, 10000),
		backendChangeChannel: make(chan *BackendChangeEvent, 1000),
		connAcceptedChannel:  make(chan net.Conn),
		backends:             make(map[string]*BackendWithParent),
		dialogBasedBackends:  NewDialogBasedBackend(dialogExpire)}

	connectionEstablished := func(conn net.Conn) {
		serverTransport := NewTCPServerTransportWithConn(conn, receivedSupport, selfLearnRoute)
		serverTransport.Start(proxy)
	}

	proxy.clientTransMgr = NewClientTransportMgr(connectionEstablished)

	go proxy.receiveAndProcessMessage()
	return proxy
}

func (p *Proxy) AddItem(item *ProxyItem) {
	if item.backend != nil {
		item.backend.AddBackendChangeListener(p)
	}
	p.items = append(p.items, item)
}

func (p *Proxy) Start() error {
	for _, item := range p.items {
		err := item.start()
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

func (p *Proxy) isBackendAddr(addr string) bool {
	_, ok := p.backends[addr]
	return ok
}
func (p *Proxy) receiveAndProcessMessage() {

	for {
		select {
		case rawMsg := <-p.msgChannel:
			msg, err := p.handleRawMessage(rawMsg)
			if err == nil {
				p.handleDialog(rawMsg.PeerAddr, rawMsg.PeerPort, msg)
				p.HandleMessage(msg)
			}
		case backendChangeEvent := <-p.backendChangeChannel:
			backend := backendChangeEvent.backend
			switch backendChangeEvent.action {
			case "add":
				p.backends[backend.GetAddress()] = &BackendWithParent{backend: backend, parent: backendChangeEvent.parent}
			case "remove":
				delete(p.backends, backend.GetAddress())
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
	if msg.IsRequest() && !p.isBackendAddr(rawMessage.PeerAddr) {
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

func (p *Proxy) getBackendOfResponse(addr string, msg *Message) (Backend, error) {
	backendWithParent, ok := p.backends[addr]
	if ok {
		return backendWithParent.backend, nil
	}

	zap.L().Warn("Fail to find backend by address", zap.String("backendAddr", addr))

	transId, err := msg.GetClientTransaction()
	if err != nil {
		return nil, err
	}
	backend, err := p.dialogBasedBackends.GetBackend(transId)
	if err != nil {
		zap.L().Warn("Fail to find backend by transaction", zap.String("clientTransaction", transId))
	}

	if msg.IsFinalResponse() {
		p.dialogBasedBackends.RemoveDialog(transId)
	}
	return backend, err

}

func (p *Proxy) handleDialog(peerAddr string, peerPort int, msg *Message) {
	if !msg.IsResponse() {
		return
	}

	addr := net.JoinHostPort(peerAddr, strconv.Itoa(peerPort))
	backend, err := p.getBackendOfResponse(addr, msg)
	if err != nil {
		return
	}
	if method, err := msg.GetMethod(); err == nil {
		switch method {
		case "INVITE":
			dialog, _ := msg.GetDialog()
			if dialog != "" {
				zap.L().Info("dialog is bind to backend", zap.String("backendAddr", addr), zap.String("dialog", dialog))
				p.dialogBasedBackends.AddBackend(dialog, backend, msg.GetExpires(0))
			}
		case "BYE":
			dialog, _ := msg.GetDialog()
			if dialog != "" {
				zap.L().Info("dialog is closed", zap.String("backendAddr", addr), zap.String("dialog", dialog))
				p.dialogBasedBackends.RemoveDialog(dialog)
			}
		}
	}
}

func (p *Proxy) HandleMessage(msg *Message) {
	zap.L().Debug("Received a message", zap.String("host", msg.ReceivedFrom.GetAddress()), zap.Int("port", msg.ReceivedFrom.GetPort()), zap.String("message", msg.String()))
	if msg.IsRequest() {
		host, port, transport, err := p.getNextRequestHop(msg)
		if err == nil {
			zap.L().Info("Get next hop", zap.String("host", host), zap.Int("port", port), zap.String("transport", transport))
			serverTrans, ok := p.selfLearnRoute.GetRoute(host)

			if ok {
				p.addVia(msg, serverTrans)
				p.addRecordRoute(msg, serverTrans)
			}
			p.sendMessage(host, port, transport, msg)
		} else if p.myName.isMyMessage(msg) {
			zap.L().Info("it is my request")
			p.sendToBackend(msg)
		} else {
			zap.L().Error("Not my message, fail to route the message")
		}
	} else {
		msg.PopVia()
		host, port, transport, err := p.getNextReponseHop(msg)
		// if the response of SUBSCRIBE to the backend
		if method, err := msg.GetMethod(); err == nil && method == "SUBSCRIBE" {
			addr := fmt.Sprintf("%s:%d", host, port)
			if backendWithParent, ok := p.backends[addr]; ok {
				if dialog, err := msg.GetDialog(); err == nil {
					zap.L().Info("bind the dialog to the response", zap.String("dialog", dialog), zap.String("backend", backendWithParent.backend.GetAddress()))

					p.dialogBasedBackends.AddBackend(dialog, backendWithParent.backend, msg.GetExpires(0))
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

// HandleBackendAdded implements the method in BackendChangeListener
func (p *Proxy) HandleBackendAdded(backend Backend, parent *RoundRobinBackend) {
	p.backendChangeChannel <- &BackendChangeEvent{action: "add", backend: backend, parent: parent}
}

// HandleBackendRemoved implements the method in BackendChangeListener
func (p *Proxy) HandleBackendRemoved(backend Backend, parent *RoundRobinBackend) {
	p.backendChangeChannel <- &BackendChangeEvent{action: "remove", backend: backend, parent: parent}
}

func (p *Proxy) addVia(msg *Message, transport ServerTransport) (*Via, error) {
	viaParam := CreateViaParam(transport.GetProtocol(), transport.GetAddress(), transport.GetPort())
	branch, err := CreateBranch()
	if err != nil {
		zap.L().Error("Fail to create branch parameter")
		return nil, err
	}
	viaParam.SetBranch(branch)
	via := NewVia()
	via.AddViaParam(viaParam)
	msg.AddVia(via)
	return via, nil
}

func (p *Proxy) addRecordRoute(msg *Message, transport ServerTransport) {
	// if no Record-Route header is found and the mustRecordRoute is false, no need to add Record-Route header
	if _, err := msg.GetHeader("Record-Route"); err != nil && !p.mustRecordRoute {
		return
	}
	addr := NewAddrSpec()
	addr.sipURI = &SIPURI{Scheme: "sip", Host: transport.GetAddress(), port: transport.GetPort()}
	addr.sipURI.AddParameter("lr", "")
	nameAddr := &NameAddr{DisplayName: "", Addr: addr}
	recRoute := NewRecRoute(nameAddr)
	recordRoute := NewRecordRoute()
	recordRoute.AddRecRoute(recRoute)
	msg.AddRecordRoute(recordRoute)

}

func (p *Proxy) sendToBackend(msg *Message) {

	backendItem := p.findBackendProxyItem()
	if backendItem == nil {
		zap.L().Error("Fail to find the backend for my message", zap.String("message", msg.String()))
	} else {
		backend, transport, err := p.findBackendByDialog(msg)
		if err != nil {
			backend = backendItem.backend
			transport = backendItem.transports[0]
		}
		if transport == nil {
			transport = backendItem.transports[0]
		}
		p.addVia(msg, transport)
		p.addRecordRoute(msg, transport)
		err = backend.Send(msg)
		if err == nil {
			transId, err := msg.GetClientTransaction()
			if err == nil {
				zap.L().Debug("bind client transaction with backend", zap.String("trandId", transId), zap.String("backend", backend.GetAddress()))
				p.dialogBasedBackends.AddBackend(transId, backend, msg.GetExpires(0))
			}
		} else {
			zap.L().Error("Fail to send the message to the backend", zap.String("backend", backend.GetAddress()), zap.String("message", msg.String()))
		}
	}
}

// findBackendByDialog find the backend information by the message dialog
func (p *Proxy) findBackendByDialog(msg *Message) (Backend, ServerTransport, error) {
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

	backend, err := p.dialogBasedBackends.GetBackend(dialog)
	var transport ServerTransport = nil
	if err == nil {
		zap.L().Info("find backend by dialog", zap.String("backendAddr", backend.GetAddress()), zap.String("dialog", dialog))
		transport, _ = p.findTransportByBackendAddr(backend.GetAddress())
	} else {
		zap.L().Warn("Fail to find backend by dialog", zap.String("dialog", dialog), zap.String("error", err.Error()))
	}
	// remove the SUBSCRIBE initialized dialog if the Subscription-State is terminated in NOTIFY message
	if method == "NOTIFY" {
		if v, err := msg.GetHeaderValue("Subscription-State"); err == nil {
			if s, ok := v.(string); ok && s == "terminated" {
				zap.L().Info("remove the dialog", zap.String("dialog", dialog))
				p.dialogBasedBackends.RemoveDialog(dialog)
			}
		}
	}
	return backend, transport, err
}

func (p *Proxy) findTransportByBackendAddr(addr string) (ServerTransport, error) {
	if backendWithParent, ok := p.backends[addr]; ok {
		proxyItem := p.findProxyItemByRoundrobinBackend(backendWithParent.parent)
		if proxyItem == nil {
			zap.L().Warn("Fail to find backend by address", zap.String("backendAddr", addr))
		} else {
			return proxyItem.transports[0], nil
		}
	}
	return nil, fmt.Errorf("fail to find backend by %s", addr)

}

func (p *Proxy) findProxyItemByRoundrobinBackend(rrBackend *RoundRobinBackend) *ProxyItem {
	for _, item := range p.items {
		if item.backend == rrBackend {
			return item
		}
	}
	return nil
}

func (p *Proxy) findBackendProxyItem() *ProxyItem {
	for _, item := range p.items {
		if item.backend != nil {
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

func (p *Proxy) getNextReponseHop(msg *Message) (host string, port int, transport string, err error) {
	via, err := msg.GetVia()
	if err != nil {
		return
	}
	viaParam, err := via.GetParam(0)
	if err != nil {
		return
	}
	transport = viaParam.Transport
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

func (p *Proxy) findClientTransport(host string, port int, transport string, transId string) (ClientTransport, error) {
	trans, err := p.clientTransMgr.GetTransport(transport, host, port, transId)
	if err == nil && trans.primary == nil {
		serverTrans, ok := p.selfLearnRoute.GetRoute(host)
		if ok {
			udpServerTrans, ok := serverTrans.(*UDPServerTransport)
			if ok {
				remoteAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, strconv.Itoa(port)))
				if err == nil && ok {
					trans.primary, _ = NewUDPClientTransportWithConn(udpServerTrans.conn, remoteAddr)
				}
			}
		}
	}
	return trans, err
}

func (p *Proxy) sendMessage(host string, port int, transport string, msg *Message) {
	ip, err := p.resolver.GetIp(host)
	if err != nil {
		ip = host
	}
	transId, err := msg.GetClientTransaction()
	if err != nil {
		transId = ""
	}
	t, err := p.findClientTransport(ip, port, transport, transId)
	if err == nil {
		if msg.IsFinalResponse() {
			p.clientTransMgr.RemoveTransport(transport, host, port, transId)
		}
		t.Send(msg)
	} else {
		zap.L().Error("Fail to find the transport", zap.String("host", host), zap.Int("port", port), zap.String("transport", transport), zap.String("message", msg.String()))
	}
}

// NewProxyItem create a sip proxy
func NewProxyItem(address string,
	udpPort int,
	tcpPort int,
	backends []string,
	dests []string,
	receivedSupport bool,
	defRoute bool,
	connAcceptedListener ConnectionAcceptedListener,
	selfLearnRoute *SelfLearnRoute,
	msgHandler MessageHandler) (*ProxyItem, error) {
	zap.L().Info("NewProxyItem", zap.Int("udpPort", udpPort), zap.Int("tcpPort", tcpPort), zap.String("backends", strings.Join(backends, ",")), zap.String("dests", strings.Join(dests, ",")), zap.Bool("receivedSupport", receivedSupport), zap.Bool("defRoute", defRoute))
	transports := make([]ServerTransport, 0)
	if udpPort > 0 {
		udpServerTrans, err := NewUDPServerTransport(address, udpPort, receivedSupport, selfLearnRoute)
		if err == nil {
			transports = append(transports, udpServerTrans)
		}
	}
	if tcpPort > 0 {
		transports = append(transports, NewTCPServerTransport(address, tcpPort, receivedSupport, connAcceptedListener, selfLearnRoute))
	}

	proxyItem := &ProxyItem{transports: transports,
		backend:    nil,
		dests:      dests,
		defRoute:   defRoute,
		msgHandler: msgHandler,
	}

	connectionEstablished := func(conn net.Conn) {
		zap.L().Info("tcp connection established", zap.String("remoteAddr", conn.RemoteAddr().String()), zap.String("localAddr", conn.LocalAddr().String()))
		proxyItem.connectionEstablished(conn, receivedSupport, selfLearnRoute)
	}

	proxyItem.backend, _ = CreateRoundRobinBackend(net.JoinHostPort(address, "0"), backends, connectionEstablished)

	return proxyItem, nil
}

func (p *ProxyItem) connectionEstablished(conn net.Conn, receivedSupport bool, selfLearnRoute *SelfLearnRoute) {
	p.Lock()
	defer p.Unlock()

	p.removeExitServerTransports()
	trans := NewTCPServerTransportWithConn(conn, receivedSupport, selfLearnRoute)
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

func (p *ProxyItem) start() error {
	for _, trans := range p.transports {
		err := trans.Start(p.msgHandler)
		if err != nil {
			return err
		}
	}
	return nil
}

