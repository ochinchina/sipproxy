package main

import (
	"fmt"
	"testing"
)

func TestParseToHeader(t *testing.T) {
	to, err := ParseTo("Bob <sip:bob@biloxi.example.com>;tag=8321234356")
	if err != nil {
		t.Fail()
	}
	fmt.Println(to)
	if tag, _ := to.GetTag(); tag != "8321234356" {
		t.Fail()
	}
}

func TestParseToHeaderWithAbsoluteURI(t *testing.T) {
	to, err := ParseTo("<urn:service:sos>;tag=8321234356")
	if err != nil {
		t.Fail()
	}
	fmt.Println(to)
	s, err := to.GetAbsoluteURI()
	if err != nil || s != "urn:service:sos" {
		t.Fail()
	}
}

func TestParseToHeaderWithTel(t *testing.T) {
	s := "<tel:190;phone-context=ims.mnc010.mcc724.3gppnetwork.org>;tag=LRF_eb0d2505"
	to, err := ParseTo(s)
	if err != nil {
		t.Errorf("Fail to parse header To: %s with error:%v", s, err)
	}

	if tag, err := to.GetTag(); err != nil || tag != "LRF_eb0d2505" {
		t.Errorf("Fail to get the tag from To: %s", s)
	}
	fmt.Println(to)
}
