package main

type SelfLearnRoute struct {
	// map between destination ip/host and local server transport
	route map[string]ServerTransport
}

func NewSelfLearnRoute() *SelfLearnRoute {
	return &SelfLearnRoute{route: make(map[string]ServerTransport)}
}

func (sl *SelfLearnRoute) AddRoute(ip string, transport ServerTransport) {
	sl.route[ip] = transport
}

func (sl *SelfLearnRoute) GetRoute(ip string) (ServerTransport, bool) {
	transport, ok := sl.route[ip]
	return transport, ok
}
