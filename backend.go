package main

import (
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"net/url"
	"strings"
	"sync"
)

type Backend interface {
	Send(msg *Message) error
	GetAddress() string
	Close()
}

type RoundRobinBackend struct {
	sync.Mutex
	msgChannel chan *Message
	index      int
	backends   []Backend
	backendMap map[string]Backend
}

type UDPBackend struct {
	backendAddr *net.UDPAddr
	udpConn     *net.UDPConn
}

var dynamicHostResolver *DynamicHostResolver

func init() {
	dynamicHostResolver = NewDynamicHostResolver(2)
}

func CreateBackend(localhostport string, addresses []string) (Backend, error) {
	if len(addresses) <= 0 {
		return nil, nil
	}
	rrBackend := NewRoundRobinBackend()
	for _, address := range addresses {
		u, err := url.Parse(address)
		if err != nil {
			return nil, err
		}

		if u.Scheme == "udp" {
			pos := strings.LastIndex(u.Host, ":")
			if pos != -1 {
				host := u.Host[0:pos]
				port := u.Host[pos+1:]
				if isIPAddress(host) {
					backend, err := NewUDPBackend( localhostport, u.Host)
					if err != nil {
						return nil, err
					}
					rrBackend.AddBackend(backend)
				} else {
					dynamicHostResolver.ResolveHost(host, func(hostname string, newIPs []string, removedIPs []string) {
						rrBackend.hostIPChanged("udp", localhostport, hostname, newIPs, removedIPs, port)
					})
				}
			}
		} else {
			return nil, fmt.Errorf("Unsupported protocol %s", u.Scheme)
		}
	}
	return rrBackend, nil

}

func NewUDPBackend( localhostport string, hostport string) (*UDPBackend, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", hostport)
	if err != nil {
		return nil, err
	}
	addr, err := net.ResolveUDPAddr("udp", localhostport )
	if err != nil {
		return nil, err
	}
	udpConn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	b := &UDPBackend{backendAddr: udpAddr, udpConn: udpConn}
	return b, nil
}

func (b *UDPBackend) Send(msg *Message) error {
	buf := bytes.NewBuffer(make([]byte, 0))
	n, err := msg.Write(buf)

	if err != nil {
		log.Error("Fail to encode message")
		return err
	}

	n, err = b.udpConn.WriteToUDP(buf.Bytes(), b.backendAddr)
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

func NewRoundRobinBackend() *RoundRobinBackend {
	rb := &RoundRobinBackend{msgChannel: make(chan *Message, 1000),
		index:      0,
		backends:   make([]Backend, 0),
		backendMap: make(map[string]Backend)}
	go rb.takeAndSendMessage()
	return rb
}

func (rb *RoundRobinBackend) takeAndSendMessage() {
	for {
		select {
		case msg, more := <-rb.msgChannel:
			if more {
				rb.doSendMessage(msg)
			} else {
				return
			}
		}
	}

}

func (rb *RoundRobinBackend) doSendMessage(msg *Message) {
	index, err := rb.getNextBackendIndex()
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Fail to send message")
		return
	}

	n := rb.getBackendCount()
	for ; n > 0; n-- {
		backend, err := rb.getBackend(index)
		index++
		if err == nil {
			err = backend.Send(msg)
			if err == nil {
				break
			}
		}
	}

}
func (rb *RoundRobinBackend) AddBackend(backend Backend) {
	rb.Lock()
	defer rb.Unlock()
	rb.backends = append(rb.backends, backend)
	rb.backendMap[backend.GetAddress()] = backend
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
	}
}

func (rb *RoundRobinBackend) Send(msg *Message) error {
	rb.msgChannel <- msg
	return nil
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
