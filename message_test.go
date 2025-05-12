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
		_ = msg.String()
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

func TestLongContact(t *testing.T) {
	s := `SIP/2.0 300 Multiple choices
CSeq: 1 INVITE
Call-ID: 6b7e369351b6e74f1e1611f622175bdb
From: <sip:+351266010860@ims.mnc006.mcc268.3gppnetwork.org>;tag=536fabd6ec3cf17669ce66bfbc570527
Contact: <sip:+351112266@psap.lab.ims.telecom.pt?Accept-Contact=*%3Borganization%3D%22020%22&Geolocation=%3Csip%3A%2B351266010860%40ims.mnc006.mcc268.3gppnetwork.org%3E&Geolocation=%3Ccid%3A2025-05-08T13%3A09%3A16.199%2B01%3A00%40ims.mnc006.mcc268.3gppnetwork.org%3E&Content-Type=multipart%2Fmixed%3Bboundary%3Dgeolocation-boundary&body=%0D%0AContent-Type%3Aapplication%2Fpidf%2Bxml%0D%0AContent-Disposition%3Arender%3Bhandling%3Doptional%0D%0AContent-ID%3A%3Csip%3A%2B351266010860%40ims.mnc006.mcc268.3gppnetwork.org%3E%0D%0A%0D%0A%3C%3Fxml%20version%3D%221.0%22%3F%3E%0D%0A%3Cpresence%20xmlns%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%22%20xmlns%3Adm%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%3Adata-model%22%20xmlns%3Agp%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%3Ageopriv10%22%20xmlns%3Agml%3D%22http%3A%2F%2Fwww.opengis.net%2Fgml%22%20xmlns%3Ags%3D%22http%3A%2F%2Fwww.opengis.net%2Fpidflo%2F1.0%22%20xmlns%3Acon%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Ageopriv%3Aconf%22%20entity%3D%22sip%3A%2B351266010860%40ims.mnc006.mcc268.3gppnetwork.org%22%3E%0D%0A%3Cdm%3Adevice%20id%3D%22Wifi%22%3E%0D%0A%3Cgp%3Ageopriv%3E%0D%0A%3Cgp%3Alocation-info%3E%0D%0A%3Cgs%3ACircle%20srsName%3D%22urn%3Aogc%3Adef%3Acrs%3AEPSG%3A%3A4326%22%3E%0D%0A%3Cgml%3Apos%3E38.731085%20-9.144653%3C%2Fgml%3Apos%3E%0D%0A%3Cgs%3Aradius%20uom%3D%22urn%3Aogc%3Adef%3Auom%3AEPSG%3A%3A9001%22%3E60.578000%3C%2Fgs%3Aradius%3E%0D%0A%3C%2Fgs%3ACircle%3E%0D%0A%3Ccon%3Aconfidence%20pdf%3D%22normal%22%3E95%3C%2Fcon%3Aconfidence%3E%0D%0A%3C%2Fgp%3Alocation-info%3E%0D%0A%3Cgp%3Amethod%3EDBH_HELO%3C%2Fgp%3Amethod%3E%0D%0A%3Cgp%3Ausage-rules%2F%3E%0D%0A%3C%2Fgp%3Ageopriv%3E%0D%0A%3Cdm%3Atimestamp%3E2025-05-08T12%3A09%3A14Z%3C%2Fdm%3Atimestamp%3E%0D%0A%3C%2Fdm%3Adevice%3E%0D%0A%3C%2Fpresence%3E%0D%0A%0D%0A%0D%0AContent-Type%3Aapplication%2Fsdp%0D%0AContent-Length%3A1137%0D%0A%0D%0Av%3D0%0D%0Ao%3Dsip%3A%2B351266010860%40ims.mnc006.mcc268.3gppnetwork.org%201746706154%201746706154%20IN%20IP4%2010.111.162.68%0D%0As%3D-%0D%0Ac%3DIN%20IP4%2010.111.168.70%0D%0At%3D0%200%0D%0Aa%3Dsendrecv%0D%0Am%3Daudio%205184%20RTP%2FAVP%20109%20104%20110%209%20102%20108%208%200%2018%20105%20100%0D%0Ab%3DAS%3A128%0D%0Ab%3DRS%3A0%0D%0Ab%3DRR%3A0%0D%0Aa%3Drtpmap%3A109%20EVS%2F16000%0D%0Aa%3Dfmtp%3A109%20br%3D5.9-24.4%3B%20bw%3Dnb-wb%3B%20max-red%3D0%0D%0Aa%3Drtpmap%3A104%20AMR-WB%2F16000%0D%0Aa%3Dfmtp%3A104%20max-red%3D0%3B%20mode-change-capability%3D2%0D%0Aa%3Drtpmap%3A110%20AMR-WB%2F16000%0D%0Aa%3Dfmtp%3A110%20max-red%3D0%3B%20mode-change-capability%3D2%3B%20octet-align%3D1%0D%0Aa%3Drtpmap%3A9%20G722%2F8000%0D%0Aa%3Drtpmap%3A102%20AMR%2F8000%0D%0Aa%3Dfmtp%3A102%20max-red%3D0%3B%20mode-change-capability%3D2%0D%0Aa%3Drtpmap%3A108%20AMR%2F8000%0D%0Aa%3Dfmtp%3A108%20max-red%3D0%3B%20mode-change-capability%3D2%3B%20octet-align%3D1%0D%0Aa%3Drtpmap%3A8%20PCMA%2F8000%0D%0Aa%3Drtpmap%3A0%20PCMU%2F8000%0D%0Aa%3Drtpmap%3A18%20G729%2F8000%0D%0Aa%3Dfmtp%3A18%20annexb%3Dyes%0D%0Aa%3Drtpmap%3A105%20telephone-event%2F16000%0D%0Aa%3Dfmtp%3A105%200-15%0D%0Aa%3Drtpmap%3A100%20telephone-event%2F8000%0D%0Aa%3Dfmtp%3A100%200-15%0D%0Aa%3Dptime%3A20%0D%0Aa%3Dmaxptime%3A40%0D%0Aa%3Dsendrecv%0D%0Aa%3Ddes%3Aqos%20mandatory%20local%20sendrecv%0D%0Aa%3Dcurr%3Aqos%20local%20none%0D%0Aa%3Ddes%3Aqos%20optional%20remote%20sendrecv%0D%0Aa%3Dcurr%3Aqos%20remote%20none%0D%0Am%3Dtext%205248%20RTP%2FAVP%20112%20111%0D%0Ab%3DAS%3A4%0D%0Ab%3DRS%3A0%0D%0Ab%3DRR%3A0%0D%0Aa%3Drtpmap%3A112%20red%2F1000%0D%0Aa%3Dfmtp%3A112%20111%2F111%2F111%0D%0Aa%3Drtpmap%3A111%20t140%2F1000%0D%0Aa%3Dsendrecv%0D%0A%0D%0A--geolocation-boundary%0D%0D%0AContent-Type%3A%20application%2Fpidf%2Bxml%0D%0D%0AContent-Disposition%3A%20render%3Bhandling%3Doptional%0D%0D%0AContent-ID%3A%20%3C2025-05-08T13:09:16.199+01:00%40ims.mnc006.mcc268.3gpp.network.org%3E%0D%0D%0A%0D%0D%0A%3C%3Fxml%20version%3D%221.0%22%20encoding%3D%22UTF-8%22%3F%3E%0D%0D%0A%3Cpresence%20xmlns%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%22%20xmlns%3Agp%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%3Ageopriv10%22%20xmlns%3Agml%3D%22http%3A%2F%2Fwww.opengis.net%2Fgml%22%20xmlns%3Ags%3D%22http%3A%2F%2Fwww.opengis.net%2Fpidflo%2F1.0%22%20xmlns%3Acl%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%3Ageopriv10%3AcivicLoc%22%20entity%3D%222025-05-08T13:09:16.199+01:00%2540ims.mnc006.mcc268.3gpp.network.org%22%3E%0D%0D%0A%09%3Ctuple%3E%0D%0D%0A%09%09%3Cstatus%3E%0D%0D%0A%09%09%09%3Cgp%3Ageopriv%3E%0D%0D%0A%09%09%09%09%3Cgp%3Alocation-info%3E%0D%0D%0A%09%09%09%09%09%3Ccl%3AcivicAddress%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AA3%3EMAIA%3C%2Fcl%3AA3%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AA6%3E SOBREIRAS%3C%2Fcl%3AA6%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3ASTS%3EMONTE%3C%2Fcl%3ASTS%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AHNO%3ESN%3C%2Fcl%3AHNO%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3APC%3EC%7050-021%3C%2Fcl%3APC%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3APCN%3EMONTEMOR-O-NOVO%3C%2Fcl%3APCN%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AHNS%3E%3C%2Fcl%3AHNS%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AUNIT%3E%3C%2Fcl%3AUNIT%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3ABLD%3E%3C%2Fcl%3ABLD%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3ANAM%3E%3C%2Fcl%3ANAM%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3APLC%3E%3C%2Fcl%3APLC%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AFLR%3E%3C%2Fcl%3AFLR%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3ALOC%3E%3C%2Fcl%3ALOC%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AROOM%3E%3C%2Fcl%3AROOM%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3ASEAT%3E%3C%2Fcl%3ASEAT%3E%0D%0D%0A%09%09%09%09%09%3C%2Fcl%3AcivicAddress%3E%0D%0D%0A%09%09%09%09%3C%2Fgp%3Alocation-info%3E%0D%0D%0A%09%09%09%3C%2Fgp%3Ageopriv%3E%0D%0D%0A%09%09%3C%2Fstatus%3E%0D%0D%0A%09%09%3Ctimestamp%3E2024-09-12T12%3A20%3A00%3C%2Ftimestamp%3E%0D%0D%0A%09%3C%2Ftuple%3E%0D%0D%0A%3C%2Fpresence%3E%0D%0D%0A%0D%0D%0A--geolocation-boundary-->,<sip:+351112266@psap2.lab.ims.telecom.pt?Accept-Contact=*%3Borganization%3D%22020%22&Geolocation=%3Csip%3A%2B351266010860%40ims.mnc006.mcc268.3gppnetwork.org%3E&Geolocation=%3Ccid%3A2025-05-08T13%3A09%3A16.199%2B01%3A00%40ims.mnc006.mcc268.3gppnetwork.org%3E&Content-Type=multipart%2Fmixed%3Bboundary%3Dgeolocation-boundary&body=%0D%0AContent-Type%3Aapplication%2Fpidf%2Bxml%0D%0AContent-Disposition%3Arender%3Bhandling%3Doptional%0D%0AContent-ID%3A%3Csip%3A%2B351266010860%40ims.mnc006.mcc268.3gppnetwork.org%3E%0D%0A%0D%0A%3C%3Fxml%20version%3D%221.0%22%3F%3E%0D%0A%3Cpresence%20xmlns%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%22%20xmlns%3Adm%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%3Adata-model%22%20xmlns%3Agp%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%3Ageopriv10%22%20xmlns%3Agml%3D%22http%3A%2F%2Fwww.opengis.net%2Fgml%22%20xmlns%3Ags%3D%22http%3A%2F%2Fwww.opengis.net%2Fpidflo%2F1.0%22%20xmlns%3Acon%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Ageopriv%3Aconf%22%20entity%3D%22sip%3A%2B351266010860%40ims.mnc006.mcc268.3gppnetwork.org%22%3E%0D%0A%3Cdm%3Adevice%20id%3D%22Wifi%22%3E%0D%0A%3Cgp%3Ageopriv%3E%0D%0A%3Cgp%3Alocation-info%3E%0D%0A%3Cgs%3ACircle%20srsName%3D%22urn%3Aogc%3Adef%3Acrs%3AEPSG%3A%3A4326%22%3E%0D%0A%3Cgml%3Apos%3E38.731085%20-9.144653%3C%2Fgml%3Apos%3E%0D%0A%3Cgs%3Aradius%20uom%3D%22urn%3Aogc%3Adef%3Auom%3AEPSG%3A%3A9001%22%3E60.578000%3C%2Fgs%3Aradius%3E%0D%0A%3C%2Fgs%3ACircle%3E%0D%0A%3Ccon%3Aconfidence%20pdf%3D%22normal%22%3E95%3C%2Fcon%3Aconfidence%3E%0D%0A%3C%2Fgp%3Alocation-info%3E%0D%0A%3Cgp%3Amethod%3EDBH_HELO%3C%2Fgp%3Amethod%3E%0D%0A%3Cgp%3Ausage-rules%2F%3E%0D%0A%3C%2Fgp%3Ageopriv%3E%0D%0A%3Cdm%3Atimestamp%3E2025-05-08T12%3A09%3A14Z%3C%2Fdm%3Atimestamp%3E%0D%0A%3C%2Fdm%3Adevice%3E%0D%0A%3C%2Fpresence%3E%0D%0A%0D%0A%0D%0AContent-Type%3Aapplication%2Fsdp%0D%0AContent-Length%3A1137%0D%0A%0D%0Av%3D0%0D%0Ao%3Dsip%3A%2B351266010860%40ims.mnc006.mcc268.3gppnetwork.org%201746706154%201746706154%20IN%20IP4%2010.111.162.68%0D%0As%3D-%0D%0Ac%3DIN%20IP4%2010.111.168.70%0D%0At%3D0%200%0D%0Aa%3Dsendrecv%0D%0Am%3Daudio%205184%20RTP%2FAVP%20109%20104%20110%209%20102%20108%208%200%2018%20105%20100%0D%0Ab%3DAS%3A128%0D%0Ab%3DRS%3A0%0D%0Ab%3DRR%3A0%0D%0Aa%3Drtpmap%3A109%20EVS%2F16000%0D%0Aa%3Dfmtp%3A109%20br%3D5.9-24.4%3B%20bw%3Dnb-wb%3B%20max-red%3D0%0D%0Aa%3Drtpmap%3A104%20AMR-WB%2F16000%0D%0Aa%3Dfmtp%3A104%20max-red%3D0%3B%20mode-change-capability%3D2%0D%0Aa%3Drtpmap%3A110%20AMR-WB%2F16000%0D%0Aa%3Dfmtp%3A110%20max-red%3D0%3B%20mode-change-capability%3D2%3B%20octet-align%3D1%0D%0Aa%3Drtpmap%3A9%20G722%2F8000%0D%0Aa%3Drtpmap%3A102%20AMR%2F8000%0D%0Aa%3Dfmtp%3A102%20max-red%3D0%3B%20mode-change-capability%3D2%0D%0Aa%3Drtpmap%3A108%20AMR%2F8000%0D%0Aa%3Dfmtp%3A108%20max-red%3D0%3B%20mode-change-capability%3D2%3B%20octet-align%3D1%0D%0Aa%3Drtpmap%3A8%20PCMA%2F8000%0D%0Aa%3Drtpmap%3A0%20PCMU%2F8000%0D%0Aa%3Drtpmap%3A18%20G729%2F8000%0D%0Aa%3Dfmtp%3A18%20annexb%3Dyes%0D%0Aa%3Drtpmap%3A105%20telephone-event%2F16000%0D%0Aa%3Dfmtp%3A105%200-15%0D%0Aa%3Drtpmap%3A100%20telephone-event%2F8000%0D%0Aa%3Dfmtp%3A100%200-15%0D%0Aa%3Dptime%3A20%0D%0Aa%3Dmaxptime%3A40%0D%0Aa%3Dsendrecv%0D%0Aa%3Ddes%3Aqos%20mandatory%20local%20sendrecv%0D%0Aa%3Dcurr%3Aqos%20local%20none%0D%0Aa%3Ddes%3Aqos%20optional%20remote%20sendrecv%0D%0Aa%3Dcurr%3Aqos%20remote%20none%0D%0Am%3Dtext%205248%20RTP%2FAVP%20112%20111%0D%0Ab%3DAS%3A4%0D%0Ab%3DRS%3A0%0D%0Ab%3DRR%3A0%0D%0Aa%3Drtpmap%3A112%20red%2F1000%0D%0Aa%3Dfmtp%3A112%20111%2F111%2F111%0D%0Aa%3Drtpmap%3A111%20t140%2F1000%0D%0Aa%3Dsendrecv%0D%0A%0D%0A--geolocation-boundary%0D%0D%0AContent-Type%3A%20application%2Fpidf%2Bxml%0D%0D%0AContent-Disposition%3A%20render%3Bhandling%3Doptional%0D%0D%0AContent-ID%3A%20%3C2025-05-08T13:09:16.199+01:00%40ims.mnc006.mcc268.3gpp.network.org%3E%0D%0D%0A%0D%0D%0A%3C%3Fxml%20version%3D%221.0%22%20encoding%3D%22UTF-8%22%3F%3E%0D%0D%0A%3Cpresence%20xmlns%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%22%20xmlns%3Agp%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%3Ageopriv10%22%20xmlns%3Agml%3D%22http%3A%2F%2Fwww.opengis.net%2Fgml%22%20xmlns%3Ags%3D%22http%3A%2F%2Fwww.opengis.net%2Fpidflo%2F1.0%22%20xmlns%3Acl%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%3Ageopriv10%3AcivicLoc%22%20entity%3D%222025-05-08T13:09:16.199+01:00%2540ims.mnc006.mcc268.3gpp.network.org%22%3E%0D%0D%0A%09%3Ctuple%3E%0D%0D%0A%09%09%3Cstatus%3E%0D%0D%0A%09%09%09%3Cgp%3Ageopriv%3E%0D%0D%0A%09%09%09%09%3Cgp%3Alocation-info%3E%0D%0D%0A%09%09%09%09%09%3Ccl%3AcivicAddress%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AA3%3EMAIA%3C%2Fcl%3AA3%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AA6%3E SOBREIRAS%3C%2Fcl%3AA6%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3ASTS%3EMONTE%3C%2Fcl%3ASTS%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AHNO%3ESN%3C%2Fcl%3AHNO%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3APC%3EC%7050-021%3C%2Fcl%3APC%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3APCN%3EMONTEMOR-O-NOVO%3C%2Fcl%3APCN%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AHNS%3E%3C%2Fcl%3AHNS%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AUNIT%3E%3C%2Fcl%3AUNIT%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3ABLD%3E%3C%2Fcl%3ABLD%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3ANAM%3E%3C%2Fcl%3ANAM%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3APLC%3E%3C%2Fcl%3APLC%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AFLR%3E%3C%2Fcl%3AFLR%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3ALOC%3E%3C%2Fcl%3ALOC%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AROOM%3E%3C%2Fcl%3AROOM%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3ASEAT%3E%3C%2Fcl%3ASEAT%3E%0D%0D%0A%09%09%09%09%09%3C%2Fcl%3AcivicAddress%3E%0D%0D%0A%09%09%09%09%3C%2Fgp%3Alocation-info%3E%0D%0D%0A%09%09%09%3C%2Fgp%3Ageopriv%3E%0D%0D%0A%09%09%3C%2Fstatus%3E%0D%0D%0A%09%09%3Ctimestamp%3E2024-09-12T12%3A20%3A00%3C%2Ftimestamp%3E%0D%0D%0A%09%3C%2Ftuple%3E%0D%0D%0A%3C%2Fpresence%3E%0D%0D%0A%0D%0D%0A--geolocation-boundary-->,<sip:+351112266@psap3.lab.ims.telecom.pt?Accept-Contact=*%3Borganization%3D%22020%22&Geolocation=%3Csip%3A%2B351266010860%40ims.mnc006.mcc268.3gppnetwork.org%3E&Geolocation=%3Ccid%3A2025-05-08T13%3A09%3A16.199%2B01%3A00%40ims.mnc006.mcc268.3gppnetwork.org%3E&Content-Type=multipart%2Fmixed%3Bboundary%3Dgeolocation-boundary&body=%0D%0AContent-Type%3Aapplication%2Fpidf%2Bxml%0D%0AContent-Disposition%3Arender%3Bhandling%3Doptional%0D%0AContent-ID%3A%3Csip%3A%2B351266010860%40ims.mnc006.mcc268.3gppnetwork.org%3E%0D%0A%0D%0A%3C%3Fxml%20version%3D%221.0%22%3F%3E%0D%0A%3Cpresence%20xmlns%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%22%20xmlns%3Adm%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%3Adata-model%22%20xmlns%3Agp%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%3Ageopriv10%22%20xmlns%3Agml%3D%22http%3A%2F%2Fwww.opengis.net%2Fgml%22%20xmlns%3Ags%3D%22http%3A%2F%2Fwww.opengis.net%2Fpidflo%2F1.0%22%20xmlns%3Acon%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Ageopriv%3Aconf%22%20entity%3D%22sip%3A%2B351266010860%40ims.mnc006.mcc268.3gppnetwork.org%22%3E%0D%0A%3Cdm%3Adevice%20id%3D%22Wifi%22%3E%0D%0A%3Cgp%3Ageopriv%3E%0D%0A%3Cgp%3Alocation-info%3E%0D%0A%3Cgs%3ACircle%20srsName%3D%22urn%3Aogc%3Adef%3Acrs%3AEPSG%3A%3A4326%22%3E%0D%0A%3Cgml%3Apos%3E38.731085%20-9.144653%3C%2Fgml%3Apos%3E%0D%0A%3Cgs%3Aradius%20uom%3D%22urn%3Aogc%3Adef%3Auom%3AEPSG%3A%3A9001%22%3E60.578000%3C%2Fgs%3Aradius%3E%0D%0A%3C%2Fgs%3ACircle%3E%0D%0A%3Ccon%3Aconfidence%20pdf%3D%22normal%22%3E95%3C%2Fcon%3Aconfidence%3E%0D%0A%3C%2Fgp%3Alocation-info%3E%0D%0A%3Cgp%3Amethod%3EDBH_HELO%3C%2Fgp%3Amethod%3E%0D%0A%3Cgp%3Ausage-rules%2F%3E%0D%0A%3C%2Fgp%3Ageopriv%3E%0D%0A%3Cdm%3Atimestamp%3E2025-05-08T12%3A09%3A14Z%3C%2Fdm%3Atimestamp%3E%0D%0A%3C%2Fdm%3Adevice%3E%0D%0A%3C%2Fpresence%3E%0D%0A%0D%0A%0D%0AContent-Type%3Aapplication%2Fsdp%0D%0AContent-Length%3A1137%0D%0A%0D%0Av%3D0%0D%0Ao%3Dsip%3A%2B351266010860%40ims.mnc006.mcc268.3gppnetwork.org%201746706154%201746706154%20IN%20IP4%2010.111.162.68%0D%0As%3D-%0D%0Ac%3DIN%20IP4%2010.111.168.70%0D%0At%3D0%200%0D%0Aa%3Dsendrecv%0D%0Am%3Daudio%205184%20RTP%2FAVP%20109%20104%20110%209%20102%20108%208%200%2018%20105%20100%0D%0Ab%3DAS%3A128%0D%0Ab%3DRS%3A0%0D%0Ab%3DRR%3A0%0D%0Aa%3Drtpmap%3A109%20EVS%2F16000%0D%0Aa%3Dfmtp%3A109%20br%3D5.9-24.4%3B%20bw%3Dnb-wb%3B%20max-red%3D0%0D%0Aa%3Drtpmap%3A104%20AMR-WB%2F16000%0D%0Aa%3Dfmtp%3A104%20max-red%3D0%3B%20mode-change-capability%3D2%0D%0Aa%3Drtpmap%3A110%20AMR-WB%2F16000%0D%0Aa%3Dfmtp%3A110%20max-red%3D0%3B%20mode-change-capability%3D2%3B%20octet-align%3D1%0D%0Aa%3Drtpmap%3A9%20G722%2F8000%0D%0Aa%3Drtpmap%3A102%20AMR%2F8000%0D%0Aa%3Dfmtp%3A102%20max-red%3D0%3B%20mode-change-capability%3D2%0D%0Aa%3Drtpmap%3A108%20AMR%2F8000%0D%0Aa%3Dfmtp%3A108%20max-red%3D0%3B%20mode-change-capability%3D2%3B%20octet-align%3D1%0D%0Aa%3Drtpmap%3A8%20PCMA%2F8000%0D%0Aa%3Drtpmap%3A0%20PCMU%2F8000%0D%0Aa%3Drtpmap%3A18%20G729%2F8000%0D%0Aa%3Dfmtp%3A18%20annexb%3Dyes%0D%0Aa%3Drtpmap%3A105%20telephone-event%2F16000%0D%0Aa%3Dfmtp%3A105%200-15%0D%0Aa%3Drtpmap%3A100%20telephone-event%2F8000%0D%0Aa%3Dfmtp%3A100%200-15%0D%0Aa%3Dptime%3A20%0D%0Aa%3Dmaxptime%3A40%0D%0Aa%3Dsendrecv%0D%0Aa%3Ddes%3Aqos%20mandatory%20local%20sendrecv%0D%0Aa%3Dcurr%3Aqos%20local%20none%0D%0Aa%3Ddes%3Aqos%20optional%20remote%20sendrecv%0D%0Aa%3Dcurr%3Aqos%20remote%20none%0D%0Am%3Dtext%205248%20RTP%2FAVP%20112%20111%0D%0Ab%3DAS%3A4%0D%0Ab%3DRS%3A0%0D%0Ab%3DRR%3A0%0D%0Aa%3Drtpmap%3A112%20red%2F1000%0D%0Aa%3Dfmtp%3A112%20111%2F111%2F111%0D%0Aa%3Drtpmap%3A111%20t140%2F1000%0D%0Aa%3Dsendrecv%0D%0A%0D%0A--geolocation-boundary%0D%0D%0AContent-Type%3A%20application%2Fpidf%2Bxml%0D%0D%0AContent-Disposition%3A%20render%3Bhandling%3Doptional%0D%0D%0AContent-ID%3A%20%3C2025-05-08T13:09:16.199+01:00%40ims.mnc006.mcc268.3gpp.network.org%3E%0D%0D%0A%0D%0D%0A%3C%3Fxml%20version%3D%221.0%22%20encoding%3D%22UTF-8%22%3F%3E%0D%0D%0A%3Cpresence%20xmlns%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%22%20xmlns%3Agp%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%3Ageopriv10%22%20xmlns%3Agml%3D%22http%3A%2F%2Fwww.opengis.net%2Fgml%22%20xmlns%3Ags%3D%22http%3A%2F%2Fwww.opengis.net%2Fpidflo%2F1.0%22%20xmlns%3Acl%3D%22urn%3Aietf%3Aparams%3Axml%3Ans%3Apidf%3Ageopriv10%3AcivicLoc%22%20entity%3D%222025-05-08T13:09:16.199+01:00%2540ims.mnc006.mcc268.3gpp.network.org%22%3E%0D%0D%0A%09%3Ctuple%3E%0D%0D%0A%09%09%3Cstatus%3E%0D%0D%0A%09%09%09%3Cgp%3Ageopriv%3E%0D%0D%0A%09%09%09%09%3Cgp%3Alocation-info%3E%0D%0D%0A%09%09%09%09%09%3Ccl%3AcivicAddress%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AA3%3EMAIA%3C%2Fcl%3AA3%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AA6%3E SOBREIRAS%3C%2Fcl%3AA6%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3ASTS%3EMONTE%3C%2Fcl%3ASTS%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AHNO%3ESN%3C%2Fcl%3AHNO%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3APC%3EC%7050-021%3C%2Fcl%3APC%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3APCN%3EMONTEMOR-O-NOVO%3C%2Fcl%3APCN%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AHNS%3E%3C%2Fcl%3AHNS%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AUNIT%3E%3C%2Fcl%3AUNIT%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3ABLD%3E%3C%2Fcl%3ABLD%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3ANAM%3E%3C%2Fcl%3ANAM%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3APLC%3E%3C%2Fcl%3APLC%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AFLR%3E%3C%2Fcl%3AFLR%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3ALOC%3E%3C%2Fcl%3ALOC%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3AROOM%3E%3C%2Fcl%3AROOM%3E%0D%0D%0A%09%09%09%09%09%09%3Ccl%3ASEAT%3E%3C%2Fcl%3ASEAT%3E%0D%0D%0A%09%09%09%09%09%3C%2Fcl%3AcivicAddress%3E%0D%0D%0A%09%09%09%09%3C%2Fgp%3Alocation-info%3E%0D%0D%0A%09%09%09%3C%2Fgp%3Ageopriv%3E%0D%0D%0A%09%09%3C%2Fstatus%3E%0D%0D%0A%09%09%3Ctimestamp%3E2024-09-12T12%3A20%3A00%3C%2Ftimestamp%3E%0D%0D%0A%09%3C%2Ftuple%3E%0D%0D%0A%3C%2Fpresence%3E%0D%0D%0A%0D%0D%0A--geolocation-boundary-->
To: <urn:service:sos>;tag=LRF_b793c194
Via: SIP/2.0/TCP 10.111.129.78:5060;branch=z9hG4bK42007f0bb446;rport=63085,SIP/2.0/TCP 10.111.173.228:5060;branch=z9hG4bK516500911f722ea7a5b714700a73cae3k555555yaaaaacaaaaaaaaaaaaa3Zqkv7af2k5l3ibaaiaiaaaaacqaaaaaabaaaaaaa,SIP/2.0/TCP 10.111.173.230:5060;branch=z9hG4bK323efb5de6ef8a3a2fbc954eab96a923k555555yaaaaaeaaaaaaaaaaaaa3Zqkv7adiktf2qbaaiaiaaaaacqaaaaaaaaaaaaaa
Record-Route: <sip:10.111.129.78:5060;lr>,<sip:3Zqkv71cWaezDMQ8QaaaaaI8v9Fubabaeaaaaae8dQuSW6Ymeeaaaaaatel%3A%2B351266010860@vecscftst1.lab.ims.telecom.pt:5060;maddr=10.111.173.228;lr>,<sip:3Zqkv71cWacGaaaacaaaaaF4RGNd%24k4jWaaaaaaaaaaaaaaaafaaaaaa4264264304@veatftst1.lab.ims.telecom.pt:5060;maddr=10.111.173.230;lr>
P-Charging-Vector: icid-value=vsbgtst1-pmp-1.lab.ims.teleco-1746-706155-105143-357;orig-ioi=lab.ims.telecom.pt;term-ioi=e-ioi3
Expires: 7200
Content-Length: 0


`
	msg, err := ParseMessage(create_reader_from_string(s))
	if err != nil {
		t.Fail()
	}

	fmt.Printf("%v", msg)
}

