package main

import (
	"bufio"
	"bytes"
	"fmt"
	"testing"
	"time"
)

func create_reader_from_string(s string) *bufio.Reader {
	return bufio.NewReader(bytes.NewBufferString(s))
}

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
	msg, err := ParseMessage(create_reader_from_string(s))
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
	msg, err := ParseMessage(create_reader_from_string(s))
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

	msg, err := ParseMessage(create_reader_from_string(s))
	if err != nil {
		t.Fail()
	}
	via, err := msg.GetVia()
	if err != nil || via == nil {
		t.Fail()
	}

	h1, err := msg.GetHeader("v")
	if err != nil {
		t.Fail()
	}

	h2, err := msg.GetHeader("via")
	if err != nil {
		t.Fail()
	}
	if h1 != h2 {
		t.Fail()
	}
	fmt.Println(msg)
}

func TestSubscribeResponse(t *testing.T) {
	s := `SIP/2.0 200 OK
CSeq: 2 SUBSCRIBE
Expires: 28
P-Charging-Vector: icid-value=sgc3.daatf005.sip.t-mobile.com-1485-460188-67377;term-ioi=e-ioi3
Via: SIP/2.0/UDP 10.68.103.193:5060;branch=z9hG4bK-383232-4ad1d19b0d288e2f7d340d9d845b7d3e;rport=16149;received=10.244.0.235^M
From: <sip:557399123456@10.68.103.193:5060>;tag=LRF_f3c4238f
To: <sip:557399123456@msg.pc.t-mobile.com>;tag=86b323bb
Call-ID: 3f375853bd0c3f138f8a18a2f26aae72@0.0.0.0
Content-Length: 0

`

	msg, err := ParseMessage(create_reader_from_string(s))
	if err != nil {
		t.Fail()
	}

	fmt.Println(msg)
}

func TestNotifyRequstParse(t *testing.T) {
	s := `NOTIFY sip:557399123456@10.68.103.193:5060 SIP/2.0
Route: <sip:ecf01.sip.t-mobile.com;lr;transport=udp>
Max-Forwards: 70
Allow: INVITE,BYE,CANCEL,ACK,SUBSCRIBE,NOTIFY,PUBLISH,MESSAGE,REFER,REGISTER,UPDATE
Call-ID: 4d5439bc97463287a2ea8b9968962c75@0.0.0.0
Contact: <sip:mavodi-0-1b2-26-1-ffffffff-@10.166.226.86:5060>
Event: dialog;call-id="MXJytcizoWFR5yuIQ15O_Q..@2607:fb90:8330:361a:0:f:cdeb:6701~ccso(0-419-3632-1)";to-tag=mavodi-0-1a3-e30-1-2000000-89e0c9-35-1e36-_00E081E57238-7627-7f259700-1e0a-588a52dc-132a9
Subscription-State: active;expires=86400
Content-Type: application/dialog-info+xml
P-Charging-Vector: icid-value=0.434.38-1485460191.420;orig-ioi=e-ioi3
Via: SIP/2.0/UDP 10.68.103.193:5060;branch=z9hG4bK-343639-4ab643fb4417a1fd5a1d459a5df76b76;received=10.68.101.167
From: <sip:557399123456@msg.pc.t-mobile.com>;tag=621f840b
To: <sip:557399123456@ecf01.sip.t-mobile.com:5060>;tag=LRF_623f6d01
CSeq: 3 NOTIFY
Content-Length: 4

test`

	msg, err := ParseMessage(create_reader_from_string(s))
	if err != nil {
		t.Fail()
	}
	fmt.Println(msg)
	requestURI, err := msg.GetRequestURI()
	if err != nil {
		t.Fail()
	}
	sipURI, err := requestURI.GetSIPURI()
	if err != nil {
		t.Fail()
	}
	if sipURI.Host != "10.68.103.193" || sipURI.GetPort() != 5060 {
		t.Fail()
	}
}

func TestParseMessagePerf(t *testing.T) {
	s := `NOTIFY sip:557399123456@10.68.103.193:5060 SIP/2.0
Route: <sip:ecf01.sip.t-mobile.com;lr;transport=udp>
Max-Forwards: 70
Allow: INVITE,BYE,CANCEL,ACK,SUBSCRIBE,NOTIFY,PUBLISH,MESSAGE,REFER,REGISTER,UPDATE
Call-ID: 4d5439bc97463287a2ea8b9968962c75@0.0.0.0
Contact: <sip:mavodi-0-1b2-26-1-ffffffff-@10.166.226.86:5060>
Event: dialog;call-id="MXJytcizoWFR5yuIQ15O_Q..@2607:fb90:8330:361a:0:f:cdeb:6701~ccso(0-419-3632-1)";to-tag=mavodi-0-1a3-e30-1-2000000-89e0c9-35-1e36-_00E081E57238-7627-7f259700-1e0a-588a52dc-132a9
Subscription-State: active;expires=86400
Content-Type: application/dialog-info+xml
P-Charging-Vector: icid-value=0.434.38-1485460191.420;orig-ioi=e-ioi3
Via: SIP/2.0/UDP 10.68.103.193:5060;branch=z9hG4bK-343639-4ab643fb4417a1fd5a1d459a5df76b76;received=10.68.101.167
From: <sip:557399123456@msg.pc.t-mobile.com>;tag=621f840b
To: <sip:557399123456@ecf01.sip.t-mobile.com:5060>;tag=LRF_623f6d01
CSeq: 3 NOTIFY
Content-Length: 4

test`

	start := time.Now()
	for i := 0; i < 1000000; i++ {
		msg, _ := ParseMessage(create_reader_from_string(s))
		msg.String()
	}
	end := time.Now()
	fmt.Printf("Total time:%d\n", end.Sub(start).Milliseconds())
}

func TestDialog(t *testing.T) {
	s1 := "SIP/2.0 300 Multiple choices\r\nCSeq: 1 INVITE\r\nCall-ID: ecscfyq63hk3hsrrs0aq0rnvray0k6h6uvanq@10.8.136.47\r\nFrom: <tel:+5521967014706>;tag=rnxyur0z\r\nTo: <tel:190;phone-context=ims.mnc010.mcc724.3gppnetwork.org>;tag=LRF_eb0d2505\r\nVia: SIP/2.0/UDP 10.8.136.164:5060;branch=z9hG4bKede0rcb2ooxho2oiieel0l2db;role=3;hpt=92e2_16;srti=s5_5\r\nRecord-Route: <sip:192.168.200.2:5060;lr>,<sip:10.8.136.164:5060;transport=udp;lr;Hpt=nw_5cd_63620067_407f5c2_ex_92e2_16;CxtId=4;TRC=ffffffff-ffffffff;srti=s5_5;X-HwB2bUaCookie=16594>\r\nContact: <sip:EEE21558000216503@vivo.com;user=phone?P-Asserted-Identity=sip:+EEE21558000216503%3Bcpc%3Demergency%3Buser%3Dphone>\r\nP-Charging-Vector: icid-value=\"rj-bar-pcscf01.19c.6c68.20221102083015\";orig-ioi=\"type 3spo-mb-scscf01.ims.mnc010.mcc724.3gppnetwork.org\u0002\";term-ioi=e-ioi3\r\nExpires: 7200\r\nContent-Length: 0\r\n\r\n"
	s2 := "ACK tel:190;phone-context=ims.mnc010.mcc724.3gppnetwork.org SIP/2.0\r\nVia: SIP/2.0/UDP 10.8.136.164:5060;branch=z9hG4bKede0rcb2ooxho2oiieel0l2db;Role=3;Hpt=92e2_16;srti=s5_5\r\nCall-ID: ecscfyq63hk3hsrrs0aq0rnvray0k6h6uvanq@10.8.136.47\r\nFrom: <tel:+5521967014706>;tag=rnxyur0z\r\nTo: <tel:190;phone-context=ims.mnc010.mcc724.3gppnetwork.org>;tag=LRF_eb0d2505\r\nCSeq: 1 ACK\r\nMax-Forwards: 70\r\nContent-Length: 0\r\n\r\n"
	fmt.Printf("%s", s1)
	fmt.Printf("%s", s2)
	msg1, err1 := ParseMessage(create_reader_from_string(s1))
	msg2, err2 := ParseMessage(create_reader_from_string(s2))
	if err1 != nil {
		t.Errorf("fail to create message from %s", s1)
	}

	if err2 != nil {
		t.Errorf("fail to create message from %s", s2)
	}

	dialog_1, _ := msg1.GetDialog()
	dialog_2, _ := msg2.GetDialog()

	if len(dialog_1) <= 0 || dialog_1 != dialog_2 {
		t.Errorf("dialog is different")
	}
	fmt.Printf("dialog is %s\n", dialog_1)
}

func TestDialogPerf(t *testing.T) {
	s1 := "SIP/2.0 300 Multiple choices\r\nCSeq: 1 INVITE\r\nCall-ID: ecscfyq63hk3hsrrs0aq0rnvray0k6h6uvanq@10.8.136.47\r\nFrom: <tel:+5521967014706>;tag=rnxyur0z\r\nTo: <tel:190;phone-context=ims.mnc010.mcc724.3gppnetwork.org>;tag=LRF_eb0d2505\r\nVia: SIP/2.0/UDP 10.8.136.164:5060;branch=z9hG4bKede0rcb2ooxho2oiieel0l2db;role=3;hpt=92e2_16;srti=s5_5\r\nRecord-Route: <sip:192.168.200.2:5060;lr>,<sip:10.8.136.164:5060;transport=udp;lr;Hpt=nw_5cd_63620067_407f5c2_ex_92e2_16;CxtId=4;TRC=ffffffff-ffffffff;srti=s5_5;X-HwB2bUaCookie=16594>\r\nContact: <sip:EEE21558000216503@vivo.com;user=phone?P-Asserted-Identity=sip:+EEE21558000216503%3Bcpc%3Demergency%3Buser%3Dphone>\r\nP-Charging-Vector: icid-value=\"rj-bar-pcscf01.19c.6c68.20221102083015\";orig-ioi=\"type 3spo-mb-scscf01.ims.mnc010.mcc724.3gppnetwork.org\u0002\";term-ioi=e-ioi3\r\nExpires: 7200\r\nContent-Length: 0\r\n\r\n"
	s2 := "ACK tel:190;phone-context=ims.mnc010.mcc724.3gppnetwork.org SIP/2.0\r\nVia: SIP/2.0/UDP 10.8.136.164:5060;branch=z9hG4bKede0rcb2ooxho2oiieel0l2db;Role=3;Hpt=92e2_16;srti=s5_5\r\nCall-ID: ecscfyq63hk3hsrrs0aq0rnvray0k6h6uvanq@10.8.136.47\r\nFrom: <tel:+5521967014706>;tag=rnxyur0z\r\nTo: <tel:190;phone-context=ims.mnc010.mcc724.3gppnetwork.org>;tag=LRF_eb0d2505\r\nCSeq: 1 ACK\r\nMax-Forwards: 70\r\nContent-Length: 0\r\n\r\n"
	fmt.Printf("%s", s1)
	fmt.Printf("%s", s2)
	for i := 0; i < 1000000; i++ {
		msg1, err1 := ParseMessage(create_reader_from_string(s1))
		msg2, err2 := ParseMessage(create_reader_from_string(s2))
		if err1 != nil {
			t.Errorf("fail to create message from %s", s1)
		}

		if err2 != nil {
			t.Errorf("fail to create message from %s", s2)
		}

		dialog_1, _ := msg1.GetDialog()
		dialog_2, _ := msg2.GetDialog()

		if len(dialog_1) <= 0 || dialog_1 != dialog_2 {
			t.Errorf("dialog is different")
		}
	}
}
