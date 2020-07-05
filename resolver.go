package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	log "github.com/sirupsen/logrus"
)

type PreConfigHostResolver struct {
	// Name to IP mapping
	hostIPs map[string]string
}

func NewPreConfigHostResolver() *PreConfigHostResolver {
	return &PreConfigHostResolver{hostIPs: make(map[string]string)}
}

func (hr *PreConfigHostResolver) AddHostIP(name string, ip string) {
	hr.hostIPs[name] = ip
}

// GetIp get the IP by hostname
func (hr *PreConfigHostResolver) GetIp(name string) (string, error) {

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

type IPResolvedCallback = func(hostname string, newIPs []string, removedIPs []string)

type addressWithCallback struct {
	addrs []string
	//times of resolved
	failed   int
	callback IPResolvedCallback
}

// DynamicHostResolver dynamically resolve the host name to IP addresses
type DynamicHostResolver struct {
	sync.Mutex
	//resolve interval
	interval time.Duration

	// 0: no stop, 1: stop the resolve
	stop int32

	hostIPs map[string]*addressWithCallback
}

func NewDynamicHostResolver(interval int) *DynamicHostResolver {
	r := &DynamicHostResolver{interval: time.Duration(interval) * time.Second,
		stop:    0,
		hostIPs: make(map[string]*addressWithCallback)}
	go r.periodicalResolve()
	return r
}

func (r *DynamicHostResolver) ResolveHost(addr string, callback IPResolvedCallback) {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.hostIPs[addr]; !ok {
		ips, err := r.doResolve(addr)
		if err == nil {
			r.hostIPs[addr] = &addressWithCallback{addrs: ips, failed: 0, callback: callback}
			callback(addr, ips, make([]string, 0))
		} else {
			r.hostIPs[addr] = &addressWithCallback{addrs: make([]string, 0), failed: 0, callback: callback}
		}
	}
}

func (r *DynamicHostResolver) StopResolve(hostname string) {
	r.Lock()
	defer r.Unlock()

	if _, ok := r.hostIPs[hostname]; ok {
		delete(r.hostIPs, hostname)

	}
}

func (r *DynamicHostResolver) getHostnames() []string {
	r.Lock()
	defer r.Unlock()

	hostnames := make([]string, 0)

	for hostname, _ := range r.hostIPs {
		hostnames = append(hostnames, hostname)
	}
	return hostnames
}

// Stop stop the hostname resolve
func (r *DynamicHostResolver) Stop() {
	if atomic.CompareAndSwapInt32(&r.stop, 0, 1) {
		log.Info("stop the hostname to IP resolve")
	}
}

func (r *DynamicHostResolver) isStopped() bool {
	return atomic.LoadInt32(&r.stop) != 0
}

func (r *DynamicHostResolver) GetAddrsOfHost(hostname string) []string {
	result := make([]string, 0)
	r.Lock()
	defer r.Unlock()

	if v, ok := r.hostIPs[hostname]; ok {
		result = append(result, v.addrs...)
	}
	return result
}
func (r *DynamicHostResolver) periodicalResolve() {
	for !r.isStopped() {
		hostnames := r.getHostnames()

		for _, hostname := range hostnames {
			addrs, err := r.doResolve(hostname)
			if err != nil {
				log.WithFields(log.Fields{"hostname": hostname}).Error("Fail to resolve host to IP")
			}
			r.addressResolved(hostname, addrs, err)
		}
		time.Sleep(r.interval)
	}
}

func (r *DynamicHostResolver) addressResolved(hostname string, addrs []string, err error) {
	r.Lock()
	defer r.Unlock()
	if entry, ok := r.hostIPs[hostname]; ok {
		if err != nil {
			entry.failed += 1
			if entry.failed > 3 && len(entry.addrs) > 0 {
				newAddrs := make([]string, 0)
				removedAddrs := entry.addrs
				entry.addrs = newAddrs
				log.WithFields(log.Fields{"hostname": hostname, "failed": entry.failed}).Error("the failed times for resolving hostname exceeds 3")
				entry.failed = 0
				go entry.callback(hostname, newAddrs, removedAddrs)
			}
		} else {
			newAddrs := strArraySub(addrs, entry.addrs)
			removedAddrs := strArraySub(entry.addrs, addrs)
			entry.failed = 0
			entry.addrs = addrs
			if len(newAddrs) > 0 || len(removedAddrs) > 0 {
				log.WithFields(log.Fields{"hostname": hostname, "newAddrs": strings.Join(newAddrs, ","), "removedAddrs": strings.Join(removedAddrs, ",")}).Info("the ip address of host is changed")
				go entry.callback(hostname, newAddrs, removedAddrs)
			}
		}
	}
}

func (r *DynamicHostResolver) doResolve(hostname string) ([]string, error) {

	ips, err := net.LookupIP(hostname)

	if err != nil {
		return nil, err
	}

	result := make([]string, 0)
	for _, ip := range ips {
		s := ip.String()
		if strings.Index(s, ":") != -1 {
			s = fmt.Sprintf("[%s]", s)
		}
		result = append(result, s )
	}
	return result, nil
}
