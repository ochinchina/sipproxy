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

	"github.com/go-redis/redis"
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
	backendAddr string
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
	b := &UDPBackend{backendAddr: hostport, udpConn: udpConn}
	return b, nil
}

func (b *UDPBackend) Send(msg *Message) (Backend, error) {
	bytes, err := msg.Bytes()

	if err != nil {
		return nil, err
	}

	n, err := b.udpConn.Write(bytes)
	if err == nil {
		zap.L().Info("Succeed send message to UDP backend", zap.String("address", b.backendAddr), zap.String("localAddress", b.udpConn.LocalAddr().String()), zap.Int("bytes", n))
		return b, err
	} else {
		zap.L().Error("Fail to send message to backend with UDP backend", zap.String("address", b.backendAddr), zap.String("error", err.Error()), zap.String("localAddress", b.udpConn.LocalAddr().String()))
		return nil, err
	}
}

func (b *UDPBackend) GetAddress() string {
	return fmt.Sprintf("udp://%s", b.backendAddr)
}

func (b *UDPBackend) Close() {
	err := b.udpConn.Close()
	if err == nil {
		zap.L().Info("Succeed to close udp backend", zap.String("address", b.backendAddr))
	} else {
		zap.L().Error("Fail to close udp backend", zap.String("address", b.backendAddr))
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

// GetBackend returns the backend related with the address
// and check if the backend is expired. If the backend is expired, it will be removed from the map.
// It will also check if the backend is already in the backend map.
// If the backend is already in the backend map, it will return the backend directly.
// If the backend is not found, it will return an error.
// address should be in the format of "tcp://host:port" or "udp://host:port"
func (rb *RoundRobinBackend) GetBackend(address string) (Backend, error) {
	rb.Lock()
	defer rb.Unlock()

	if v, ok := rb.backendMap[address]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("fail to find backend by %s", address)
}

// RemoveBackend removes the backend from the list of backends
// and closes the backend connection.
// It will also remove the backend from the backend map.
// If the backend is not found, it will do nothing.
// It is used to remove the backend when the host is changed.
// address should be in the format of "tcp://host:port" or "udp://host:port"
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
		zap.L().Info("find a new IP for host", zap.String("host", hostname), zap.String("ip", ip), zap.String("port", port), zap.String("protocol", protocol))
		// add the backend
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
		zap.L().Info("remove ip for host", zap.String("host", hostname), zap.String("ip", ip), zap.String("port", port), zap.String("protocol", protocol))
		// remove the backend
		hostport := net.JoinHostPort(ip, port)
		if protocol == "udp" {
			rb.RemoveBackend(fmt.Sprintf("udp://%s", hostport))
			backendFactory.RemoveUDPBackend(localhostport, hostport)
		} else if protocol == "tcp" {
			rb.RemoveBackend(fmt.Sprintf("tcp://%s", hostport))
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
type SessionBasedBackend interface {
	GetBackend(sessionId string) (Backend, error)
	AddBackend(sessionId string, backend Backend, expireSeconds int)
	RemoveSession(sessionId string)
}

type LocalSessionBasedBackend struct {
	timeout time.Duration
	// map between dialog and the backend
	backends      map[string]*ExpireBackend
	nextCleanTime time.Time
}

type MasterSlaveRedisSessionBasedBackend struct {
	// rdbs is a list of Redis clients.
	// It is used to connect to the Redis server and subscribe to backend updates.
	rdbs []*redis.Client
	// timeout is the dialog timeout in seconds.
	// It is used to set the expire time for the session in Redis.
	timeout time.Duration
	// redisChannel is the Redis channel to publish/subscribe to backend updates.
	// It is used to receive backend updates from Redis.
	redisChannel string
	// masterIndex is the index of the master Redis client.
	// It is used to publish backend updates to the master Redis client.
	masterIndex int
	// sessionBackendAddrs is a map between sessionId and backend address.
	// The address is in the format of "tcp://host:port" or "udp://host:port".
	sessionBackendAddrs *RedisSessionBackendAddrMgr
	// findBackendByAddr is a function to find the backend by address.
	findBackendByAddr func(string) (Backend, error)

	// retryTimeout is the timeout in milliseconds for retrying to connect to Redis.
	retryTimeout int
}

type RedisSessionBackendAddrMgr struct {
	sync.Mutex
	// sessionBackends is a map between sessionId and address info.
	sessionBackendAddrs map[string]struct {
		address string
		expires int64 // expires is the expiration time in seconds since epoch
	}

	nextCleanTime int64
}

type CompositeSessionBasedBackend struct {
	backends []SessionBasedBackend
}

func NewRedisSessionBackendAddrMgr() *RedisSessionBackendAddrMgr {
	return &RedisSessionBackendAddrMgr{
		sessionBackendAddrs: make(map[string]struct {
			address string
			expires int64
		}),
		nextCleanTime: 0,
	}
}

func (rsb *RedisSessionBackendAddrMgr) GetBackendAddress(sessionId string) (string, error) {
	rsb.Lock()
	defer rsb.Unlock()

	rsb.cleanExpiredSession()

	if addrInfo, ok := rsb.sessionBackendAddrs[sessionId]; ok && addrInfo.expires > time.Now().Unix() {
		return addrInfo.address, nil
	}
	return "", fmt.Errorf("no backend address found for session %s", sessionId)
}

// SetBackendAddress sets the backend address for the given sessionId.
// It updates the sessionBackendAddrs map with the sessionId and address and expires.
func (rsb *RedisSessionBackendAddrMgr) SetBackendAddress(sessionId string, address string, expires int64) {
	rsb.Lock()
	defer rsb.Unlock()

	rsb.cleanExpiredSession()

	rsb.sessionBackendAddrs[sessionId] = struct {
		address string
		expires int64
	}{
		address: address,
		expires: expires + time.Now().Unix(), // Set the expiration time to the current time plus the expires value
	}
}

func (rsb *RedisSessionBackendAddrMgr) RemoveBackend(sessionId string) error {
	rsb.Lock()
	defer rsb.Unlock()

	if _, exists := rsb.sessionBackendAddrs[sessionId]; exists {
		delete(rsb.sessionBackendAddrs, sessionId)
		return nil
	} else {
		zap.L().Warn("Session backend address not found for removal", zap.String("sessionId", sessionId))
		return fmt.Errorf("session backend address not found for session %s", sessionId)
	}
}

func (rsb *RedisSessionBackendAddrMgr) cleanExpiredSession() {
	if rsb.nextCleanTime > time.Now().Unix() {
		return
	}
	rsb.nextCleanTime = time.Now().Unix() + 60 // Clean expired sessions every 60 seconds

	expiredSessions := make([]string, 0)
	for sessionId, addrInfo := range rsb.sessionBackendAddrs {
		if addrInfo.expires <= time.Now().Unix() {
			expiredSessions = append(expiredSessions, sessionId)
		}
	}

	for _, sessionId := range expiredSessions {
		delete(rsb.sessionBackendAddrs, sessionId)
		zap.L().Info("remove expired session backend address", zap.String("sessionId", sessionId))
	}
}

func NewLocalSessionBasedBackend(timeoutSeconds int64) *LocalSessionBasedBackend {
	zap.L().Info("set the dialog timeout ", zap.Int64("timeout", timeoutSeconds))

	return &LocalSessionBasedBackend{timeout: time.Duration(timeoutSeconds) * time.Second,
		backends:      make(map[string]*ExpireBackend),
		nextCleanTime: time.Now().Add(time.Duration(timeoutSeconds) * time.Second)}
}

// GetBackend returns the backend related with the sessionId
// and check if the backend is expired. If the backend is expired, it will be removed from the map.
func (dbb *LocalSessionBasedBackend) GetBackend(sessionId string) (Backend, error) {
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
func (dbb *LocalSessionBasedBackend) AddBackend(sessionId string, backend Backend, expireSeconds int) {
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
func (dbb *LocalSessionBasedBackend) RemoveSession(sessionId string) {
	delete(dbb.backends, sessionId)
}

func (dbb *LocalSessionBasedBackend) cleanExpiredSession() {
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

// createRedisClient creates a Redis client with the given address.
// It checks if the address is valid and if the DB number is valid (>= 0).
// If the address is empty or the DB number is invalid, it logs an error and returns nil.
func createRedisClient(address RedisAddress) *redis.Client {
	if address.Address == "" {
		zap.L().Error("Redis address is empty, please check your configuration")
		return nil
	}
	if address.Db < 0 {
		zap.L().Error("Redis DB number must be greater than or equal to 0", zap.Int("db", address.Db))
		return nil
	}
	return redis.NewClient(&redis.Options{
		Addr:     address.Address,
		Password: address.Password, // no password set
		DB:       address.Db,       // use default DB,
	})

}

// NewMasterSlaveRedisSessionBasedBackend creates a new MasterSlaveRedisSessionBasedBackend instance.
// It initializes the Redis clients based on the provided RedisSessionStore configuration.
// The `timeoutSeconds` parameter specifies the timeout for session expiration.
// The `findBackendByAddr` function is used to find the backend by its address.
func NewMasterSlaveRedisSessionBasedBackend(redisSessionStore RedisSessionStore,
	timeoutSeconds int64,
	// findBackendByAddr is a function to find the backend by address.
	findBackendByAddr func(backendAddr string) (Backend, error)) *MasterSlaveRedisSessionBasedBackend {

	if len(redisSessionStore.Addresses) == 0 {
		zap.L().Error("No Redis addresses provided, please check your configuration")
		return nil
	}

	rdbs := make([]*redis.Client, 0)

	for _, addr := range redisSessionStore.Addresses {
		rdb := createRedisClient(addr)
		if rdb != nil {
			rdbs = append(rdbs, rdb)
		}
	}

	channel := redisSessionStore.Channel
	if channel == "" {
		channel = "sipproxy:session" // Default channel name
	}

	rsb := &MasterSlaveRedisSessionBasedBackend{
		rdbs:                rdbs,
		timeout:             time.Duration(timeoutSeconds) * time.Second,
		masterIndex:         0, // Default to the first Redis client as master
		redisChannel:        channel,
		findBackendByAddr:   findBackendByAddr,
		sessionBackendAddrs: NewRedisSessionBackendAddrMgr(),
		retryTimeout: func() int {
			if redisSessionStore.RetryTimeout <= 0 {
				return 5
			}
			return redisSessionStore.RetryTimeout
		}(),
	}
	rsb.start()
	return rsb
}

// start initializes the Redis clients and subscribes to backend updates.
// It should be called after creating the MasterSlaveRedisSessionBasedBackend instance.
func (msrsb *MasterSlaveRedisSessionBasedBackend) start() {
	for _, rdb := range msrsb.rdbs {
		go msrsb.subscribeToBackendUpdates(rdb)
	}

}

// GetBackend retrieves the backend associated with the given sessionId.
// It uses the sessionBackendAddrs manager to get the backend address for the sessionId.
func (msrsb *MasterSlaveRedisSessionBasedBackend) GetBackend(sessionId string) (Backend, error) {
	zap.L().Info("get backend for session from redis", zap.String("sessionId", sessionId))

	backendAddr, err := msrsb.sessionBackendAddrs.GetBackendAddress(sessionId)

	if err == nil && msrsb.findBackendByAddr != nil {
		return msrsb.findBackendByAddr(backendAddr)
	}

	return nil, fmt.Errorf("no Redis client available to get backend for session %s", sessionId)
}

func (rsb *MasterSlaveRedisSessionBasedBackend) subscribeToBackendUpdates(rdb *redis.Client) {
	for {
		pubsub := rsb.doSubscribe(rdb)
		if pubsub != nil {
			rsb.receiveSubscribeMessage(pubsub)
			time.Sleep(time.Duration(rsb.retryTimeout) * time.Second) // Wait before retrying subscription
		} else {
			time.Sleep(time.Duration(rsb.retryTimeout) * time.Second) // Wait before retrying subscription
			zap.L().Warn("Retrying subscription to Redis channel", zap.String("channel", rsb.redisChannel), zap.String("address", rdb.Options().Addr))
		}
	}

}

func (rsb *MasterSlaveRedisSessionBasedBackend) doSubscribe(rdb *redis.Client) *redis.PubSub {
	pubsub := rdb.Subscribe(rsb.redisChannel)
	if pubsub == nil {
		zap.L().Error("Failed to subscribe to Redis channel", zap.String("address", rdb.Options().Addr))
		return nil
	}

	// Wait for the subscription to be confirmed
	_, err := pubsub.ReceiveTimeout(time.Duration(2 * time.Second))

	if err != nil {
		zap.L().Error("Failed to receive subscription confirmation", zap.String("address", rdb.Options().Addr), zap.Error(err))
		return nil
	}

	zap.L().Info("Subscribed to Redis channel for backend updates", zap.String("channel", rsb.redisChannel), zap.String("address", rdb.Options().Addr))

	return pubsub
}

// receiveSubscribeMessage listens for messages on the Redis PubSub channel.
// It processes each message by calling sessionBackendUpdated to handle the session updates.
func (rsb *MasterSlaveRedisSessionBasedBackend) receiveSubscribeMessage(pubsub *redis.PubSub) {
	defer pubsub.Close() // Ensure the PubSub is closed when done

	for {
		msg, err := pubsub.ReceiveMessage()
		if err == nil {
			rsb.sessionBackendUpdated(msg.Payload)
		} else {
			zap.L().Error("Failed to receive message from Redis", zap.Error(err))
			break // Exit the loop if an error occurs
		}
	}

}

// sessionBackendUpdated processes the received message from Redis.
// It expects messages in the format "add <sessionId> <address> <expires>" or "delete <sessionId>".
// It updates the sessionBackendAddrs manager accordingly.
// If the message is an "add" command, it adds the sessionId and address to the manager.
// If the message is a "delete" command, it removes the sessionId from the manager.
func (rsb *MasterSlaveRedisSessionBasedBackend) sessionBackendUpdated(msg string) {
	zap.L().Info("Received backend update from redis", zap.String("message", msg))
	// Parse the message payload to extract sessionId and address
	// Expected format: "add <sessionId> <address>" or "delete <sessionId>"
	fields := strings.Split(msg, " ")
	if fields[0] == "add" && len(fields) == 4 {
		// Handle session addition
		sessionId := fields[1]
		address := fields[2]
		expires, _ := strconv.ParseInt(fields[3], 10, 32)
		// Convert expires to an integer
		rsb.sessionBackendAddrs.SetBackendAddress(sessionId, address, expires)
		zap.L().Info("Get session backend address from redis", zap.String("sessionId", sessionId), zap.String("address", address))
	} else if fields[0] == "delete" && len(fields) == 2 {
		// Handle session deletion
		sessionId := fields[1]
		err := rsb.sessionBackendAddrs.RemoveBackend(sessionId)
		if err == nil {
			zap.L().Info("Deleted session backend address from redis", zap.String("sessionId", sessionId))
		} else {
			zap.L().Warn("Session backend address not found for deletion in redis", zap.String("sessionId", sessionId))
		}
	}
}

// AddBackend adds a backend for the sessionId and set the expire time.
// It publishes a message to the Redis channel in the format "add <sessionId> <address>".
func (rsb *MasterSlaveRedisSessionBasedBackend) AddBackend(sessionId string, backend Backend, expireSeconds int) {
	zap.L().Info("add backend for session to redis", zap.String("sessionId", sessionId), zap.String("backend", backend.GetAddress()))

	timeout := rsb.timeout
	// check if the expireSeconds is greater than the timeout value
	// if it is, set the timeout value to the expireSeconds
	if float64(expireSeconds) > timeout.Seconds() {
		timeout = time.Duration(expireSeconds) * time.Second
	}
	expire := int64(timeout.Seconds())
	// Set the backend address in Redis with an expiration time
	rsb.ForEachRedis(func(rdb *redis.Client) error {
		return rdb.Publish(rsb.redisChannel, fmt.Sprintf("add %s %s %d", sessionId, backend.GetAddress(), expire)).Err()
	})
}

func (rsb *MasterSlaveRedisSessionBasedBackend) RemoveSession(sessionId string) {
	zap.L().Info("remove backend for session from redis", zap.String("sessionId", sessionId))

	rsb.ForEachRedis(func(rdb *redis.Client) error {
		return rdb.Publish(rsb.redisChannel, fmt.Sprintf("delete %s", sessionId)).Err()
	})
	rsb.sessionBackendAddrs.RemoveBackend(sessionId)
}

func (rsb *MasterSlaveRedisSessionBasedBackend) ForEachRedis(process func(rdb *redis.Client) error) error {
	n := len(rsb.rdbs)

	if n == 0 {
		return fmt.Errorf("no Redis clients available")
	}

	for i := range n {
		index := (rsb.masterIndex + i) % n // Calculate the index of the Redis client to use
		rdb := rsb.rdbs[index]
		if rdb == nil {
			zap.L().Warn("Redis client is nil, skipping", zap.Int("index", index))
			continue // Skip this Redis client if it's nil
		}
		err := process(rdb)
		if err != nil {
			if strings.Contains(err.Error(), "READONLY") {
				zap.L().Warn("Redis is in read-only mode, skipping processing", zap.String("address", rdb.Options().Addr))
			}
		} else {
			if i > 0 {
				rsb.masterIndex = index // Update master index to the next available Redis client
				zap.L().Info("Updated redis master index", zap.Int("masterIndex", rsb.masterIndex))
			}
			return nil // Exit the loop if processing is successful
		}
	}
	return fmt.Errorf("failed to process any Redis client")
}

func NewCompositeSessionBasedBackend(backends []SessionBasedBackend) *CompositeSessionBasedBackend {
	return &CompositeSessionBasedBackend{backends: backends}
}

func (csb *CompositeSessionBasedBackend) GetBackend(sessionId string) (Backend, error) {
	for _, backend := range csb.backends {
		if backend != nil {
			b, err := backend.GetBackend(sessionId)
			if err == nil && b != nil {
				return b, nil
			}
		}
	}
	return nil, fmt.Errorf("no backend found for session %s", sessionId)
}

// AddBackend adds a backend for the sessionId and set the expire time
func (csb *CompositeSessionBasedBackend) AddBackend(sessionId string, backend Backend, expireSeconds int) {
	for _, b := range csb.backends {
		if b != nil {
			b.AddBackend(sessionId, backend, expireSeconds)
		}
	}
}

// RemoveSession removes the backend related with the sessionId
func (csb *CompositeSessionBasedBackend) RemoveSession(sessionId string) {
	for _, b := range csb.backends {
		if b != nil {
			b.RemoveSession(sessionId)
		}
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

