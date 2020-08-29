package main

import (
	"bufio"
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"strconv"
	"strings"
	"sync"
)

// RawMessage raw sip message
type RawMessage struct {
	PeerAddr string
	PeerPort int
	//Message         *[]byte
	Message         *Message
	From            ServerTransport
	ReceivedSupport bool
}

func NewRawMessage(peerAddr string, peerPort int, from ServerTransport, receivedSupport bool, msg *Message) *RawMessage {
	return &RawMessage{
		PeerAddr:        peerAddr,
		PeerPort:        peerPort,
		From:            from,
		ReceivedSupport: receivedSupport,
		Message:         msg,
	}
}

type MessageHandler interface {
	HandleRawMessage(msg *RawMessage)
	HandleMessage(msg *Message)
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

type TCPServerTransport struct {
	addr            string
	port            int
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
		reader := bufio.NewReaderSize(bytes.NewBuffer(buf), n)
		msg, err := ParseMessage(reader)
		if err == nil {
			u.msgHandler.HandleRawMessage(NewRawMessage(address, port, u, u.receivedSupport, msg))
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

func NewTCPServerTransport(addr string, port int, receivedSupport bool, selfLearnRoute *SelfLearnRoute) *TCPServerTransport {
	return &TCPServerTransport{addr: addr,
		port:            port,
		receivedSupport: receivedSupport,
		selfLearnRoute:  selfLearnRoute}
}

func (t *TCPServerTransport) Start(msgHandler MessageHandler) error {
	t.msgHandler = msgHandler
	hostPort := net.JoinHostPort(t.addr, strconv.Itoa(t.port))
	ln, err := net.Listen("tcp", hostPort)
	if err != nil {
		log.Error("Fail to listen on ", hostPort)
		return err
	}
	log.Info("Succeed listen on ", hostPort)
	go func() {
		for {
			conn, err := ln.Accept()
			if err == nil {
				go t.receiveMessage(conn)
			}
		}
	}()
	return nil

}

func (t *TCPServerTransport) receiveMessage(conn net.Conn) {
	reader := bufio.NewReader(conn)
	peerAddr, port, _ := net.SplitHostPort(conn.RemoteAddr().String())
	peerPort, _ := strconv.Atoi(port)
	for {
		msg, err := ParseMessage(reader)
		if err != nil {
			break
		}
		msg.ReceivedFrom = t
		t.msgHandler.HandleRawMessage(NewRawMessage(peerAddr, peerPort, t, t.receivedSupport, msg))
	}
}

func (t *TCPServerTransport) Send(host string, port int, message *Message) error {
	return nil
}

func (t *TCPServerTransport) GetProtocol() string {
	return "TCP"
}

func (t *TCPServerTransport) GetAddress() string {
	return t.addr
}

func (t *TCPServerTransport) GetPort() int {
	return t.port
}
