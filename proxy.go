package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
)

type ProxyItem struct {
	transports []ServerTransport
	backend    Backend
	dests      []string
	defRoute   bool
}

type Proxy struct {
	name           string
	preConfigRoute *PreConfigRoute
	resolver       *PreConfigHostResolver
	items          []*ProxyItem
	clientTransMgr *ClientTransportMgr
}

func NewProxy(name string, preConfigRoute *PreConfigRoute, resolver *PreConfigHostResolver) *Proxy {
	return &Proxy{name: name,
		preConfigRoute: preConfigRoute,
		resolver:       resolver,
		items:          make([]*ProxyItem, 0),
		clientTransMgr: NewClientTransportMgr()}
}

func (p *Proxy) AddItem(item *ProxyItem) {
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

func (p *Proxy) startItem(item *ProxyItem) error {
	return item.start(func(msg *Message) {
		p.handleMessage(msg, item)
	})

}

func (p *Proxy) isMyMessage(msg *Message) bool {
	to, err := msg.GetTo()
	if err != nil {
		log.Error("Fail to find the header To im message:", msg)
		return false
	}

	userHost, err := to.GetUserHost()
	return err == nil && p.name == userHost
}

func (p *Proxy) handleMessage(msg *Message, from *ProxyItem) {
	log.Info(msg)
	if msg.IsRequest() {
		if p.isMyMessage(msg) {
			log.Info("it is my request")
			p.sendToBackend(msg)
		} else {
			host, port, transport, err := p.getNextRequestHop(msg)
			log.Info("Not my request, get next hop, host=", host, ",port=", port, ",transport=", transport)
			if err != nil {
				log.Error("Fail to find the next hop for request:", msg)
			} else {
				p.sendMessage(host, port, transport, msg)
			}
		}
	} else {
		log.Info("received a response")
		host, port, transport, err := p.getNextReponseHop(msg)
		if err != nil {
			log.Error("Fail to find the next hop for response:", msg)
		} else {
			log.WithFields(log.Fields{"host": host, "port": port, "transport": transport}).Info("Send response")
			p.sendMessage(host, port, transport, msg)
		}
	}
}

func (p *Proxy) sendToBackend(msg *Message) {
	backendItem := p.findBackendProxyItem()
	if backendItem == nil {
		log.Error("Fail to find the backend for my message\n", msg)
	} else {
		transport := backendItem.transports[0]
		viaParam := CreateViaParam(transport.GetProtocol(), transport.GetAddress(), transport.GetPort())
		branch, err := CreateBranch()
		if err != nil {
			log.Error("Fail to create branch parameter")
			return
		}
		viaParam.SetBranch(branch)
		via := NewVia()
		via.AddViaParam(viaParam)
		msg.AddVia(via)

		addr := NewAddrSpec()
		addr.sipURI = &SIPURI{Scheme: "sip", Host: transport.GetAddress(), port: transport.GetPort()}
		nameAddr := &NameAddr{DisplayName: "", Addr: addr}
		recRoute := NewRecRoute(nameAddr)
		recordRoute := NewRecordRoute()
		recordRoute.AddRecRoute(recRoute)
		msg.AddRecordRoute(recordRoute)
		err = backendItem.backend.Send(msg)
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Error("Fail to send message to backend")
		}
	}

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
	msg.PopVia()
	return
}

func (p *Proxy) findClientTransport(host string, port int, transport string) (ClientTransport, error) {
	return p.clientTransMgr.GetTransport(transport, host, port)
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
		log.Error("Fail to find the transport by ", transport)
	}
}

// NewProxyItem create a sip proxy
func NewProxyItem(address string,
	udpPort int,
	tcpPort int,
	backends []string,
	dests []string,
	defRoute bool) (*ProxyItem, error) {
	transports := make([]ServerTransport, 0)
	if udpPort > 0 {
		transports = append(transports, NewUDPServerTransport(address, udpPort))
	}
	backend, err := CreateBackend(backends)
	if err != nil {
		return nil, err
	}
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
