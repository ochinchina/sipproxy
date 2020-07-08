package main

import (
	"fmt"
	"testing"
)

func TestParseInviteMessage(t *testing.T) {
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

func TestParseResponseMessage(t *testing.T) {
	s := `SIP/2.0 300 Multiple choices
CSeq: 1 INVITE
Call-ID: MXJytcizoWFR5yuIQ15O_Q..@2607:fb90:8330:361a:0:f:cdeb:6701~ccso(0-419-3632-1)
From: <sip:557399123456@msg.pc.t-mobile.com>;tag=mavodi-0-1a3-e30-1-2000000-89e0c9-35-1e36-_00E081E57238-7627-7f259700-1e0a-588a52dc-132a9
To: <urn:service:sos>;tag=LRF_28a5ff91
Via: SIP/2.0/UDP 10.68.103.193:5060;branch=z9hG4bK02ebe95c32c4,SIP/2.0/UDP 10.68.101.167:5061;branch=z9hG4bKmavodi-0-177-1c7-1-e1510000-6a8dc0611909d96d;received=10.68.103.193,SIP/2.0/UDP 10.166.226.87:5066;nwkintf=6;realm=realm-mw;recvdsrvport=5060;recvdsrvip=10.166.226.86;mav-udp-rport=5066;received=10.166.226.87;branch=z9hG4bKmavodi-0-1a3-e30-1-2000000-89e0c9-35-1e36-2799083920-30247
Record-Route: <sip:10.68.103.193:5060>,<sip:mavodi-0-177-1c7-1-1150000-@10.166.226.86:5060;lr>,<sip:mavodi-0-1a2-3fffffff-1-ffffffff-@10.166.226.87:5066;mavsipodi-0-1a9-35-1-1e36;lr>
Contact: <sip:11719121111111@vivo.com;user=phone?P-Asserted-Identity=sip:+13125111078%3Bcpc%3Demergency%3Buser%3Dphone>
P-Charging-Vector: icid-value=sgc3.daatf005.sip.t-mobile.com-1485-460188-67377;term-ioi=e-ioi3
nExpires: 7200
Content-Length: 0

CK
Accept-Contact: *;+g.3gpp.icsi-ref=\"urn%3Aurn-7%3A3gpp-service.ims.icsi.mmtel\"\nRecord-Route: <sip:mavodi-0-177-1c7-1-1150000-@10.166.226.86:5060;lr>,<sip:mavodi-0-1a2-3fffffff-1-ffffffff-@10.166.226.87:5066;mavsipodi-0-1a9-35-1-1e36;lr>\nP-Charging-Vector: icid-value=sgc3.daatf005.sip.t-mobile.com-1485-460188-67377;icid-generated-at=sgc3.daatf005.sip.t-mobile.com\nP-Asserted-Identity: <sip:557399123456@msg.pc.t-mobile.com>\nP-Access-Network-Info: 3GPP-E-UTRAN-FDD; utran-cell-id-3gpp=3114802c340001815\nUser-Agent: T-Mobile VoLTE-RCS-ussd SEC/N920TUVU4D 6.0.1 Mavenir UAG/v4.5 EATF/v4.5-14042501o\nPriority: emergency\nContent-Type: application/sdp\nContent-Length: 4\n\ntest\n`
	msg, err := ParseMessage([]byte(s))
	if err != nil {
		t.Fail()
	}
	fmt.Println(msg)
}

func TestParseSubscribeMessage(t *testing.T) {
	s := `SUBSCRIBE sip:10.68.103.193:5060 SIP/2.0
Via: SIP/2.0/UDP 10.68.103.193:5060;branch=z9hG4bK-343437-6119ae735a31a77df85d22346c030fa7;rport
Route: <sip:mavodi-0-177-1c7-1-1150000-@10.166.226.86:5060;lr>,<sip:mavodi-0-1a2-3fffffff-1-ffffffff-@10.166.226.87:5066;mavsipodi-0-1a9-35-1-1e36;lr>,<sip:557399123456@msg.pc.t-mobile.com>
From: <sip:557399123456@10.68.103.193:5060>;tag=LRF_c8389031
To: <sip:557399123456@msg.pc.t-mobile.com>
Call-ID: 020012769fd67de8196463c0a6d01bf6@0.0.0.0
CSeq: 2 SUBSCRIBE
Contact: <sip:557399123456@10.68.103.193:5060;transport=udp>
Max-Forwards: 70
Expires: 86400
Event: dialog;call-id=MXJytcizoWFR5yuIQ15O_Q..@2607:fb90:8330:361a:0:f:cdeb:6701~ccso(0-419-3632-1);to-tag=mavodi-0-1a3-e30-1-2000000-89e0c9-35-1e36-_00E081E57238-7627-7f259700-1e0a-588a52dc-132a9
nP-Charging-Vector: icid-value=sgc3.daatf005.sip.t-mobile.com-1485-460188-67377
Content-Length: 0

`

	msg, err := ParseMessage([]byte(s))
	if err != nil {
		t.Fail()
	}
	via, err := msg.GetVia()
	if err != nil || via == nil {
		t.Fail()
	}

	fmt.Println(msg)
}
