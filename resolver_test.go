package main

import (
	"testing"
)

func TestResolveHost(t *testing.T) {
	resolver := NewPreConfigHostResolver()
	resolver.AddHostIP("atlanta.example.com", "192.0.2.101")
	resolver.AddHostIP("atlanta.example.com", "192.0.2.102")
	resolver.AddHostIP("biloxi.example.com", "192.0.2.201")
	resolver.AddHostIP("lrf.sip.ims.telecom.pt", "10.111.129.18")

	ips, err := resolver.GetIps("biloxi.example.com")
	if err != nil {
		t.Error("fail to get IP of biloxi.example.com")
	}
	if len(ips) != 1 || ips[0] != "192.0.2.201" {
		t.Error("the ip of biloxi.example.com is not 192.0.2.201")
	}

	ips, err = resolver.GetIps("atlanta.example.com")

	if err != nil {
		t.Error("fail to get IP of atlanta.example.com")
	}

	if len(ips) != 2 || ips[0] != "192.0.2.101" {
		t.Error("the first ip of atlanta.example.com is not 192.0.2.101")
	}

	ips, err = resolver.GetIps("lrf.sip.ims.telecom.pt")

	if err != nil {
		t.Fail()
	}
	if len(ips) != 1 || ips[0] != "10.111.129.18" {
		t.Fail()
	}

	ips, err = resolver.GetIps("unknown.example.com")
	if err != nil {
		t.Fail()
	}
	if len(ips) != 1 || ips[0] != "unknown.example.com" {
		t.Fail()
	}
}

