package main

import (
	"fmt"
	"net"
	"net/url"
	"sync/atomic"
)

type Backend interface {
	Send(msg *Message) error
}

type RoundRobinBackend struct {
	index    int32
	backends []Backend
}

type UDPBackend struct {
	udpConn *net.UDPConn
	msgChannel chan *Message
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
			backend, err := NewUDPBackend(u.Host)
			if err != nil {
				return nil, err
			}
			rrBackend.AddBackend(backend)
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

	b := &UDPBackend{udpConn: udpConn, msgChannel: make( chan *Message, 1000 ) }
	go b.takeAndSendToBackend()
	return b, nil
}

func (b *UDPBackend) Send(msg *Message) error {
	b.msgChannel <- msg
	return nil
}

func (b *UDPBackend) takeAndSendToBackend() {
	for {
		select {
		case msg := <-b.msgChannel:
			msg.Write( b.udpConn )
		}
	}
}

func NewRoundRobinBackend() *RoundRobinBackend {
	return &RoundRobinBackend{index: 0, backends: make([]Backend, 0)}
}

func (rb *RoundRobinBackend) AddBackend(backend Backend) {
	rb.backends = append(rb.backends, backend)
}

func (rb *RoundRobinBackend) Send(msg *Message) error {
	backend, err := rb.getNextBackend()
	if err != nil {
		return err
	}
	return backend.Send(msg)
}

func (rb *RoundRobinBackend) getNextBackend() (Backend, error) {
	for {
		old := atomic.LoadInt32( &rb.index )
		if atomic.CompareAndSwapInt32(&rb.index, old, old+1) {
			n := int32(len(rb.backends))
			var index int32 = (old + 1) % n
			return rb.backends[index], nil
		}
	}
}
