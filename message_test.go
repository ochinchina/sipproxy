package main

import (
	"fmt"
	"testing"
)

func TestParseMessage(t *testing.T) {
	s := `INVITE sip:bob@biloxi.example.com SIP/2.0
Via: SIP/2.0/UDP client.atlanta.example.com:5060;branch=z9hG4bK74bf9
Max-Forwards: 70
From: Alice <sip:alice@atlanta.example.com>;tag=9fxced76sl
To: Bob <sip:bob@biloxi.example.com>
Call-ID: 3848276298220188511@atlanta.example.com
CSeq: 1 INVITE
Contact: <sip:alice@client.atlanta.example.com;transport=tcp>
Content-Type: application/sdp
Content-Length: 22

this is a test message`
	//fmt.Println( s )
	msg, err := ParseMessage([]byte(s))
	if err != nil {
		t.Fail()
	}
	fmt.Println("============")
	fmt.Println(msg)
	fmt.Println("============")
}
