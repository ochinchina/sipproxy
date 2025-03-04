package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RawMessage raw sip message
type RawMessage struct {
	PeerAddr        string
	PeerPort        int
	Message         *Message
	From            ServerTransport
	ReceivedSupport bool
	TcpConn         net.Conn
}

func NewRawMessage(peerAddr string, peerPort int, from ServerTransport, receivedSupport bool, msg *Message) *RawMessage {
	return &RawMessage{
		PeerAddr:        peerAddr,
		PeerPort:        peerPort,
		From:            from,
		ReceivedSupport: receivedSupport,
		Message:         msg,
		TcpConn:         nil}
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

	// return True if the transport exit
	IsExit() bool
}

type SizedByteArray struct {
	b          []byte
	n          int
	msgHandler func(msg *Message)
}
type UDPServerTransport struct {
	localAddr       *net.UDPAddr
	conn            *net.UDPConn
	receivedSupport bool
	selfLearnRoute  *SelfLearnRoute
	msgHandler      MessageHandler
	msgBufPool      *ByteArrayPool
	msgParseChannel chan SizedByteArray
}

type ConnectionAcceptedListener interface {
	ConnectionAccepted(conn net.Conn)
}
type TCPServerTransport struct {
	addr                 string
	port                 int
	conn                 net.Conn
	receivedSupport      bool
	selfLearnRoute       *SelfLearnRoute
	msgHandler           MessageHandler
	connAcceptedListener ConnectionAcceptedListener
	exit                 bool
}

type ClientTransport interface {
	Send(msg *Message) error
	IsExpired() bool
}

type FailOverClientTransport struct {
	primary   ClientTransport
	secondary ClientTransport
}

func NewFailOverClientTransport(primary ClientTransport, secondary ClientTransport) *FailOverClientTransport {
	return &FailOverClientTransport{primary: primary, secondary: secondary}
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
	return fmt.Errorf("fail to send message")
}

func (fct *FailOverClientTransport) IsConnected() bool {
	return fct.primary != nil || fct.secondary != nil
}

func (fct *FailOverClientTransport) IsExpired() bool {
	return (fct.primary != nil && fct.primary.IsExpired()) || (fct.secondary != nil && fct.secondary.IsExpired())
}

type UDPClientTransport struct {
	conn       *net.UDPConn
	localAddr  *net.UDPAddr
	remoteAddr *net.UDPAddr
}

type TCPClientTransport struct {
	addr                  string
	reconnectable         bool
	conn                  net.Conn
	expire                int64
	connectionEstablished ConnectionEstablished
}

var SupportedProtocol = map[string]string{"udp": "udp", "tcp": "tcp"}

// NewUDPClientTransport create a UDP client transport with host and port
func NewUDPClientTransport(host string, port int) (*UDPClientTransport, error) {
	raddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		zap.L().Error("Fail to resolve udp host address", zap.String("host", host), zap.Int("port", port), zap.String("error", err.Error()))
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
			zap.L().Error("Fail to listen on UDP", zap.String("localAddr", u.localAddr.String()), zap.String("error", err.Error()))
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
		zap.L().Info("Succeed to send message", zap.Int("length", n), zap.String("localAddr", u.localAddr.String()), zap.String("remoteAddr", u.remoteAddr.String()))
	} else {
		zap.L().Error("Fail to send message", zap.String("localAddr", u.localAddr.String()), zap.String("remoteAddr", u.remoteAddr.String()), zap.String("message", msg.String()), zap.String("error", err.Error()))
	}
	return err

}

func (u *UDPClientTransport) IsExpired() bool {
	return false
}

type ClientTransportMgr struct {
	sync.Mutex
	transports            map[string]*FailOverClientTransport
	lastCleanTime         int64
	connectionEstablished ConnectionEstablished
}

// NewClientTransportMgr create a client transport manager object
func NewClientTransportMgr(connectionEstablished ConnectionEstablished) *ClientTransportMgr {
	return &ClientTransportMgr{transports: make(map[string]*FailOverClientTransport),
		lastCleanTime:         time.Now().Unix(),
		connectionEstablished: connectionEstablished}

}

// GetTransport Get the client ransport by the protocol, host and port
func (c *ClientTransportMgr) GetTransport(protocol string, host string, port int, transId string) (*FailOverClientTransport, error) {
	c.Lock()
	defer c.Unlock()

	c.cleanExpiredTransport()

	protocol = strings.ToLower(protocol)
	if _, ok := SupportedProtocol[protocol]; !ok {
		return nil, fmt.Errorf("not support %s", protocol)
	}
	fullAddr := c.getFullAddr(protocol, host, port, transId)

	zap.L().Info("get full address", zap.String("fullAddr", fullAddr))

	if trans, ok := c.transports[fullAddr]; ok {
		zap.L().Info("get client by full address", zap.String("fullAddr", fullAddr))
		return trans, nil
	}
	trans, err := c.createClientTransport(protocol, host, port)
	if err != nil {
		zap.L().Info("fail to client by full address", zap.String("fullAddr", fullAddr))
		return nil, err
	}
	c.transports[fullAddr] = trans
	zap.L().Info("create client by full address", zap.String("fullAddr", fullAddr))
	return trans, nil
}

func (c *ClientTransportMgr) RemoveTransport(protocol string, host string, port int, transId string) {
	c.Lock()
	defer c.Unlock()
	protocol = strings.ToLower(protocol)
	if _, ok := SupportedProtocol[protocol]; !ok {
		return
	}
	fullAddr := c.getFullAddr(protocol, host, port, transId)
	delete(c.transports, fullAddr)
}

func (c *ClientTransportMgr) getFullAddr(protocol string, host string, port int, transId string) string {
	fullAddr := fmt.Sprintf("%s://%s", protocol, net.JoinHostPort(host, strconv.Itoa(port)))

	if protocol == "tcp" && transId != "" {
		fullAddr = fmt.Sprintf("%s-%s", fullAddr, transId)
	}

	return fullAddr
}

func (c *ClientTransportMgr) createClientTransport(protocol string, host string, port int) (*FailOverClientTransport, error) {
	var client ClientTransport = nil
	var err error = nil
	if protocol == "udp" {
		client, err = NewUDPClientTransport(host, port)
		if err == nil {
			return NewFailOverClientTransport(client, nil), nil
		} else {
			return nil, err
		}
	} else if protocol == "tcp" {
		addr := c.getFullAddr(protocol, host, port, "")
		if trans, ok := c.transports[addr]; ok {
			client = trans.secondary
		} else {
			client, err = NewTCPClientTransport(host, port, c.connectionEstablished)
			if err == nil {
				c.transports[addr] = NewFailOverClientTransport(nil, client)
			} else {
				return nil, err
			}
		}
		return NewFailOverClientTransport(nil, client), nil

	} else {
		return nil, fmt.Errorf("not support %s", protocol)
	}

}

func (c *ClientTransportMgr) cleanExpiredTransport() {
	var now int64 = time.Now().Unix()
	if now-c.lastCleanTime < 60 {
		return
	}
	c.lastCleanTime = now
	var expiredTransports []string = make([]string, 0)

	for key, transport := range c.transports {
		if (transport.primary != nil && transport.primary.IsExpired()) || (transport.secondary != nil && transport.secondary.IsExpired()) {
			expiredTransports = append(expiredTransports, key)
		}
	}
	for _, key := range expiredTransports {
		delete(c.transports, key)
	}

}

// NewTCPClientTransport create a TCP client transport with the specified host and port
func NewTCPClientTransport(host string, port int, connectionEstablished ConnectionEstablished) (*TCPClientTransport, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	return &TCPClientTransport{addr: addr,
		reconnectable:         true,
		conn:                  nil,
		expire:                0,
		connectionEstablished: connectionEstablished,
	}, nil
}

func NewTCPClientTransportWithConn(conn net.Conn) (*TCPClientTransport, error) {
	zap.L().Info("create TCPClientTransportWithConn", zap.String("remoteAddr", conn.RemoteAddr().String()))
	return &TCPClientTransport{addr: conn.RemoteAddr().String(),
		reconnectable:         false,
		conn:                  conn,
		expire:                time.Now().Unix() + 3600,
		connectionEstablished: nil}, nil
}

func (t *TCPClientTransport) Send(msg *Message) error {
	b, err := msg.Bytes()

	if err != nil {
		return err
	}

	for i := 0; i < 2; i++ {
		if t.conn == nil && t.reconnectable {
			conn, err := net.Dial("tcp", t.addr)
			if err != nil {
				zap.L().Error("Fail to connect tcp server", zap.String("addr", t.addr))
				return err
			} else {
				zap.L().Info("Succeed to connect tcp server", zap.String("addr", t.addr))
			}
			t.conn = conn
			if t.connectionEstablished != nil {
				t.connectionEstablished(conn)
			}
		}
		if t.conn == nil {
			continue
		}
		n, err := t.conn.Write(b)
		if err == nil {
			zap.L().Debug("Succeed to send message to TCP server", zap.String("addr", t.addr), zap.Int("bytesWritten", n))
			return nil
		}
		zap.L().Warn("Fail to send message to TCP server at this time, try it again", zap.String("addr", t.addr))
		t.conn.Close()
		t.conn = nil
	}
	zap.L().Error("Fail to send message to TCP server", zap.String("addr", t.addr))
	return fmt.Errorf("fail to send message to %s", t.addr)
}

func (t *TCPClientTransport) IsExpired() bool {
	return t.expire > 0 && time.Now().Unix() > t.expire
}

func NewUDPServerTransport(addr string, port int, receivedSupport bool, selfLearnRoute *SelfLearnRoute) (*UDPServerTransport, error) {

	zap.L().Info("Create new UDP server transport", zap.String("addr", addr), zap.Int("port", port))
	localAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(addr, strconv.Itoa(port)))
	if err != nil {
		zap.L().Error("Not a valid ip address", zap.String("addr", addr), zap.Int("port", port))
		return nil, err
	}
	return &UDPServerTransport{localAddr: localAddr,
		receivedSupport: receivedSupport,
		msgHandler:      nil,
		msgParseChannel: make(chan SizedByteArray, 40960),
		msgBufPool:      NewByteArrayPool(40960, 64*1024),
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
		zap.L().Info("Succeed to send message", zap.Int("length", n), zap.String("localAddr", u.localAddr.String()), zap.String("remoteAddress", remoteAddr.String()))
	} else {
		zap.L().Error("Fail to send message", zap.String("localAddr", u.localAddr.String()), zap.String("remoteAddress", remoteAddr.String()), zap.String("error", err.Error()))
	}
	return err
}

func (u *UDPServerTransport) Start(msgHandler MessageHandler) error {
	u.msgHandler = msgHandler
	conn, err := net.ListenUDP("udp", u.localAddr)
	if err != nil {
		zap.L().Error("Fail to listen on UDP", zap.String("localAddr", u.localAddr.String()))
		return err
	}
	u.conn = conn
	zap.L().Info("Success to listen on UDP", zap.String("localAddr", u.localAddr.String()))
	go u.startParseMessage()
	go u.receiveMessage()
	return nil
}

func (u *UDPServerTransport) receiveMessage() {
	for {
		buf := u.msgBufPool.Alloc()
		n, peerAddr, err := u.conn.ReadFromUDP(buf)
		if err != nil {
			zap.L().Error("Fail to read data", zap.String("localAddr", u.localAddr.String()), zap.String("error", err.Error()))
			break
		}
		address := peerAddr.IP.String()
		port := peerAddr.Port
		zap.L().Info("a UDP packet is received", zap.Int("length", n), zap.String("localAddr", u.localAddr.String()), zap.String("remoteAddr", peerAddr.String()))
		u.msgParseChannel <- SizedByteArray{b: buf, n: n, msgHandler: func(msg *Message) {
			u.msgHandler.HandleRawMessage(NewRawMessage(address, port, u, u.receivedSupport, msg))
		}}

	}
}

func (u *UDPServerTransport) startParseMessage() {
	for {
		sized_byte_array := <-u.msgParseChannel
		reader := bufio.NewReaderSize(bytes.NewBuffer(sized_byte_array.b), sized_byte_array.n)
		msg, err := ParseMessage(reader)
		u.msgBufPool.Free(sized_byte_array.b)
		if err == nil {
			sized_byte_array.msgHandler(msg)
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

func (u *UDPServerTransport) IsExit() bool {
	return false
}

func NewTCPServerTransport(addr string,
	port int,
	receivedSupport bool,
	connAcceptedListener ConnectionAcceptedListener,
	selfLearnRoute *SelfLearnRoute) *TCPServerTransport {
	return &TCPServerTransport{addr: addr,
		port:                 port,
		conn:                 nil,
		receivedSupport:      receivedSupport,
		connAcceptedListener: connAcceptedListener,
		selfLearnRoute:       selfLearnRoute,
		exit:                 false,
	}
}

func NewTCPServerTransportWithConn(conn net.Conn,
	receivedSupport bool,
	selfLearnRoute *SelfLearnRoute) *TCPServerTransport {
	addr, port, err := net.SplitHostPort(conn.LocalAddr().String())

	if err == nil {
		port_i, _ := strconv.Atoi(port)
		zap.L().Info("Create new TCP server transport", zap.String("remoteAddr", conn.RemoteAddr().String()), zap.String("localAddr", conn.LocalAddr().String()))
		return &TCPServerTransport{addr: addr,
			port:                 port_i,
			conn:                 conn,
			receivedSupport:      receivedSupport,
			connAcceptedListener: nil,
			selfLearnRoute:       selfLearnRoute,
			exit:                 false,
		}
	}
	return nil

}

func (t *TCPServerTransport) Start(msgHandler MessageHandler) error {
	t.msgHandler = msgHandler
	if t.conn == nil {
		hostPort := net.JoinHostPort(t.addr, strconv.Itoa(t.port))
		ln, err := net.Listen("tcp", hostPort)
		if err != nil {
			zap.L().Error("Fail to listen", zap.String("hostPort", hostPort))
			return err
		}
		zap.L().Info("Succeed to listen on TCP", zap.String("hostPort", hostPort))
		go t.acceptConnection(ln)
	} else {
		go t.receiveMessage(t.conn)
	}
	return nil
}

func (t *TCPServerTransport) acceptConnection(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err == nil {
			zap.L().Info("Accept a connection", zap.String("localAddr", ln.Addr().String()), zap.String("remoteAddr", conn.RemoteAddr().String()))
			t.connAcceptedListener.ConnectionAccepted(conn)
			go t.receiveMessage(conn)
		} else {
			zap.L().Error("Fail to accept client connection", zap.String("localAddr", ln.Addr().String()), zap.String("error", err.Error()))
			break
		}
	}

}

func (t *TCPServerTransport) receiveMessage(conn net.Conn) {
	reader := bufio.NewReader(conn)
	peerAddr, remotePort, _ := net.SplitHostPort(conn.RemoteAddr().String())
	peerPort, _ := strconv.Atoi(remotePort)
	localAddr, localPort, _ := net.SplitHostPort(conn.LocalAddr().String())
	zap.L().Info("start to receive sip message from tcp", zap.String("peerAddr", peerAddr), zap.String("peerPort", remotePort), zap.String("localAddr", localAddr), zap.String("localPort", localPort))
	for {
		msg, err := ParseMessage(reader)
		if err != nil {
			conn.Close()
			zap.L().Error("Fail to parse message", zap.String("peerAddr", peerAddr), zap.String("peerPort", remotePort), zap.String("localAddr", localAddr), zap.String("localPort", localPort), zap.String("error", err.Error()))
			break
		}
		msg.ReceivedFrom = t
		rawMsg := NewRawMessage(peerAddr, peerPort, t, t.receivedSupport, msg)
		rawMsg.TcpConn = conn
		t.msgHandler.HandleRawMessage(rawMsg)
	}
	if t.conn != nil {
		t.exit = true
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

func (u *TCPServerTransport) IsExit() bool {
	return u.conn != nil && u.exit
}

