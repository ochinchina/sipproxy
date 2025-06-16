package main

import (
	"testing"
)

func TestResolveHost(t *testing.T) {
	resolver := NewPreConfigHostResolver()
	resolver.AddHostIP("atlanta.example.com", "192.0.2.101")
	resolver.AddHostIP("biloxi.example.com", "192.0.2.201")
	resolver.AddHostIP("lrf.sip.ims.telecom.pt", "10.111.129.18")

	ip, err := resolver.GetIp("biloxi.example.com")
	if err != nil {
		t.Fail()
	}
	if ip != "192.0.2.201" {
		t.Fail()
	}

	ip, err = resolver.GetIp("lrf.sip.ims.telecom.pt")
	if err != nil {
		t.Fail()
	}
	if ip != "10.111.129.18" {
		t.Fail()
	}
}

