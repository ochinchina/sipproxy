package main

import (
	"sync"

	"go.uber.org/zap"
)

type SelfLearnRoute struct {
	sync.Mutex

	// map between destination ip/host and local server transport
	route map[string]ServerTransport
}

func NewSelfLearnRoute() *SelfLearnRoute {
	return &SelfLearnRoute{route: make(map[string]ServerTransport)}
}

func (sl *SelfLearnRoute) AddRoute(ip string, transport ServerTransport) {
	sl.Lock()
	defer sl.Unlock()

	old, ok := sl.route[ip]
	if ok && sl.isSameTransport(old, transport) {
		return
	}
	zap.L().Info("Add route for ip", zap.String("ip", ip), zap.String("protocol", transport.GetProtocol()), zap.String("addr", transport.GetAddress()), zap.Int("port", transport.GetPort()))
	sl.route[ip] = transport
}

func (sl *SelfLearnRoute) isSameTransport(transport1 ServerTransport, transport2 ServerTransport) bool {
	return transport1.GetProtocol() == transport2.GetProtocol() &&
		transport1.GetAddress() == transport2.GetAddress() &&
		transport1.GetPort() == transport2.GetPort()
}

func (sl *SelfLearnRoute) GetRoute(ip string) (ServerTransport, bool) {
	sl.Lock()
	defer sl.Unlock()

	transport, ok := sl.route[ip]
	if ok {
		zap.L().Info("Succeed to get route for ip", zap.String("ip", ip), zap.String("protocol", transport.GetProtocol()), zap.String("addr", transport.GetAddress()), zap.Int("port", transport.GetPort()))
	}
	return transport, ok
}

