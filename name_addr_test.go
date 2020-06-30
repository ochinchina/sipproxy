package main

import (
	"testing"
)

func TestNameAddrParse(t *testing.T) {
	nameAddr, err := ParseNameAddr("Server10<sip:server10.biloxi.com;lr>")
	if err != nil {
		t.Fail()
	}
	if nameAddr.DisplayName != "Server10" {
		t.Fail()
	}
	uri, _ := nameAddr.Addr.GetSIPURI()
	if uri.String() != "sip:server10.biloxi.com;lr" {
		t.Fail()
	}
	if nameAddr.String() != "Server10<sip:server10.biloxi.com;lr>" {
		t.Fail()
	}
}
