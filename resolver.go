package main

import (
	"fmt"
	"net"
)

type HostResolver struct {
	// Name to IP mapping
	hostIPs map[string]string
}

func NewHostResolver() *HostResolver {
	return &HostResolver{hostIPs: make(map[string]string)}
}

func (hr *HostResolver) AddHostIP(name string, ip string) {
	hr.hostIPs[name] = ip
}

// GetIp get the IP by hostname
func (hr *HostResolver) GetIp(name string) (string, error) {

	if net.ParseIP(name) != nil {
		return name, nil
	}

	if ip, ok := hr.hostIPs[name]; ok {
		return ip, nil
	}
	ips, err := net.LookupIP(name)
	if err == nil && len(ips) > 0 {
		return ips[0].String(), nil
	}
	return "", fmt.Errorf("Fail to find IP of %s", name)
}
