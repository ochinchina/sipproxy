package main

import (
	"fmt"
	"testing"
)

func TestSIPURIParse(t *testing.T) {
	sipUri, err := ParseSipURI("sip:alice@atlanta.com;maddr=239.255.255.1;lr;ttl=15?subject=project%20x&priority=urgent")
	if err != nil {
		t.Fail()
	}
	if sipUri.Host != "atlanta.com" || sipUri.GetPort() != 5060 {
		t.Fail()
	}
	if v, err := sipUri.GetParameter("ttl"); err != nil && v != "15" {
		t.Fail()
	}
}

func TestSIPURIAddParameter(t *testing.T) {
	sipUri, err := ParseSipURI("sip:alice@atlanta.com" )
	if err != nil {
		t.Fail()
	}
	sipUri.AddParameter( "lr", "" )
	sipUri.AddParameter( "ttl", "100" )
	sipUri, err = ParseSipURI( sipUri.String() )
	if v, err := sipUri.GetParameter("ttl"); err != nil && v != "100" {
                t.Fail()
        }
	if _, err := sipUri.GetParameter( "lr" ); err != nil {
		t.Fail()
	}
	fmt.Println( sipUri )
}
