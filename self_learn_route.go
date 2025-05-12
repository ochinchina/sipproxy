package main

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

type SelfLearnItem struct {
	serverTransport ServerTransport
	expire          int64
}
type SelfLearnRoute struct {
	sync.Mutex
	// cleanInterval is the interval to clean the expired items
	// It is used to clean the expired items in the map
	cleanInterval int64
	// lastCleanTime is the last time we clean the expired items
	lastCleanTime int64
	// expire is the expire time of the item in the map
	// It is used to check if the item is expired or not
	expire int64
	// map between destination ip/host and local server transport
	route map[string]SelfLearnItem
}

func NewSelfLearnRoute() *SelfLearnRoute {
	return &SelfLearnRoute{
		cleanInterval: 60,
		lastCleanTime: time.Now().Unix(),
		// expire time is 20 minutes
		expire: 20 * 60,
		route:  make(map[string]SelfLearnItem),
	}
}

// SelfLearnRoute is used to learn the route of the server
// It is used to learn the route of the server when the server is not in the config file
func (sl *SelfLearnRoute) AddRoute(ip string, transport ServerTransport) {
	sl.Lock()
	defer sl.Unlock()

	sl.cleanExpires()

	zap.L().Info("Add route for ip", zap.String("ip", ip), zap.String("protocol", transport.GetProtocol()), zap.String("addr", transport.GetAddress()), zap.Int("port", transport.GetPort()))
	sl.route[ip] = SelfLearnItem{serverTransport: transport, expire: time.Now().Unix() + sl.expire}
}

func (sl *SelfLearnRoute) isSameTransport(transport1 ServerTransport, transport2 ServerTransport) bool {
	return transport1.GetProtocol() == transport2.GetProtocol() &&
		transport1.GetAddress() == transport2.GetAddress() &&
		transport1.GetPort() == transport2.GetPort()
}

func (sl *SelfLearnRoute) GetRoute(ip string) (ServerTransport, bool) {
	sl.Lock()
	defer sl.Unlock()

	item, ok := sl.route[ip]
	if ok {
		transport := item.serverTransport
		zap.L().Info("Succeed to get route for ip", zap.String("ip", ip), zap.String("protocol", transport.GetProtocol()), zap.String("addr", transport.GetAddress()), zap.Int("port", transport.GetPort()))
	}
	return item.serverTransport, ok
}

// GetRouteAddress returns the address of the route for the given ip
// It returns the address of the route for the given ip, empty string if not found
func (sl *SelfLearnRoute) GetRouteAddress(ip string) string {
	transport, ok := sl.GetRoute(ip)
	if ok {
		return transport.GetAddress()
	} else {
		return ""
	}
}

func (sl *SelfLearnRoute) cleanExpires() {
	now := time.Now().Unix()

	// Check if it's time to clean expired items
	if now-sl.lastCleanTime < sl.cleanInterval || len(sl.route) < 10000 {
		return
	}
	sl.lastCleanTime = now

	expiredKeys := make([]string, 0)

	// Iterate over the map to find expired items
	for ip, item := range sl.route {
		if item.expire < now {
			expiredKeys = append(expiredKeys, ip)
		}
	}

	// Remove expired items from the map
	for _, ip := range expiredKeys {
		delete(sl.route, ip)
		zap.L().Info("Remove expired route for ip", zap.String("ip", ip))
	}
}

