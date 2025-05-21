package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Backend interface {
	// Send message to backend
	Send(msg *Message) (Backend, error)
	GetAddress() string
	Close()
}
type RoundRobinBackend struct {
	sync.Mutex
	index      int
	backends   []Backend
	backendMap map[string]Backend
	//backendChangeListenerMgr *BackendChangeListenerMgr
	//dialogBasedBackend       *DialogBasedBackend
}

type UDPBackend struct {
	backendAddr *net.UDPAddr
	udpConn     *net.UDPConn
}

type TCPBackend struct {
	localAddr             string
	backendAddr           string
	conn                  net.Conn
	connectionEstablished ConnectionEstablishedFunc
}

type BackendFactory struct {
	backends map[string]Backend
}

type ConnectionEstablishedFunc func(conn net.Conn)

var dynamicHostResolver *DynamicHostResolver
var backendFactory *BackendFactory

func init() {
	dynamicHostResolver = NewDynamicHostResolver(2)
	backendFactory = NewBackendFactory()
}

func createViaConfig(via string) *ViaConfig {
	u, err := url.Parse(via)
	if err != nil {
		return nil
	}
	// check if the scheme is tcp or udp
	if u.Scheme != "tcp" && u.Scheme != "udp" {
		return nil
	}
	host, s_port, _ := net.SplitHostPort(u.Host)
	port, err := strconv.Atoi(s_port)
	if err != nil {
		return nil
	}
	return &ViaConfig{
		Address:  host,
		Port:     port,
		Protocol: u.Scheme,
	}
}

func NewBackendFactory() *BackendFactory {
	return &BackendFactory{backends: make(map[string]Backend)}
}

func (bf *BackendFactory) CreateUDPBackend(localAddr string, hostport string) (Backend, error) {
	key := fmt.Sprintf("udp:%s-%s", localAddr, hostport)
	if backend, ok := bf.backends[key]; ok {
		return backend, nil
	}
	backend, err := NewUDPBackend(localAddr, hostport)
	if err != nil {
		return nil, err
	}
	bf.backends[key] = backend
	return backend, nil
}

func (bf *BackendFactory) CreateTCPBackend(localAddr string, hostport string, connectionEstablished ConnectionEstablishedFunc) (Backend, error) {
	key := fmt.Sprintf("tcp:%s-%s", localAddr, hostport)
	if backend, ok := bf.backends[key]; ok {
		return backend, nil
	}
	backend, err := NewTCPBackend(localAddr, hostport, connectionEstablished)
	if err != nil {
		return nil, err
	}
	bf.backends[key] = backend
	return backend, nil
}

func (bf *BackendFactory) RemoveTCPBackend(localAddr string, hostport string) (Backend, error) {
	key := fmt.Sprintf("tcp:%s-%s", localAddr, hostport)
	return bf.removeBackend(key)
}

func (bf *BackendFactory) RemoveUDPBackend(localAddr string, hostport string) (Backend, error) {
	key := fmt.Sprintf("udp:%s-%s", localAddr, hostport)
	return bf.removeBackend(key)

}

func (bf *BackendFactory) removeBackend(key string) (Backend, error) {

	if backend, ok := bf.backends[key]; ok {
		delete(bf.backends, key)
		return backend, nil
	}
	return nil, fmt.Errorf("fail to find backend %s", key)
}

func CreateRoundRobinBackend(backends []BackendConfig, connectionEstablished ConnectionEstablishedFunc) (*RoundRobinBackend, error) {
	zap.L().Info("create round robin backend", zap.Any("backends", backends))
	if len(backends) <= 0 {
		return nil, fmt.Errorf("no backends")
	}
	rrBackend := NewRoundRobinBackend()
	for _, backendConf := range backends {
		localBindAddress := net.JoinHostPort(backendConf.LocalAddress, "0")
		u, err := url.Parse(backendConf.Address)
		if err != nil {
			zap.L().Error("Fail to parse url address", zap.String("address", backendConf.Address))
			return nil, err
		}

		if u.Scheme == "udp" || u.Scheme == "tcp" {
			pos := strings.LastIndex(u.Host, ":")
			if pos == -1 {
				zap.L().Error("Fail to find port number", zap.String("address", u.Host))
			} else {
				host := u.Host[0:pos]
				port := u.Host[pos+1:]
				zap.L().Info("add backend", zap.String("host", host), zap.String("port", port), zap.String("protocol", u.Scheme))
				if isIPAddress(host) {
					var backend Backend
					var err error
					if u.Scheme == "udp" {
						backend, err = backendFactory.CreateUDPBackend(localBindAddress, u.Host)
					} else {
						backend, err = backendFactory.CreateTCPBackend(localBindAddress, u.Host, connectionEstablished)
					}
					if err != nil {
						return nil, err
					}
					rrBackend.AddBackend(backend)
				} else {
					zap.L().Info("add host to dynamic resolver", zap.String("host", host))

					dynamicHostResolver.ResolveHost(host, func(hostname string, newIPs []string, removedIPs []string) {
						rrBackend.hostIPChanged(u.Scheme, localBindAddress, hostname, newIPs, removedIPs, port, connectionEstablished)
					})
				}
			}
		} else {
			return nil, fmt.Errorf("unsupported protocol %s", u.Scheme)
		}
	}
	return rrBackend, nil
}

func NewUDPBackend(localhostport string, hostport string) (*UDPBackend, error) {
	zap.L().Info("create udp backend", zap.String("localhostport", localhostport), zap.String("hostport", hostport))
	backendAddr, err := net.ResolveUDPAddr("udp", hostport)
	if err != nil {
		return nil, err
	}
	localAddr, err := net.ResolveUDPAddr("udp", localhostport)
	if err != nil {
		return nil, err
	}
	udpConn, err := net.DialUDP("udp", localAddr, backendAddr)
	if err != nil {
		return nil, err
	}

	zap.L().Info("Succeed to create udp backend", zap.String("localAddr", udpConn.LocalAddr().String()), zap.String("backendAddr", backendAddr.String()))
	b := &UDPBackend{backendAddr: backendAddr, udpConn: udpConn}
	return b, nil
}

func (b *UDPBackend) Send(msg *Message) (Backend, error) {
	bytes, err := msg.Bytes()

	if err != nil {
		return nil, err
	}

	n, err := b.udpConn.Write(bytes)
	if err == nil {
		zap.L().Info("Succeed send message to UDP backend", zap.String("address", b.backendAddr.String()), zap.String("localAddress", b.udpConn.LocalAddr().String()), zap.Int("bytes", n))
		return b, err
	} else {
		zap.L().Error("Fail to send message to backend with UDP backend", zap.String("address", b.backendAddr.String()), zap.String("error", err.Error()), zap.String("localAddress", b.udpConn.LocalAddr().String()))
		return nil, err
	}
}

func (b *UDPBackend) GetAddress() string {
	return fmt.Sprintf("udp://%s", b.backendAddr.String())
}

func (b *UDPBackend) Close() {
	err := b.udpConn.Close()
	if err == nil {
		zap.L().Info("Succeed to close udp backend", zap.String("address", b.backendAddr.String()))
	} else {
		zap.L().Error("Fail to close udp backend", zap.String("address", b.backendAddr.String()))
	}
}

// / NewTCPBackend creates a TCP backend with the given local and remote addresses.
// / The local address is used to bind the connection, and the remote address is the destination.
func NewTCPBackend(localhostport string, hostport string, connectionEstablished ConnectionEstablishedFunc) (*TCPBackend, error) {
	zap.L().Info("create tcp backend", zap.String("localhostport", localhostport), zap.String("hostport", hostport))
	return &TCPBackend{localAddr: localhostport,
		backendAddr:           hostport,
		conn:                  nil,
		connectionEstablished: connectionEstablished}, nil
}

func (t *TCPBackend) Send(msg *Message) (Backend, error) {
	b, err := msg.Bytes()
	if err != nil {
		return nil, err
	}

	zap.L().Info("send message to TCP backend with conn", zap.String("backendAddr", t.backendAddr), zap.Any("conn", t.conn))

	for i := 0; i < 2; i++ {
		if t.conn == nil {
			t.connect()
		}

		if t.conn == nil {
			continue
		}

		_, err := t.conn.Write(b)
		if err == nil {
			zap.L().Debug("Succeed to send message to TCP backend", zap.String("backendAddr", t.backendAddr), zap.String("localAddress", t.conn.LocalAddr().String()), zap.String("message", string(b)))
			return t, nil
		}
		zap.L().Error("Fail to send message to backend with TCP backend", zap.String("backendAddr", t.backendAddr), zap.String("error", err.Error()), zap.String("localAddress", t.conn.LocalAddr().String()), zap.String("message", string(b)))
		t.conn.Close()
		t.conn = nil
	}
	return nil, fmt.Errorf("fail to send message to backend %s", t.backendAddr)
}

func (t *TCPBackend) connect() error {
	conn, err := net.Dial("tcp", t.backendAddr)
	if err != nil {
		zap.L().Error("Fail to connect backend", zap.String("backendAddr", t.backendAddr))
		t.conn = nil
		return err
	}
	zap.L().Info("Succeed to connect backend", zap.String("backendAddr", t.backendAddr), zap.String("remotAddr", conn.LocalAddr().String()))
	t.conn = conn
	t.connectionEstablished(conn)
	return nil
}

func (t *TCPBackend) GetAddress() string {
	return fmt.Sprintf("tcp://%s", t.backendAddr)
}

func (t *TCPBackend) Close() {
	if t.conn != nil {
		t.conn.Close()
	}
}

func NewRoundRobinBackend() *RoundRobinBackend {
	rb := &RoundRobinBackend{index: 0,
		backends:   make([]Backend, 0),
		backendMap: make(map[string]Backend)}
	return rb
}

func (rb *RoundRobinBackend) AddBackend(backend Backend) {
	rb.Lock()
	defer rb.Unlock()
	if _, ok := rb.backendMap[backend.GetAddress()]; ok {
		for index, p := range rb.backends {
			if backend.GetAddress() == p.GetAddress() {
				rb.backends = append(rb.backends[0:index], rb.backends[index+1:]...)
				break
			}
		}
		delete(rb.backendMap, backend.GetAddress())
		backend.Close()
	}
	rb.backends = append(rb.backends, backend)
	rb.backendMap[backend.GetAddress()] = backend
}

func (rb *RoundRobinBackend) GetBackend(address string) (Backend, error) {
	rb.Lock()
	defer rb.Unlock()

	if v, ok := rb.backendMap[address]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("fail to find backend by %s", address)
}

func (rb *RoundRobinBackend) RemoveBackend(address string) {
	rb.Lock()
	defer rb.Unlock()
	if _, ok := rb.backendMap[address]; ok {
		for index, p := range rb.backends {
			if address == p.GetAddress() {
				p.Close()
				backends := rb.backends[0:index]
				backends = append(backends, rb.backends[index+1:]...)
				rb.backends = backends
				break
			}
		}
		delete(rb.backendMap, address)
	}
}

func (rb *RoundRobinBackend) GetAllBackend() map[string]Backend {
	rb.Lock()
	defer rb.Unlock()

	r := make(map[string]Backend)
	for k, v := range rb.backendMap {
		r[k] = v
	}
	return r
}

func (rb *RoundRobinBackend) Send(msg *Message) (Backend, error) {
	index, err := rb.getNextBackendIndex()
	if err != nil {
		zap.L().Error("Fail to send message", zap.String("error", err.Error()))
		return nil, errors.New("fail to get next backend")
	}

	n := rb.getBackendCount()

	for ; n > 0; n-- {
		backend, err := rb.getBackend(index)
		index++
		if err == nil && backend != nil {
			r, err := backend.Send(msg)
			if err == nil {
				return r, err
			}
		}
	}

	return nil, errors.New("fail to send msg to all the backend")
}

func (rb *RoundRobinBackend) GetAddress() string {
	rb.Lock()
	defer rb.Unlock()

	r := "RoundRobin://"
	for index, backend := range rb.backends {
		if index != 0 {
			r += ","
		}
		r += backend.GetAddress()
	}
	return r
}

func (rb *RoundRobinBackend) getNextBackendIndex() (int, error) {
	rb.Lock()
	defer rb.Unlock()
	n := len(rb.backends)
	if n <= 0 {
		return 0, fmt.Errorf("no backend available")
	}
	rb.index = (rb.index + 1) % n
	return rb.index, nil
}

func (rb *RoundRobinBackend) getBackend(index int) (Backend, error) {
	rb.Lock()
	defer rb.Unlock()
	n := len(rb.backends)
	if n <= 0 {
		return nil, fmt.Errorf("no backend available at %d", index)
	}
	return rb.backends[index%n], nil
}

func (rb *RoundRobinBackend) getBackendCount() int {
	rb.Lock()
	defer rb.Unlock()
	return len(rb.backends)

}

func (rb *RoundRobinBackend) hostIPChanged(protocol string,
	localhostport, hostname string,
	newIPs []string,
	removedIPs []string,
	port string,
	connectionEstablished ConnectionEstablishedFunc) {
	for _, ip := range newIPs {
		zap.L().Info("find a new IP for host", zap.String("host", hostname), zap.String("ip", ip))
		hostport := net.JoinHostPort(ip, port)
		if protocol == "udp" {
			backend, err := backendFactory.CreateUDPBackend(localhostport, hostport)
			if err == nil {
				rb.AddBackend(backend)
			}
		} else if protocol == "tcp" {
			backend, err := backendFactory.CreateTCPBackend(localhostport, hostport, connectionEstablished)
			if err == nil {
				rb.AddBackend(backend)
			}
		}
	}
	for _, ip := range removedIPs {
		zap.L().Info("remove ip for host", zap.String("host", hostname), zap.String("ip", ip))
		hostport := net.JoinHostPort(ip, port)
		rb.RemoveBackend(hostport)
		if protocol == "udp" {
			backendFactory.RemoveUDPBackend(localhostport, hostport)
		} else if protocol == "tcp" {
			backendFactory.RemoveTCPBackend(localhostport, hostport)
		}
	}
}

func (rb *RoundRobinBackend) Close() {
	rb.Lock()
	defer rb.Unlock()
	for _, p := range rb.backends {
		p.Close()
	}
}

type ExpireBackend struct {
	backend Backend
	expire  time.Time
}
type SessionBasedBackend struct {
	timeout time.Duration
	// map between dialog and the backend
	backends      map[string]*ExpireBackend
	nextCleanTime time.Time
}

func NewSessionBasedBackend(timeoutSeconds int64) *SessionBasedBackend {
	zap.L().Info("set the dialog timeout ", zap.Int64("timeout", timeoutSeconds))

	return &SessionBasedBackend{timeout: time.Duration(timeoutSeconds) * time.Second,
		backends:      make(map[string]*ExpireBackend),
		nextCleanTime: time.Now().Add(time.Duration(timeoutSeconds) * time.Second)}
}

// GetBackend returns the backend related with the sessionId
// and check if the backend is expired. If the backend is expired, it will be removed from the map.
func (dbb *SessionBasedBackend) GetBackend(sessionId string) (Backend, error) {
	if value, ok := dbb.backends[sessionId]; ok {
		if value.expire.After(time.Now()) {
			return value.backend, nil
		}
		delete(dbb.backends, sessionId)
	}
	return nil, fmt.Errorf("no backend related with dialog %s", sessionId)

}

// AddBackend adds a backend for the sessionId and set the expire time
// for the backend. The expire time is set to the timeout value if it is greater than the timeout value.
func (dbb *SessionBasedBackend) AddBackend(sessionId string, backend Backend, expireSeconds int) {
	timeout := dbb.timeout
	// check if the expireSeconds is greater than the timeout value
	// if it is, set the timeout value to the expireSeconds
	if float64(expireSeconds) > timeout.Seconds() {
		timeout = time.Duration(expireSeconds) * time.Second
	}
	expire := time.Now().Add(timeout)
	dbb.backends[sessionId] = &ExpireBackend{backend: backend, expire: expire}
	zap.L().Info("add backend for session", zap.String("sessionId", sessionId), zap.String("expire", expire.String()), zap.String("backend", backend.GetAddress()))
	dbb.cleanExpiredSession()
}

// RemoveSession removes the backend related with the sessionId
// and closes the backend connection.
func (dbb *SessionBasedBackend) RemoveSession(sessionId string) {
	delete(dbb.backends, sessionId)
}

func (dbb *SessionBasedBackend) cleanExpiredSession() {
	if dbb.nextCleanTime.After(time.Now()) {
		return
	}
	dbb.nextCleanTime = time.Now().Add(dbb.timeout)

	// clean expired sessions
	expiredSessions := make(map[string]string)
	for k, v := range dbb.backends {
		if v.expire.Before(time.Now()) {
			expiredSessions[k] = k
		}
	}

	for k := range expiredSessions {
		delete(dbb.backends, k)
	}
}

func getAllBackendAddresses(backend Backend) []string {
	r := make([]string, 0)

	if v, ok := backend.(*RoundRobinBackend); ok {
		for _, t := range v.backends {
			r = append(r, t.GetAddress())
		}
	} else {
		r = append(r, backend.GetAddress())
	}

	return r
}

