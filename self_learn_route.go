package main

import (
	log "github.com/sirupsen/logrus"
)

type SelfLearnRoute struct {
	// map between destination ip/host and local server transport
	route map[string]ServerTransport
}

func NewSelfLearnRoute() *SelfLearnRoute {
	return &SelfLearnRoute{route: make(map[string]ServerTransport)}
}

func (sl *SelfLearnRoute) AddRoute(ip string, transport ServerTransport) {
	old, ok := sl.route[ip]
	if ok && sl.isSameTransport(old, transport) {
		return
	}
	log.WithFields(log.Fields{"protocol": transport.GetProtocol(), "addr": transport.GetAddress(), "port": transport.GetPort()}).Info("Add route for ", ip)
	sl.route[ip] = transport
}

func (sl *SelfLearnRoute) isSameTransport(transport1 ServerTransport, transport2 ServerTransport) bool {
	return transport1.GetProtocol() == transport2.GetProtocol() &&
		transport1.GetAddress() == transport2.GetAddress() &&
		transport1.GetPort() == transport2.GetPort()
}

func (sl *SelfLearnRoute) GetRoute(ip string) (ServerTransport, bool) {
	transport, ok := sl.route[ip]
	if ok {
		log.WithFields(log.Fields{"protocol": transport.GetProtocol(), "addr": transport.GetAddress(), "port": transport.GetPort()}).Info("Succeed to get route for ", ip)
	}
	return transport, ok
}
