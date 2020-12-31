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
	PeerAddr        string
	PeerPort        int
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
		Message:         msg}
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
	localAddr       *net.UDPAddr
	conn            *net.UDPConn
	receivedSupport bool
	selfLearnRoute  *SelfLearnRoute
	msgHandler      MessageHandler
}

type ConnectionAcceptedListener interface {
	ConnectionAccepted(conn net.Conn)
}
type TCPServerTransport struct {
	addr                 string
	port                 int
	receivedSupport      bool
	selfLearnRoute       *SelfLearnRoute
	msgHandler           MessageHandler
	connAcceptedListener ConnectionAcceptedListener
}

type ClientTransport interface {
	Send(msg *Message) error
}

type FailOverClientTransport struct {
	primary   ClientTransport
	secondary ClientTransport
}

func NewFailOverClientTransport() *FailOverClientTransport {
	return &FailOverClientTransport{}
}

func (fct *FailOverClientTransport) SetPrimary(primary ClientTransport) {
	fct.primary = primary
}

func (fct *FailOverClientTransport) SetSecondary(secondary ClientTransport) {
	fct.secondary = secondary
}

func (fct *FailOverClientTransport) Send(msg *Message) error {
	if fct.primary != nil {
		err := fct.primary.Send(msg)
		if err == nil {
			return nil
		}
		fct.primary = nil
	}
	if fct.secondary != nil {
		return fct.secondary.Send(msg)
	}
	return fmt.Errorf("Fail to send message")
}

func (fct *FailOverClientTransport) IsConnected() bool {
	return fct.primary != nil || fct.secondary != nil
}

type UDPClientTransport struct {
	conn       *net.UDPConn
	localAddr  *net.UDPAddr
	remoteAddr *net.UDPAddr
}

type TCPClientTransport struct {
	addr          string
	reconnectable bool
	conn          net.Conn
}

var SupportedProtocol = map[string]string{"udp": "udp", "tcp": "tcp"}

// NewUDPClientTransport create a UDP client transport with host and port
func NewUDPClientTransport(host string, port int) (*UDPClientTransport, error) {
	raddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		log.WithFields(log.Fields{"host": host, "port": port, "error": err}).Error("Fail to resolve udp host address")
		return nil, err
	}
	laddr, _ := net.ResolveUDPAddr("udp", ":0")
	return &UDPClientTransport{conn: nil, localAddr: laddr, remoteAddr: raddr}, nil
}

func NewUDPClientTransportWithConn(conn *net.UDPConn, remoteAddr *net.UDPAddr) (*UDPClientTransport, error) {
	localAddr, err := net.ResolveUDPAddr("udp", conn.LocalAddr().String())
	if err != nil {
		return nil, err
	}
	return &UDPClientTransport{conn: conn, localAddr: localAddr, remoteAddr: remoteAddr}, err
}

func (u *UDPClientTransport) connect() error {
	if u.conn == nil {
		conn, err := net.ListenUDP("udp", u.localAddr)
		if err != nil {
			log.WithFields(log.Fields{"localAddr": u.localAddr, "error": err}).Error("Fail to listen on UDP")
			return err
		}
		u.conn = conn
	}
	return nil

}
func (u *UDPClientTransport) Send(msg *Message) error {
	err := u.connect()
	if err != nil {
		return err
	}
	b, err := msg.Bytes()
	if err != nil {
		return err
	}
	n, err := u.conn.WriteToUDP(b, u.remoteAddr)
	if err == nil {
		log.WithFields(log.Fields{"length": n, "localAddr": u.localAddr, "remoteAddr": u.remoteAddr, "message": msg}).Info("Succeed to send message")
	} else {
		log.WithFields(log.Fields{"localAddr": u.localAddr, "remoteAddr": u.remoteAddr, "error": err, "message": msg}).Error("Fail to send message")
	}
	return err

}

type ClientTransportMgr struct {
	sync.Mutex
	transports map[string]*FailOverClientTransport
}

// NewClientTransportMgr create a client transport manager object
func NewClientTransportMgr() *ClientTransportMgr {
	return &ClientTransportMgr{transports: make(map[string]*FailOverClientTransport)}
}

// GetTransport Get the client ransport by the protocl, host and port
func (c *ClientTransportMgr) GetTransport(protocol string, host string, port int) (*FailOverClientTransport, error) {
	c.Lock()
	defer c.Unlock()

	protocol = strings.ToLower(protocol)
	if _, ok := SupportedProtocol[protocol]; !ok {
		return nil, fmt.Errorf("not support %s", protocol)
	}
	fullAddr := fmt.Sprintf("%s://%s", protocol, net.JoinHostPort(host, strconv.Itoa(port)))
	if trans, ok := c.transports[fullAddr]; ok {
		return trans, nil
	}
	trans, err := c.createClientTransport(protocol, host, port)
	if err != nil {
		return nil, err
	}
	c.transports[fullAddr] = trans
	return trans, nil
}

func (c *ClientTransportMgr) createClientTransport(protocol string, host string, port int) (*FailOverClientTransport, error) {
	var client ClientTransport = nil
	var err error = nil
	if protocol == "udp" {
		client, err = NewUDPClientTransport(host, port)
	} else if protocol == "tcp" {
		client, err = NewTCPClientTransport(host, port)
	} else {
		return nil, fmt.Errorf("not support %s", protocol)
	}
	if err != nil {
		return nil, err
	}
	trans := NewFailOverClientTransport()
	trans.secondary = client
	return trans, nil

}

// NewTCPClientTransport create a TCP client transport with the specified host and port
func NewTCPClientTransport(host string, port int) (*TCPClientTransport, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	return &TCPClientTransport{addr: addr,
		reconnectable: true,
		conn:          nil}, nil
}

func NewTCPClientTransportWithConn(conn net.Conn) (*TCPClientTransport, error) {
	return &TCPClientTransport{addr: conn.RemoteAddr().String(),
		reconnectable: false,
		conn:          nil}, nil
}

func (t *TCPClientTransport) Send(msg *Message) error {
	b, err := msg.Bytes()

	if err != nil {
		return err
	}

	for i := 0; i < 2; i++ {
		if t.conn == nil {
			conn, err := net.Dial("tcp", t.addr)
			if err != nil {
				log.WithFields(log.Fields{"addr": t.addr}).Error("Fail to connect tcp server")
				return err
			}
			t.conn = conn
		}
		_, err := t.conn.Write(b)
		if err == nil {
			log.WithFields(log.Fields{"addr": t.addr}).Debug("Succeed to send message to server")
			return nil
		}
		log.WithFields(log.Fields{"addr": t.addr}).Error("Fail to send message to server")
		if t.reconnectable {
			t.conn.Close()
			t.conn = nil
		} else {
			break
		}
	}
	log.WithFields(log.Fields{"addr": t.addr}).Error("Fail to send message to tcp server")
	return fmt.Errorf("Fail to send message to %s", t.addr)
}

func NewUDPServerTransport(addr string, port int, receivedSupport bool, selfLearnRoute *SelfLearnRoute) (*UDPServerTransport, error) {

	log.WithFields(log.Fields{"addr": addr, "port": port}).Info("Create new UDP server transport")
	localAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(addr, strconv.Itoa(port)))
	if err != nil {
		log.WithFields(log.Fields{"addr": addr, "port": port}).Error("Not a valid ip address")
		return nil, err
	}
	return &UDPServerTransport{localAddr: localAddr,
		receivedSupport: receivedSupport,
		msgHandler:      nil,
		selfLearnRoute:  selfLearnRoute}, nil
}

func (u *UDPServerTransport) Send(host string, port int, msg *Message) error {
	remoteAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return err
	}
	b, err := msg.Bytes()
	if err != nil {
		return err
	}
	n, err := u.conn.WriteToUDP(b, remoteAddr)
	if err == nil {
		log.WithFields(log.Fields{"length": n, "localAddr": u.localAddr, "remoteAddress": remoteAddr}).Info("Succeed to send message")
	} else {
		log.WithFields(log.Fields{"localAddr": u.localAddr, "remoteAddress": remoteAddr, "error": err}).Error("Fail to send message")
	}
	return err
}

func (u *UDPServerTransport) Start(msgHandler MessageHandler) error {
	u.msgHandler = msgHandler
	conn, err := net.ListenUDP("udp", u.localAddr)
	if err != nil {
		log.WithFields(log.Fields{"localAddr": u.localAddr}).Error("Fail to listen on UDP")
		return err
	}
	u.conn = conn
	log.WithFields(log.Fields{"localAddr": u.localAddr}).Info("Success to listen on UDP")
	go u.receiveMessage()
	return nil
}

func (u *UDPServerTransport) receiveMessage() {
	for {
		buf := make([]byte, 1024*64)
		n, peerAddr, err := u.conn.ReadFromUDP(buf)
		if err != nil {
			log.WithFields(log.Fields{"localAddr": u.localAddr, "error": err}).Error("Fail to read data")
			break
		}
		address := peerAddr.IP.String()
		port := peerAddr.Port
		log.WithFields(log.Fields{"length": n, "localAddr": u.localAddr, "remoteAddr": peerAddr}).Info("a UDP packet is received")
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
	host, _, _ := net.SplitHostPort(u.localAddr.String())
	return host
}

func (u *UDPServerTransport) GetPort() int {
	_, port, _ := net.SplitHostPort(u.localAddr.String())
	i, _ := strconv.Atoi(port)
	return i
}

func NewTCPServerTransport(addr string,
	port int,
	receivedSupport bool,
	connAcceptedListener ConnectionAcceptedListener,
	selfLearnRoute *SelfLearnRoute) *TCPServerTransport {
	return &TCPServerTransport{addr: addr,
		port:                 port,
		receivedSupport:      receivedSupport,
		connAcceptedListener: connAcceptedListener,
		selfLearnRoute:       selfLearnRoute}
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
	go t.acceptConnection(ln)
	return nil
}

func (t *TCPServerTransport) acceptConnection(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err == nil {
			log.WithFields(log.Fields{"localAddr": ln.Addr(), "remoteAddr": conn.RemoteAddr()}).Info("Accept a connection")
			t.connAcceptedListener.ConnectionAccepted(conn)
			go t.receiveMessage(conn)
		} else {
			log.WithFields(log.Fields{"localAddr": ln.Addr(), "error": err}).Error("Fail to accept client connection")
			break
		}
	}

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
