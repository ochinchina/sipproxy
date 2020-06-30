package main

import (
	"fmt"
	"testing"
)

func TestParseVia(t *testing.T) {
	via, err := ParseVia("SIP/2.0/TLS client.biloxi.example.com:5061;branch=z9hG4bKnashds7;ttl=100,SIP/2.0/UDP client1.biloxi.example.com:5061;branch=z9hG4bKnashdm8;ttl=150")
	if err != nil {
		t.Fail()
	}

	if via.Size() != 2 {
		t.Fail()
	}

	viaParam, _ := via.GetParam(0)

	if v, err := viaParam.GetParam("branch"); err != nil || v != "z9hG4bKnashds7" {
		t.Fail()
	}

	if viaParam.Host != "client.biloxi.example.com" || viaParam.GetPort() != 5061 {
		t.Fail()
	}

}

func TestCreateViaParam(t *testing.T) {
	viaParam := CreateViaParam("UDP", "client.biloxi.example.com", 0)
	branch, _ := CreateBranch()
	viaParam.SetBranch(branch)
	fmt.Println(viaParam.String())
}
