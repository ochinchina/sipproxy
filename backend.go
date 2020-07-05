package main

import (
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
	index      int
	backends   []Backend
	backendMap map[string]Backend
}

type UDPBackend struct {
	address    string
	udpConn    *net.UDPConn
	msgChannel chan *Message
}

var dynamicHostResolver *DynamicHostResolver

func init() {
	dynamicHostResolver = NewDynamicHostResolver(2)
}

func CreateBackend(addresses []string) (Backend, error) {
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
					backend, err := NewUDPBackend(u.Host)
					if err != nil {
						return nil, err
					}
					rrBackend.AddBackend(backend)
				} else {
					dynamicHostResolver.ResolveHost(host, func(hostname string, newIPs []string, removedIPs []string) {
						rrBackend.hostIPChanged("udp", hostname, newIPs, removedIPs, port)
					})
				}
			}
		} else {
			return nil, fmt.Errorf("Unsupported protocol %s", u.Scheme)
		}
	}
	return rrBackend, nil

}

func NewUDPBackend(hostport string) (*UDPBackend, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", hostport)
	if err != nil {
		return nil, err
	}
	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	b := &UDPBackend{address: hostport, udpConn: udpConn, msgChannel: make(chan *Message, 1000)}
	go b.takeAndSendToBackend()
	return b, nil
}

func (b *UDPBackend) Send(msg *Message) error {
	b.msgChannel <- msg
	return nil
}

func (b *UDPBackend) GetAddress() string {
	return b.address
}

func (b *UDPBackend) takeAndSendToBackend() {
	for {
		select {
		case msg, more := <-b.msgChannel:
			if more {
				_, err := msg.Write(b.udpConn)
				if err == nil {
					log.WithFields(log.Fields{"address": b.address}).Info("Succeed send message to backend")
				} else {
					log.WithFields(log.Fields{"address": b.address}).Error("Fail to send message to backend")
				}
			} else {
				log.WithFields(log.Fields{"address": b.address}).Info("No message to backend anymore")
				return
			}
		}
	}
}

func (b *UDPBackend) Close() {
	close(b.msgChannel)
}

func NewRoundRobinBackend() *RoundRobinBackend {
	return &RoundRobinBackend{index: 0,
		backends:   make([]Backend, 0),
		backendMap: make(map[string]Backend)}
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
	backend, err := rb.getNextBackend()
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Fail to send message to any backend")
		return err
	}
	return backend.Send(msg)
}

func (rb *RoundRobinBackend) GetAddress() string {
	return ""
}

func (rb *RoundRobinBackend) getNextBackend() (Backend, error) {
	rb.Lock()
	defer rb.Unlock()
	n := len(rb.backends)
	if n <= 0 {
		return nil, fmt.Errorf("No backend available")
	}
	rb.index = (rb.index + 1) % n
	return rb.backends[rb.index], nil

}

func (rb *RoundRobinBackend) hostIPChanged(protocol string, hostname string, newIPs []string, removedIPs []string, port string) {
	for _, ip := range newIPs {
		log.WithFields(log.Fields{"ip": ip}).Info("find a new IP for ", hostname)
		if protocol == "udp" {
			hostport := fmt.Sprintf("%s:%s", ip, port)
			if isIPv6(ip) {
				hostport = fmt.Sprintf("[%s]:%s", ip, port)
			}
			backend, err := NewUDPBackend(hostport)
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
