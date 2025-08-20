package main

import (
	"net"
	"testing"
)

func TestGetTransportFromClientTransportMgr(t *testing.T) {
	selfLearnRoute := NewSelfLearnRoute()
	resolver := NewPreConfigHostResolver()

	clientTransMgr := NewClientTransportMgr(NewClientTransportFactory(resolver), selfLearnRoute, func(conn net.Conn) {
	})

	trans_1, err := clientTransMgr.GetTransport("udp", "192.168.100.96", 3088, "")
	if err != nil {
		t.Errorf("Failed to get transport: %v", err)
		return
	}
	for i := 0; i < 10; i++ {
		trans_2, err := clientTransMgr.GetTransport("udp", "192.168.100.96", 3088, "")
		if err != nil {
			t.Errorf("Failed to get transport: %v", err)
			return
		}

		if trans_1 != trans_2 {
			t.Errorf("Expected the same transport, but got different ones")
		}
	}

}

