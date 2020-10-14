package main

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

type BackendChangeListener interface {
	HandleBackendAdded(backend Backend, parent *RoundRobinBackend)
	HandleBackendRemoved(backend Backend, parent *RoundRobinBackend)
}

type BackendChangeListenerMgr struct {
	sync.Mutex
	listeners []BackendChangeListener
}

func NewBackendChangeListenerMgr() *BackendChangeListenerMgr {
	return &BackendChangeListenerMgr{listeners: make([]BackendChangeListener, 0)}
}

func (bm *BackendChangeListenerMgr) AddChangeListener(listener BackendChangeListener) {
	bm.Lock()
	defer bm.Unlock()

	bm.listeners = append(bm.listeners, listener)
}

func (bm *BackendChangeListenerMgr) HandleBackendAdded(backend Backend, parent *RoundRobinBackend) {
	bm.Lock()
	defer bm.Unlock()

	for _, listener := range bm.listeners {
		listener.HandleBackendAdded(backend, parent)
	}
}

func (bm *BackendChangeListenerMgr) HandleBackendRemoved(backend Backend, parent *RoundRobinBackend) {
	bm.Lock()
	defer bm.Unlock()

	for _, listener := range bm.listeners {
		listener.HandleBackendRemoved(backend, parent)
	}
}

type Backend interface {
	Send(msg *Message) error
	GetAddress() string
	Close()
}
type RoundRobinBackend struct {
	sync.Mutex
	index                    int
	backends                 []Backend
	backendMap               map[string]Backend
	backendChangeListenerMgr *BackendChangeListenerMgr
	dialogBasedBackend       *DialogBasedBackend
}

type UDPBackend struct {
	backendAddr *net.UDPAddr
	udpConn     *net.UDPConn
}

type TCPBackend struct {
	localAddr   string
	backendAddr string
	conn        net.Conn
}

var dynamicHostResolver *DynamicHostResolver

func init() {
	dynamicHostResolver = NewDynamicHostResolver(2)
}

func CreateRoundRobinBackend(localhostport string, addresses []string) (*RoundRobinBackend, error) {
	if len(addresses) <= 0 {
		return nil, fmt.Errorf("No address")
	}
	rrBackend := NewRoundRobinBackend()
	for _, address := range addresses {
		u, err := url.Parse(address)
		if err != nil {
			return nil, err
		}

		if u.Scheme == "udp" || u.Scheme == "tcp" {
			pos := strings.LastIndex(u.Host, ":")
			if pos != -1 {
				host := u.Host[0:pos]
				port := u.Host[pos+1:]
				if isIPAddress(host) {
					var backend Backend
					var err error
					if u.Scheme == "udp" {
						backend, err = NewUDPBackend(localhostport, u.Host)
					} else {
						backend, err = NewTCPBackend(localhostport, u.Host)
					}
					if err != nil {
						return nil, err
					}
					rrBackend.AddBackend(backend)
				} else {
					dynamicHostResolver.ResolveHost(host, func(hostname string, newIPs []string, removedIPs []string) {
						rrBackend.hostIPChanged(u.Scheme, localhostport, hostname, newIPs, removedIPs, port)
					})
				}
			}
		} else {
			return nil, fmt.Errorf("Unsupported protocol %s", u.Scheme)
		}
	}
	return rrBackend, nil
}

func NewUDPBackend(localhostport string, hostport string) (*UDPBackend, error) {
	backendAddr, err := net.ResolveUDPAddr("udp", hostport)
	if err != nil {
		return nil, err
	}
	localAddr, err := net.ResolveUDPAddr("udp", localhostport)
	if err != nil {
		return nil, err
	}
	udpConn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, err
	}

	b := &UDPBackend{backendAddr: backendAddr, udpConn: udpConn}
	return b, nil
}

func (b *UDPBackend) Send(msg *Message) error {
	bytes, err := msg.Bytes()

	if err != nil {
		return err
	}

	n, err := b.udpConn.WriteToUDP(bytes, b.backendAddr)
	if err == nil {
		log.WithFields(log.Fields{"address": b.backendAddr, "bytes": n}).Info("Succeed send message to backend")
	} else {
		log.WithFields(log.Fields{"address": b.backendAddr}).Error("Fail to send message to backend")
	}

	return err
}

func (b *UDPBackend) GetAddress() string {
	return b.backendAddr.String()
}

func (b *UDPBackend) Close() {
	err := b.udpConn.Close()
	if err == nil {
		log.WithFields(log.Fields{"address": b.udpConn.RemoteAddr()}).Info("Succeed to close udp backend")
	} else {
		log.WithFields(log.Fields{"address": b.udpConn.RemoteAddr()}).Error("Fail to close udp backend")
	}
}

func NewTCPBackend(localhostport string, hostport string) (*TCPBackend, error) {
	return &TCPBackend{localAddr: localhostport,
		backendAddr: hostport,
		conn:        nil}, nil
}

func (t *TCPBackend) Send(msg *Message) error {
	b, err := msg.Bytes()
	if err != nil {
		return err
	}

	for i := 0; i < 2; i++ {
		if t.conn == nil {
			conn, err := net.Dial("tcp", t.backendAddr)
			if err != nil {
				log.WithFields(log.Fields{"backendAddr": t.backendAddr}).Error("Fail to connect backend")
				return err
			}
			t.conn = conn
		}
		_, err := t.conn.Write(b)
		if err == nil {
			log.WithFields(log.Fields{"backendAddr": t.backendAddr}).Debug("Succeed to send message to backend")
			return nil
		}
		log.WithFields(log.Fields{"backendAddr": t.backendAddr}).Debug("Fail to send message to backend")
		t.conn.Close()
		t.conn = nil
	}
	return fmt.Errorf("Fail to send message to backend ", t.backendAddr)
}

func (t *TCPBackend) GetAddress() string {
	return t.backendAddr
}

func (t *TCPBackend) Close() {
	if t.conn != nil {
		t.conn.Close()
	}
}

func NewRoundRobinBackend() *RoundRobinBackend {
	rb := &RoundRobinBackend{index: 0,
		backends:                 make([]Backend, 0),
		backendMap:               make(map[string]Backend),
		backendChangeListenerMgr: NewBackendChangeListenerMgr()}
	return rb
}

func (rb *RoundRobinBackend) AddBackend(backend Backend) {
	rb.Lock()
	defer rb.Unlock()
	rb.backends = append(rb.backends, backend)
	rb.backendMap[backend.GetAddress()] = backend
	rb.backendChangeListenerMgr.HandleBackendAdded(backend, rb)
}

func (rb *RoundRobinBackend) GetBackend(address string) (Backend, error) {
	rb.Lock()
	defer rb.Unlock()

	if v, ok := rb.backendMap[address]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("Fail to find backend by %s", address)
}

func (rb *RoundRobinBackend) RemoveBackend(address string) {
	rb.Lock()
	defer rb.Unlock()
	if backend, ok := rb.backendMap[address]; ok {
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
		rb.backendChangeListenerMgr.HandleBackendRemoved(backend, rb)
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

func (rb *RoundRobinBackend) Send(msg *Message) error {
	index, err := rb.getNextBackendIndex()
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Fail to send message")
		return errors.New("Fail to get next backend")
	}

	n := rb.getBackendCount()
	for ; n > 0; n-- {
		backend, err := rb.getBackend(index)
		index++
		if err == nil {
			return backend.Send(msg)
		}
	}
	return errors.New("Fail to send msg to all the backend")
}

func (rb *RoundRobinBackend) GetAddress() string {
	return ""
}

func (rb *RoundRobinBackend) getNextBackendIndex() (int, error) {
	rb.Lock()
	defer rb.Unlock()
	n := len(rb.backends)
	if n <= 0 {
		return 0, fmt.Errorf("No backend available")
	}
	rb.index = (rb.index + 1) % n
	return rb.index, nil
}

func (rb *RoundRobinBackend) getBackend(index int) (Backend, error) {
	rb.Lock()
	defer rb.Unlock()
	n := len(rb.backends)
	if n <= 0 {
		return nil, fmt.Errorf("No backend available at %d", index)
	}
	return rb.backends[index%n], nil
}

func (rb *RoundRobinBackend) getBackendCount() int {
	rb.Lock()
	defer rb.Unlock()
	return len(rb.backends)

}

func (rb *RoundRobinBackend) AddBackendChangeListener(listener BackendChangeListener) {
	rb.backendChangeListenerMgr.AddChangeListener(listener)
	rb.Lock()
	defer rb.Unlock()
	for _, backend := range rb.backendMap {
		listener.HandleBackendAdded(backend, rb)
	}
}

func (rb *RoundRobinBackend) hostIPChanged(protocol string, localhostport, hostname string, newIPs []string, removedIPs []string, port string) {
	for _, ip := range newIPs {
		log.WithFields(log.Fields{"ip": ip}).Info("find a new IP for ", hostname)
		if protocol == "udp" {
			hostport := fmt.Sprintf("%s:%s", ip, port)
			if isIPv6(ip) {
				hostport = fmt.Sprintf("[%s]:%s", ip, port)
			}
			backend, err := NewUDPBackend(localhostport, hostport)
			if err == nil {
				rb.AddBackend(backend)
			}
		}
	}
	for _, ip := range removedIPs {
		log.WithFields(log.Fields{"ip": ip}).Info("remve ip for ", hostname)
		hostport := fmt.Sprintf("%s:%s", ip, port)
		if isIPv6(ip) {
			hostport = fmt.Sprintf("[%s]:%s", ip, port)
		}
		rb.RemoveBackend(hostport)
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
type DialogBasedBackend struct {
	timeout time.Duration
	// map between dialog and the backend
	backends map[string]*ExpireBackend
}

func NewDialogBasedBackend(timeoutSeconds int64) *DialogBasedBackend {
	return &DialogBasedBackend{timeout: time.Duration(timeoutSeconds) * time.Second,
		backends: make(map[string]*ExpireBackend)}
}

func (dbb *DialogBasedBackend) GetBackend(dialog string) (Backend, error) {
	if value, ok := dbb.backends[dialog]; ok {
		if value.expire.After(time.Now()) {
			return value.backend, nil
		}
		delete(dbb.backends, dialog)
	}
	return nil, fmt.Errorf("No backend related with dialog %s", dialog)

}

func (dbb *DialogBasedBackend) AddBackend(dialog string, backend Backend) {
	expire := time.Now().Add(dbb.timeout)
	dbb.backends[dialog] = &ExpireBackend{backend: backend, expire: expire}
	dbb.cleanExpiredDialog()
}

func (dbb *DialogBasedBackend) RemoveDialog(dialog string) {
	delete(dbb.backends, dialog)
}

func (dbb *DialogBasedBackend) cleanExpiredDialog() {
	expiredDialogs := make(map[string]string)
	for k, v := range dbb.backends {
		if v.expire.Before(time.Now()) {
			expiredDialogs[k] = k
		}
	}

	for k, _ := range expiredDialogs {
		delete(dbb.backends, k)
	}
}
