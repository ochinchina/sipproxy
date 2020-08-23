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

func TestParseVia2(t *testing.T) {
	via, err := ParseVia("SIP/2.0/UDP 10.68.103.193:5060;branch=z9hG4bK-343437-6119ae735a31a77df85d22346c030fa7;rport")
	if err != nil {
		t.Fail()
	}
	fmt.Println(via)
}
func TestCreateViaParam(t *testing.T) {
	viaParam := CreateViaParam("UDP", "client.biloxi.example.com", 0)
	branch, _ := CreateBranch()
	viaParam.SetBranch(branch)
	fmt.Println(viaParam.String())
}

func TestParseVia3(t *testing.T) {
	via, err := ParseVia("SIP/2.0/UDP 10.244.2.174:5060;branch=z9hG4bK-343034-8ae1b84c48a3a15106a4bcade0030d29;rport=9999;received=192.168.1.72")
	if err != nil {
		t.Fail()
	}

	viaParam, err := via.GetParam(0)
	if err != nil {
		t.Fail()
	}

	host, err := viaParam.GetReceived()

	if err != nil {
		t.Fail()
	}

	port, err := viaParam.GetRPort()
	if err != nil {
		port = viaParam.GetPort()
	}

	if viaParam.HasParam("rport") {
		fmt.Println("rport param is supported")
	}

	fmt.Printf("host=%s, port=%d\n", host, port)
}
