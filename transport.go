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
	// TCPConn is used to send message back to the peer
	// when the message is received from TCP connection
	TcpConn net.Conn
	// the prefered backend
	Backend Backend
	Via     *ViaConfig
}

func NewRawMessage(peerAddr string, peerPort int, from ServerTransport, receivedSupport bool, msg *Message, backend Backend, via *ViaConfig) *RawMessage {
	return &RawMessage{
		PeerAddr:        peerAddr,
		PeerPort:        peerPort,
		From:            from,
		ReceivedSupport: receivedSupport,
		Message:         msg,
		TcpConn:         nil,
		Backend:         backend,
		Via:             via,
	}
}

func (rm *RawMessage) IsFromTCP() bool {
	return rm.TcpConn != nil
}

func (rm *RawMessage) GetRemoteTcpAddr() string {
	if rm.TcpConn == nil {
		return ""
	}
	return rm.TcpConn.RemoteAddr().String()
}

type MessageHandler interface {
	HandleRawMessage(msg *RawMessage)
	//HandleMessage(msg *Message, backend Backend)
}

type ServerTransport interface {
	Start(msgHandler MessageHandler) error
	// send message to remote host with port
	Send(host string, port int, message *Message) error
	// tcp, udp or tls
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
	via             *ViaConfig
	backend         Backend
	msgHandler      MessageHandler
	msgBufPool      *ByteArrayPool
	msgParseChannel chan SizedByteArray
}

type ConnectionAcceptedListener interface {
	ConnectionAccepted(conn net.Conn)
}
type TCPServerTransport struct {
	addr            string
	port            int
	conn            net.Conn
	receivedSupport bool
	selfLearnRoute  *SelfLearnRoute
	via             *ViaConfig
	// Optional backend for the transport
	backend              Backend
	msgHandler           MessageHandler
	connAcceptedListener ConnectionAcceptedListener
	exit                 bool
}

type ClientTransport interface {
	Send(msg *Message) error
	IsExpired() bool
}

type FailOverClientTransport struct {
	primary     ClientTransport
	secondaries []ClientTransport
}

type ClientTransportFactory struct {
	resolver         *PreConfigHostResolver
	clientTransports map[string]ClientTransport
}

func NewClientTransportFactory(resolver *PreConfigHostResolver) *ClientTransportFactory {
	return &ClientTransportFactory{resolver: resolver, clientTransports: make(map[string]ClientTransport)}
}

// CreateUDPClientTransport create a UDP client transport with host and port
// localAddress is the local address to bind to
func (ctf *ClientTransportFactory) CreateUDPClientTransport(host string, port int, localAddress string) (ClientTransport, error) {

	key := fmt.Sprintf("udp:%s:%d:%s", host, port, localAddress)
	if client, ok := ctf.clientTransports[key]; ok {
		return client, nil
	}

	ips, err := ctf.resolver.GetIps(host)
	if err != nil {
		return nil, err
	}
	clients := make([]ClientTransport, 0)
	for _, ip := range ips {
		client, err := NewUDPClientTransport(ip, port, localAddress)
		if err == nil {
			clients = append(clients, client)
		}
	}

	if len(clients) > 0 {
		ctf.clientTransports[key] = NewFailOverClientTransport(nil, clients)
		return ctf.clientTransports[key], nil
	} else {
		return nil, fmt.Errorf("fail to create UDP client transport")
	}
}

func (ctf *ClientTransportFactory) CreateUDPClientTransportWithConn(host string, port int, conn *net.UDPConn) (ClientTransport, error) {
	key := fmt.Sprintf("udp:%s:%d:%p", host, port, conn)
	if client, ok := ctf.clientTransports[key]; ok {
		return client, nil
	}
	ips, err := ctf.resolver.GetIps(host)
	if err != nil {
		zap.L().Error("fail to resolve host to IP for creating UDP client transport", zap.String("host", host))
		return nil, err
	}

	clients := make([]ClientTransport, 0)
	for _, ip := range ips {
		client, err := NewUDPClientTransportWithConn(ip, port, conn)
		if err == nil {
			clients = append(clients, client)
		}
	}

	if len(clients) > 0 {
		ctf.clientTransports[key] = NewFailOverClientTransport(nil, clients)
		return ctf.clientTransports[key], nil
	} else {
		return nil, fmt.Errorf("fail to create UDP client transport")
	}
}

// CreateTCPClientTransport create a TCP client transport with host and port
// localAddress is the local address to bind to
func (ctf *ClientTransportFactory) CreateTCPClientTransport(host string, port int, localAddress string, connectionEstablished ConnectionEstablishedFunc) (ClientTransport, error) {
	key := fmt.Sprintf("tcp:%s:%d:%s", host, port, localAddress)
	if client, ok := ctf.clientTransports[key]; ok {
		return client, nil
	}
	ips, err := ctf.resolver.GetIps(host)
	if err != nil || len(ips) == 0 {
		zap.L().Error("fail to resolve host to IP for creating TCP client transport", zap.String("host", host))
		return nil, fmt.Errorf("fail to resolve host to IP")
	}
	clients := make([]ClientTransport, 0)
	for _, ip := range ips {
		zap.L().Info("try to create TCP client transport", zap.String("ip", ip), zap.Int("port", port))
		client, err := NewTCPClientTransport(ip, port, localAddress, connectionEstablished)
		if err == nil {
			clients = append(clients, client)
		}
	}
	if len(clients) > 0 {
		ctf.clientTransports[key] = NewFailOverClientTransport(nil, clients)
		return ctf.clientTransports[key], nil
	} else {
		return nil, fmt.Errorf("fail to create TCP client transport")
	}
}

// RemoveUDPClientTransport remove the UDP client transport with host and port
// localAddress is the local address to bind to
func (ctf *ClientTransportFactory) RemoveUDPClientTransport(host string, port int, localAddress string) {
	key := fmt.Sprintf("udp:%s:%d:%s", host, port, localAddress)
	delete(ctf.clientTransports, key)
}

// RemoveTCPClientTransport remove the TCP client transport with host and port
// localAddress is the local address to bind to
func (ctf *ClientTransportFactory) RemoveTCPClientTransport(host string, port int, localAddress string) {
	key := fmt.Sprintf("tcp:%s:%d:%s", host, port, localAddress)
	delete(ctf.clientTransports, key)
}

// NewFailOverClientTransport create a fail over client transport with primary and secondary
// client transport
func NewFailOverClientTransport(primary ClientTransport, secondaries []ClientTransport) *FailOverClientTransport {
	return &FailOverClientTransport{secondaries: make([]ClientTransport, 0)}
}

// Set primary client transport
func (fct *FailOverClientTransport) SetPrimary(client ClientTransport) {
	fct.primary = client
}

// AddSecondary add the cient to back
func (fct *FailOverClientTransport) AddSecondary(client ClientTransport) {
	fct.secondaries = append(fct.secondaries, client)
}

func (fct *FailOverClientTransport) SetSecondaries(clients []ClientTransport) {
	fct.secondaries = clients
}

// Send send message to the primary client transport, if failed, send to the secondary client transport
// if both failed, return error
func (fct *FailOverClientTransport) Send(msg *Message) error {
	if fct.primary != nil {
		err := fct.primary.Send(msg)
		if err == nil {
			return nil
		}
	}
	for _, client := range fct.secondaries {
		err := client.Send(msg)
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("fail to send message")
}

// IsConnected return true if the primary or secondary client transport is connected
func (fct *FailOverClientTransport) IsConnected() bool {
	return fct.primary != nil || len(fct.secondaries) > 0
}

// IsExpired return true if the primary or secondary client transport is expired
func (fct *FailOverClientTransport) IsExpired() bool {
	if fct.primary != nil && fct.primary.IsExpired() {
		return true
	}
	for _, client := range fct.secondaries {
		if client.IsExpired() {
			return true
		}
	}
	return false
}

type UDPClientTransport struct {
	// if the connection is pre-connected, if true, the conn is pre-connected connection, otherwise, it is not a pre-connected connection
	preConnected bool
	// conn       is the UDP connection to send message
	conn *net.UDPConn
	// localAddr  is the local address to bind to or nil if not specified:
	// - not nil, the conn is pre-connected connection, conn.Write() should be used to send message to remote
	// - nil, the conn is not a pre-connected connection, conn.WriteToUDP() should be used to send message to remote
	localAddr *net.UDPAddr
	// remoteAddr is the remote address to send message to
	remoteAddr *net.UDPAddr
}

type TCPClientTransport struct {
	addr                  string
	localAddress          string
	reconnectable         bool
	conn                  net.Conn
	expire                int64
	connectionEstablished ConnectionEstablishedFunc
}

var SupportedProtocol = map[string]string{"udp": "udp", "tcp": "tcp"}

// NewUDPClientTransport create a UDP client transport with host and port
func NewUDPClientTransport(host string, port int, localAddress string) (*UDPClientTransport, error) {
	raddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		zap.L().Error("Fail to resolve udp host address", zap.String("host", host), zap.Int("port", port), zap.String("error", err.Error()))
		return nil, err
	}
	var laddr *net.UDPAddr = nil
	if len(localAddress) > 0 {
		laddr, _ = net.ResolveUDPAddr("udp", net.JoinHostPort(localAddress, "0"))
	}
	return &UDPClientTransport{preConnected: true, conn: nil, localAddr: laddr, remoteAddr: raddr}, nil
}

func NewUDPClientTransportWithConn(host string, port int, conn *net.UDPConn) (*UDPClientTransport, error) {
	return &UDPClientTransport{preConnected: false, conn: conn, localAddr: nil, remoteAddr: &net.UDPAddr{IP: net.ParseIP(host), Port: port}}, nil
}

func (u *UDPClientTransport) connect() error {
	if u.conn == nil {
		conn, err := net.DialUDP("udp", u.localAddr, u.remoteAddr)
		if err != nil {
			zap.L().Error("Fail to listen on UDP", zap.String("localAddr", u.localAddr.String()), zap.String("error", err.Error()))
			return err
		}
		u.conn = conn
	}
	return nil

}

// Send send message to the remote address
func (u *UDPClientTransport) Send(msg *Message) error {
	err := u.connect()
	if err != nil {
		return err
	}
	b, err := msg.Bytes()
	if err != nil {
		return err
	}
	if u.preConnected {
		_, err = u.conn.Write(b)
	} else {
		_, err = u.conn.WriteToUDP(b, u.remoteAddr)
	}
	if err == nil {
		zap.L().Debug("Succeed to send message through UDP", zap.String("localAddr", u.conn.LocalAddr().String()), zap.String("remoteAddr", u.remoteAddr.String()), zap.String("message", msg.String()))
	} else {
		zap.L().Error("Fail to send message", zap.String("localAddr", u.conn.LocalAddr().String()), zap.String("remoteAddr", u.remoteAddr.String()), zap.String("message", msg.String()), zap.String("error", err.Error()))
	}
	return err

}

func (u *UDPClientTransport) IsExpired() bool {
	return false
}

type ClientTransportMgr struct {
	sync.Mutex
	transports             map[string]*FailOverClientTransport
	lastCleanTime          int64
	selfLearnRoute         *SelfLearnRoute
	clientTransportFactory *ClientTransportFactory
	connectionEstablished  ConnectionEstablishedFunc
}

// NewClientTransportMgr create a client transport manager object
func NewClientTransportMgr(clientTransportFactory *ClientTransportFactory,
	selfLearnRoute *SelfLearnRoute,
	connectionEstablished ConnectionEstablishedFunc) *ClientTransportMgr {

	return &ClientTransportMgr{transports: make(map[string]*FailOverClientTransport),
		lastCleanTime:          time.Now().Unix(),
		selfLearnRoute:         selfLearnRoute,
		clientTransportFactory: clientTransportFactory,
		connectionEstablished:  connectionEstablished}

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
		zap.L().Info("get client transport by full address", zap.String("fullAddr", fullAddr))
		return trans, nil
	}
	trans, err := c.createClientTransport(protocol, host, port)
	if err != nil {
		zap.L().Info("fail to create client transport by full address", zap.String("fullAddr", fullAddr))
		return nil, err
	}
	c.transports[fullAddr] = trans
	zap.L().Info("succeed to create client by full address", zap.String("fullAddr", fullAddr))
	return trans, nil
}

// RemoveTransport remove the client transport by the protocol, host and port
// transId is used to identify the transport
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
	var localAddress string = c.getLocalAddress(host, protocol)

	switch protocol {
	case "udp":
		serverTransport, success := c.selfLearnRoute.GetRoute(host, protocol)
		if success && serverTransport != nil {
			if udpServerTransport, ok := serverTransport.(*UDPServerTransport); ok {
				client, err = c.clientTransportFactory.CreateUDPClientTransportWithConn(host, port, udpServerTransport.conn)
			}
		}

		if client == nil || err != nil {
			// if self learn route is available, use it to create a new UDP client transport
			// if self learn route is not available, create a new UDP client transport
			client, err = c.clientTransportFactory.CreateUDPClientTransport(host, port, localAddress)
		}
		if err == nil {
			return NewFailOverClientTransport(client, make([]ClientTransport, 0)), nil
		} else {
			return nil, err
		}
	case "tcp":
		addr := c.getFullAddr(protocol, host, port, "")
		if trans, ok := c.transports[addr]; ok {
			return NewFailOverClientTransport(nil, trans.secondaries), nil
		} else {
			client, err = c.clientTransportFactory.CreateTCPClientTransport(host, port, localAddress, c.connectionEstablished)
			if err == nil {
				c.transports[addr] = NewFailOverClientTransport(nil, []ClientTransport{client})
				return c.transports[addr], nil
			} else {
				return nil, err
			}
		}

	default:
		return nil, fmt.Errorf("not support %s", protocol)
	}

}

func (c *ClientTransportMgr) getLocalAddress(host string, protocol string) string {
	if c.selfLearnRoute != nil {
		return c.selfLearnRoute.GetRouteAddress(host, protocol)
	}
	return ""
}

func (c *ClientTransportMgr) cleanExpiredTransport() {
	var now int64 = time.Now().Unix()
	if now-c.lastCleanTime < 60 {
		return
	}
	c.lastCleanTime = now
	var expiredTransports []string = make([]string, 0)

	for key, transport := range c.transports {
		if transport.IsExpired() {
			expiredTransports = append(expiredTransports, key)
		}
	}
	for _, key := range expiredTransports {
		delete(c.transports, key)
	}

}

// NewTCPClientTransport create a TCP client transport with the specified host and port
func NewTCPClientTransport(host string, port int, localAddress string, connectionEstablished ConnectionEstablishedFunc) (*TCPClientTransport, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	return &TCPClientTransport{addr: addr,
		localAddress:          localAddress,
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
			laddr, _ := net.ResolveTCPAddr("tcp", net.JoinHostPort(t.localAddress, "0"))
			raddr, _ := net.ResolveTCPAddr("tcp", t.addr)
			zap.L().Info("Try to connect TCP server", zap.String("addr", t.addr), zap.String("localAddress", t.localAddress))
			conn, err := net.DialTCP("tcp", laddr, raddr)
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

func NewUDPServerTransport(addr string, port int, receivedSupport bool, selfLearnRoute *SelfLearnRoute, via *ViaConfig, backend Backend) (*UDPServerTransport, error) {

	zap.L().Info("Create new UDP server transport", zap.String("addr", addr), zap.Int("port", port))
	localAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(addr, strconv.Itoa(port)))
	if err != nil {
		zap.L().Error("Not a valid ip address", zap.String("addr", addr), zap.Int("port", port))
		return nil, err
	}
	return &UDPServerTransport{
		localAddr:       localAddr,
		receivedSupport: receivedSupport,
		msgHandler:      nil,
		msgParseChannel: make(chan SizedByteArray, 40960),
		msgBufPool:      NewByteArrayPool(40960, 64*1024),
		via:             via,
		selfLearnRoute:  selfLearnRoute,
		backend:         backend,
	}, nil
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
		zap.L().Info("Succeed to send message through UDP", zap.Int("length", n), zap.String("localAddr", u.conn.LocalAddr().String()), zap.String("remoteAddress", remoteAddr.String()))
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
			u.msgHandler.HandleRawMessage(NewRawMessage(address, port, u, u.receivedSupport, msg, u.backend, u.via))
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
	return "udp"
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
	selfLearnRoute *SelfLearnRoute,
	via *ViaConfig,
	backend Backend) *TCPServerTransport {
	return &TCPServerTransport{addr: addr,
		port:                 port,
		conn:                 nil,
		receivedSupport:      receivedSupport,
		connAcceptedListener: connAcceptedListener,
		selfLearnRoute:       selfLearnRoute,
		via:                  via,
		backend:              backend,
		msgHandler:           nil,
		exit:                 false,
	}
}

func NewTCPServerTransportWithConn(conn net.Conn,
	receivedSupport bool,
	selfLearnRoute *SelfLearnRoute,
	backend Backend) *TCPServerTransport {
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
			backend:              backend,
			msgHandler:           nil,
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
			if err.Error() == "EOF" {
				zap.L().Info("Connection closed", zap.String("peerAddr", peerAddr), zap.String("peerPort", remotePort), zap.String("localAddr", localAddr), zap.String("localPort", localPort))
			} else {
				zap.L().Error("Fail to parse message", zap.String("peerAddr", peerAddr), zap.String("peerPort", remotePort), zap.String("localAddr", localAddr), zap.String("localPort", localPort), zap.String("error", err.Error()))
			}
			break
		}
		msg.ReceivedFrom = t
		rawMsg := NewRawMessage(peerAddr, peerPort, t, t.receivedSupport, msg, t.backend, t.via)
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
	return "tcp"
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

