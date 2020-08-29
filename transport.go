package main

import (
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"strings"
	"sync"
)

// RawMessage raw sip message
type RawMessage struct {
	PeerAddr        string
	PeerPort        int
	Message         *[]byte
	From            ServerTransport
	ReceivedSupport bool
}

func NewRawMessage(peerAddr string, peerPort int, from ServerTransport, receivedSupport bool, msg *[]byte) *RawMessage {
	return &RawMessage{
		PeerAddr:        peerAddr,
		PeerPort:        peerPort,
		From:            from,
		ReceivedSupport: receivedSupport,
		Message:         msg,
	}
}

type MessageHandler interface {
	HandleMessage(msg *RawMessage)
}

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
	addr            string
	port            int
	conn            *net.UDPConn
	receivedSupport bool
	selfLearnRoute  *SelfLearnRoute
	msgHandler      MessageHandler
}

type ClientTransport interface {
	Send(msg *Message) error
}

type UDPClientTransport struct {
	conn       *net.UDPConn
	remoteAddr *net.UDPAddr
}

func NewUDPClientTransport(host string, port int) (*UDPClientTransport, error) {
	raddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		log.WithFields(log.Fields{"host": host, "port": port, "error": err}).Error("Fail to resolve udp host address")
		return nil, err
	}
	laddr, _ := net.ResolveUDPAddr("udp", ":0")
	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		log.WithFields(log.Fields{"host": host, "port": port, "error": err}).Error("Fail to dial UDP")
		return nil, err
	}
	ut := &UDPClientTransport{conn: conn, remoteAddr: raddr}
	return ut, nil
}

func (u *UDPClientTransport) Send(msg *Message) error {
	buf := bytes.NewBuffer(make([]byte, 0))
	msg.Write(buf)
	n, err := u.conn.WriteToUDP(buf.Bytes(), u.remoteAddr)
	if err == nil {
		log.WithFields(log.Fields{"length": n, "address": u.remoteAddr}).Info("Succeed to send message")
	} else {
		log.WithFields(log.Fields{"error": err}).Error("Fail to send message")
	}
	return err

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

func NewUDPServerTransport(addr string, port int, receivedSupport bool, selfLearnRoute *SelfLearnRoute) *UDPServerTransport {

	log.WithFields(log.Fields{"addr": addr, "port": port}).Info("Create new UDP server transport")
	return &UDPServerTransport{addr: addr,
		port:            port,
		receivedSupport: receivedSupport,
		msgHandler:      nil,
		selfLearnRoute:  selfLearnRoute}
}

func (u *UDPServerTransport) Send(host string, port int, msg *Message) error {
	remoteAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(make([]byte, 0))
	msg.Write(buf)
	n, err := u.conn.WriteToUDP(buf.Bytes(), remoteAddr)
	if err == nil {
		log.WithFields(log.Fields{"length": n, "address": remoteAddr}).Info("Succeed to send message")
	} else {
		log.WithFields(log.Fields{"error": err}).Error("Fail to send message")
	}
	return err
}

func (u *UDPServerTransport) Start(msgHandler MessageHandler) error {
	u.msgHandler = msgHandler
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", u.addr, u.port))
	if err != nil {
		log.WithFields(log.Fields{"addr": u.addr}).Error("Not a valid ip address")
	}
	u.conn, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.WithFields(log.Fields{"addr": u.addr, "port": u.port}).Error("Fail to listen on UDP")
		return err
	}

	log.WithFields(log.Fields{"addr": u.addr, "port": u.port}).Info("Success to listen on UDP")
	go u.receiveMessage()
	return nil
}

func (u *UDPServerTransport) receiveMessage() {
	for {
		buf := make([]byte, 1024*64)
		log.Info("try to read a packet")
		n, peerAddr, err := u.conn.ReadFromUDP(buf)
		log.WithFields(log.Fields{"length": n}).Info("read a packet with length")
		if err != nil {
			log.WithFields(log.Fields{"addr": u.addr, "port": u.port, "error": err}).Error("Fail to read data")
			break
		}
		address := peerAddr.IP.String()
		port := peerAddr.Port
		log.WithFields(log.Fields{"length": n, "address": address, "port": port}).Info("a UDP packet is received")
		b := buf[0:n]
		u.msgHandler.HandleMessage(NewRawMessage(address, port, u, u.receivedSupport, &b))
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
