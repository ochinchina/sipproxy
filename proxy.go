package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"strconv"
	"strings"
)

type BackendChangeEvent struct {
	action  string
	backend Backend
	parent  *RoundRobinBackend
}

type ProxyItem struct {
	transports []ServerTransport
	backend    *RoundRobinBackend
	dests      []string
	defRoute   bool
}

type BackendWithParent struct {
	backend Backend
	parent  *RoundRobinBackend
}

type Proxy struct {
	names                []string
	preConfigRoute       *PreConfigRoute
	resolver             *PreConfigHostResolver
	items                []*ProxyItem
	clientTransMgr       *ClientTransportMgr
	selfLearnRoute       *SelfLearnRoute
	msgChannel           chan *RawMessage
	backendChangeChannel chan *BackendChangeEvent
	connAcceptedChannel  chan net.Conn
	backends             map[string]*BackendWithParent
	dialogBasedBackends  *DialogBasedBackend
}

func NewProxy(name string,
	preConfigRoute *PreConfigRoute,
	resolver *PreConfigHostResolver,
	selfLearnRoute *SelfLearnRoute) *Proxy {
	proxy := &Proxy{names: strings.Split(name, ","),
		preConfigRoute:       preConfigRoute,
		resolver:             resolver,
		items:                make([]*ProxyItem, 0),
		clientTransMgr:       NewClientTransportMgr(),
		selfLearnRoute:       selfLearnRoute,
		msgChannel:           make(chan *RawMessage, 10000),
		backendChangeChannel: make(chan *BackendChangeEvent, 1000),
		connAcceptedChannel:  make(chan net.Conn),
		backends:             make(map[string]*BackendWithParent),
		dialogBasedBackends:  NewDialogBasedBackend(600)}
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
		err := p.startItem(item)
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
					trans, err := p.clientTransMgr.GetTransport("tcp", host, port_i)
					if err == nil {
						trans.primary, _ = NewTCPClientTransportWithConn(conn)
					}
				}
			}
		}
	}

}
func (p *Proxy) startItem(item *ProxyItem) error {
	return item.start(p)
}

func (p *Proxy) isMyMessage(msg *Message) bool {
	requestURI, err := msg.GetRequestURI()
	if err != nil {
		log.Error("Fail to find the requestURI in message:", msg)
		return false
	}
	absoluteURI, err := requestURI.GetAbsoluteURI()
	if err == nil {
		for _, name := range p.names {
			if absoluteURI.String() == name {
				return true
			}
		}
	}

	sipUri, err := requestURI.GetSIPURI()
	if err == nil {
		if sipUri.Host == msg.ReceivedFrom.GetAddress() && sipUri.GetPort() == msg.ReceivedFrom.GetPort() {
			return true
		}
		for _, name := range p.names {
			pos := strings.Index(name, "@")
			if pos == -1 {
				if sipUri.Host == name {
					return true
				}
			} else {
				if sipUri.Host == name[pos+1:] && sipUri.User == name[0:pos] {
					return true
				}
			}
		}
	}
	return false

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
	// The proxy will inspect the URI in the topmost Route header
	// field value.  If it indicates this proxy, the proxy removes it
	// from the Route header field (this route node has been
	// reached).
	msg.TryRemoveTopRoute(rawMessage.From.GetAddress(), rawMessage.From.GetPort())
	return msg, nil
}

func (p *Proxy) getBackendOfResponse(addr string, msg *Message) (Backend, error) {
	backendWithParent, ok := p.backends[addr]
	if ok {
		return backendWithParent.backend, nil
	}

	log.WithFields(log.Fields{"backendAddr": addr}).Warn("Fail to find backend by address")

	transId, err := msg.GetClientTransaction()
	if err != nil {
		return nil, err
	}
	backend, err := p.dialogBasedBackends.GetBackend(transId)
	if err != nil {
		log.WithFields(log.Fields{"clientTransaction": transId}).Warn("Fail to find backend by transaction")
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
				log.WithFields(log.Fields{"backendAddr": addr, "dialog": dialog}).Info("dialog is bind to backend")
				p.dialogBasedBackends.AddBackend(dialog, backend)
			}
		case "BYE":
			dialog, _ := msg.GetDialog()
			if dialog != "" {
				log.WithFields(log.Fields{"backendAddr": addr, "dialog": dialog}).Info("dialog is closed")
				p.dialogBasedBackends.RemoveDialog(dialog)
			}
		}
	}
}

func (p *Proxy) HandleMessage(msg *Message) {
	log.WithFields(log.Fields{"host": msg.ReceivedFrom.GetAddress(), "port": msg.ReceivedFrom.GetPort(), "mesasge": msg}).Debug("Received a message")
	if msg.IsRequest() {
		if p.isMyMessage(msg) {
			log.Info("it is my request")
			p.sendToBackend(msg)
		} else {
			host, port, transport, err := p.getNextRequestHop(msg)
			log.Info("Not my request, get next hop, host=", host, ",port=", port, ",transport=", transport)
			serverTrans, ok := p.selfLearnRoute.GetRoute(host)
			if ok {
				p.addVia(msg, serverTrans)
				p.addRecordRoute(msg, serverTrans)
			}
			if err != nil {
				log.Error("Fail to find the next hop for request:", msg)
			} else {
				p.sendMessage(host, port, transport, msg)
			}
		}
	} else {
		msg.PopVia()
		host, port, transport, err := p.getNextReponseHop(msg)
		// if the response of SUBSCRIBE to the backend
		if method, err := msg.GetMethod(); err == nil && method == "SUBSCRIBE" {
			addr := fmt.Sprintf("%s:%d", host, port)
			if backendWithParent, ok := p.backends[addr]; ok {
				if dialog, err := msg.GetDialog(); err == nil {
					log.WithFields(log.Fields{"dialog": dialog, "backend": backendWithParent.backend.GetAddress()}).Info("bind the dialog to the response")
					p.dialogBasedBackends.AddBackend(dialog, backendWithParent.backend)
				}
			}
		}
		if err != nil {
			log.WithFields(log.Fields{"message": msg}).Error("Fail to find the next hop for response")
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
		log.Error("Fail to create branch parameter")
		return nil, err
	}
	viaParam.SetBranch(branch)
	via := NewVia()
	via.AddViaParam(viaParam)
	msg.AddVia(via)
	return via, nil
}

func (p *Proxy) addRecordRoute(msg *Message, transport ServerTransport) {
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
		log.Error("Fail to find the backend for my message\n", msg)
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
				log.WithFields(log.Fields{"trandId": transId, "backend": backend}).Debug("bind client transaction with backend")
				p.dialogBasedBackends.AddBackend(transId, backend)
			}
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
		log.WithFields(log.Fields{"backendAddr": backend.GetAddress(), "dialog": dialog}).Info("find backend by dialog")
		transport, _ = p.findTransportByBackendAddr(backend.GetAddress())
	} else {
		log.WithFields(log.Fields{"dialog": dialog, "error": err}).Warn("Fail to find backend by dialog")
	}
	// remove the SUBSCRIBE initialized dialog if the Subscription-State is terminated in NOTIFY message
	if method == "NOTIFY" {
		if v, err := msg.GetHeaderValue("Subscription-State"); err == nil {
			if s, ok := v.(string); ok && s == "terminated" {
				log.WithFields(log.Fields{"dialog": dialog}).Info("remove the dialog")
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
			log.WithFields(log.Fields{"backendAddr": addr}).Warn("Fail to find backend by address")
		} else {
			return proxyItem.transports[0], nil
		}
	}
	return nil, fmt.Errorf("Fail to find backend by %s", addr)

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
		log.Error("Fail to find the header To im message:", msg)
		return "", 0, "", fmt.Errorf("No To header in message")
	}
	destHost, err := to.GetHost()
	if err != nil {
		log.Error("Fail to find the Host in header To of message:", msg)
		return "", 0, "", fmt.Errorf("Fail to find Host in To header of message")
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
	msg.PopRoute()
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

func (p *Proxy) findClientTransport(host string, port int, transport string) (ClientTransport, error) {
	trans, err := p.clientTransMgr.GetTransport(transport, host, port)
	if err == nil && trans.primary == nil {
		serverTrans, ok := p.selfLearnRoute.GetRoute(host)
		if ok {
			udpServerTrans, ok := serverTrans.(*UDPServerTransport)
			remoteAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, strconv.Itoa(port)))
			if err == nil && ok {
				trans.primary = NewUDPClientTransportWithConn(udpServerTrans.conn, remoteAddr)
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
	t, err := p.findClientTransport(ip, port, transport)
	if err == nil {
		t.Send(msg)
	} else {
		log.WithFields(log.Fields{"host": host, "port": port, "transport": transport, "message": msg}).Error("Fail to find the transport by ", transport)
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
	selfLearnRoute *SelfLearnRoute) (*ProxyItem, error) {
	transports := make([]ServerTransport, 0)
	if udpPort > 0 {
		udpServerTrans, err := NewUDPServerTransport(address, udpPort, receivedSupport, selfLearnRoute)
		if err == nil {
			transports = append(transports, udpServerTrans)
		}
	} else if tcpPort > 0 {
		transports = append(transports, NewTCPServerTransport(address, tcpPort, receivedSupport, connAcceptedListener, selfLearnRoute))
	}
	backend, _ := CreateRoundRobinBackend(net.JoinHostPort(address, "0"), backends)
	proxyItem := &ProxyItem{transports: transports,
		backend:  backend,
		dests:    dests,
		defRoute: defRoute}

	return proxyItem, nil
}

func (p *ProxyItem) start(msgHandler MessageHandler) error {
	for _, trans := range p.transports {
		err := trans.Start(msgHandler)
		if err != nil {
			return err
		}
	}
	return nil
}
