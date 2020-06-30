package main

import (
	"testing"
)

func TestSIPURIParse(t *testing.T) {
	sipUri, err := ParseSipURI("sip:alice@atlanta.com;maddr=239.255.255.1;lr;ttl=15?subject=project%20x&priority=urgent")
	if err != nil {
		t.Fail()
	}
	if v, err := sipUri.GetParameter("ttl"); err != nil && v != "15" {
		t.Fail()
	}

}
