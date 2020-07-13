package main

import (
	"bytes"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"strings"
	"sync"
)

type MessageHandler = func(message *Message)

type ServerTransport interface {
	Start(msgHandler MessageHandler) error
	// send message to remote host with port
	Send(host string, port int, message *Message) error
	// TCP, UDP or TLS
	GetProtocol() string

	// Get Address
	GetAddress() string

	GetPort() int
}

type UDPServerTransport struct {
	addr       string
	port       int
	selfLearnRoute *SelfLearnRoute
	msgHandler MessageHandler
}

type ClientTransport interface {
	Send(msg *Message) error
}

type UDPClientTransport struct {
	conn       *net.UDPConn
	remoteAddr  *net.UDPAddr
	msgChannel chan *Message
}

func NewUDPClientTransport(host string, port int) (*UDPClientTransport, error) {
	raddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		log.WithFields(log.Fields{"host": host, "port": port, "error": err}).Error("Fail to resolve udp host address")
		return nil, err
	}
        laddr, _:= net.ResolveUDPAddr("udp", ":0" )
	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		log.WithFields(log.Fields{"host": host, "port": port, "error": err}).Error("Fail to dial UDP")
		return nil, err
	}
	ut := &UDPClientTransport{conn: conn, remoteAddr: raddr, msgChannel: make(chan *Message, 1000)}
	go ut.takeAndSendMessage()
	return ut, nil
}

func (u *UDPClientTransport) Send(msg *Message) error {
	u.msgChannel <- msg
	return nil
}

func (u *UDPClientTransport) takeAndSendMessage() {
	for {
		select {
		case msg := <-u.msgChannel:
			buf := bytes.NewBuffer( make([]byte, 0 ) )
			msg.Write( buf )
			n, err := u.conn.WriteToUDP( buf.Bytes(), u.remoteAddr )
			if err == nil {
				log.WithFields( log.Fields{ "length": n, "address": u.remoteAddr }).Info( "Succeed to send message" )
			} else {
				log.WithFields(log.Fields{"error": err}).Error("Fail to send message")
			}
		}
	}
}

type ClientTransportMgr struct {
	sync.Mutex
	transports map[string]ClientTransport
}

func NewClientTransportMgr() *ClientTransportMgr {
	return &ClientTransportMgr{transports: make(map[string]ClientTransport)}
}

func (c *ClientTransportMgr) GetTransport(protocol string, host string, port int) (ClientTransport, error) {
	c.Lock()
	defer c.Unlock()

	if strings.EqualFold(protocol, "udp") {
		fullAddr := fmt.Sprintf("udp://%s:%d", host, port)
		if trans, ok := c.transports[fullAddr]; ok {
			return trans, nil
		}
		trans, err := NewUDPClientTransport(host, port)
		if err != nil {
			return nil, err
		}
		c.transports[fullAddr] = trans
		return trans, nil
	}
	return nil, fmt.Errorf("not support %s", protocol)
}

func NewUDPServerTransport(addr string, port int, selfLearnRoute *SelfLearnRoute ) *UDPServerTransport {

	log.WithFields(log.Fields{"addr": addr, "port": port}).Info("Create new UDP server transport")
	return &UDPServerTransport{addr: addr, port: port, msgHandler: nil, selfLearnRoute: selfLearnRoute}
}

func (u *UDPServerTransport) Send(host string, port int, message *Message) error {
	return nil
}

func (u *UDPServerTransport) Start(msgHandler MessageHandler) error {
	u.msgHandler = msgHandler
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", u.addr, u.port))
	if err != nil {
		log.WithFields(log.Fields{"addr": u.addr}).Error("Not a valid ip address")
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.WithFields(log.Fields{"addr": u.addr, "port": u.port}).Error("Fail to listen on UDP")
		return err
	}

	log.WithFields(log.Fields{"addr": u.addr, "port": u.port}).Info("Success to listen on UDP")
	var msgChannel chan *Message = make(chan *Message, 1000)
	go u.processMessage(msgChannel)
	go func() {
		buf := make([]byte, 1024*64)
		for {
			log.Info("try to read a packet")
			n, peerAddr, err := conn.ReadFromUDP(buf)
			log.WithFields( log.Fields{"length": n}).Info("read a packet with length")
			if err != nil {
				log.WithFields(log.Fields{"addr": u.addr, "port": u.port, "error": err}).Error("Fail to read data")
				break
			}
			address := peerAddr.IP.String()
			port := peerAddr.Port
			log.WithFields(log.Fields{"length": n, "address": address, "port": port}).Info("a UDP packet is received")
                        b := buf[0:n]
			msg, err := u.parseMessage(address, port, b)
			if err != nil {
				log.Error("Fail to parse sip message:\n", string(b))
			} else {
				msgChannel <- msg
			}
		}
	}()
	return nil
}

// parse message
func (u *UDPServerTransport) parseMessage(peerAddr string, peerPort int, buf []byte) (*Message, error) {
	msg, err := ParseMessage(buf)
	if err != nil {
		log.Error("Fail to parse sip message ", string(buf))
		return nil, errors.New("Fail to parse sip message")
	}
	msg.ReceivedFrom = u
	u.selfLearnRoute.AddRoute( peerAddr, u )
	msg.ForEachViaParam( func( viaParam *ViaParam ) {
		u.selfLearnRoute.AddRoute( viaParam.Host,  u ) 
	})
	// set the received parameters
	if msg.IsRequest() {
		via, err := msg.GetVia()
		if err != nil {
			log.Error("Fail to find Via header in request")
			return nil, err
		}
		viaParam, err := via.GetParam(0)
		if err != nil {
			log.Error("Fail to find via-param in Via header")
			return nil, err
		}
		viaParam.SetReceived(peerAddr)
		if viaParam.HasParam("rport") {
			viaParam.SetParam("rport", fmt.Sprintf("%d", peerPort))
		}
	}

	// The proxy will inspect the URI in the topmost Route header
        // field value.  If it indicates this proxy, the proxy removes it
        // from the Route header field (this route node has been
        // reached).
	route, err := msg.GetRoute()
	if err == nil {
		routeParam, err := route.GetRouteParam( 0 )
		if err == nil {
			sipUri, err := routeParam.GetAddress().GetAddress().GetSIPURI()
			if err == nil && sipUri.Host == u.addr && sipUri.GetPort() == u.port {
				log.WithFields( log.Fields{ "route-param": routeParam } ).Info( "remove top route item because the top item is my address" )
				msg.PopRoute()
			}
		}
	}
	return msg, nil

}

func (u *UDPServerTransport) processMessage(msgChannel chan *Message) {

	for {
		select {
		case msg := <-msgChannel:
			u.msgHandler(msg)
		}
	}

}

func (u *UDPServerTransport) GetProtocol() string {
	return "UDP"
}

func (u *UDPServerTransport) GetAddress() string {
	return u.addr
}

func (u *UDPServerTransport) GetPort() int {
	return u.port
}
