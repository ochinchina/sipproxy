package main

import (
	"bytes"
	"bufio"
	"fmt"
	"testing"
)


func TestIsMyMessage(t *testing.T ) {
	s:= `admin:
  addr: "127.0.0.1:8899"
proxies:
- name: .+
  listens:
  - address: 192.168.0.25
    udp-port: 5060
    no-received: true
    backends:
    - udp://nls-lrf:5061
`
	msg_txt := `INVITE urn:service:sos SIP/2.0
Via: SIP/2.0/UDP 192.168.0.42:5061;branch=z9hG4bK-343538-f5cc2dfbdfbc60a7ee8cb62d931fec35,SIP/2.0/UDP 10.166.226.87:5066;nwkintf=6;realm=realm-mw;recvdsrvport=5060;recvdsrvip=10.166.226.86;mav-udp-rport=5066;received=10.166.226.87;branch=z9hG4bKmavodi-0-1a3-e30-1-2000000-89e0c9-35-1e36-2799083920-30247
Max-Forwards: 66
From: <sip:73990000004@msg.pc.t-mobile.com>;tag=7566dc2f
To: <urn:service:sos>
CSeq: 1 INVITE
Contact: <sip:54375929@10.161.118.70:5060;trunk-context=10.161.118.70;eribind-generated-at=10.161.118.70;EriBindingId=54375929;transport=udp>;+g.3gpp.icsi-ref="urn%3Aurn-7%3A3gpp-service.ims.icsi.mmtel";+sip.instance="<urn:gsma:imei:35375707-002406-0>"
Accept: application/sdp,application/3gpp-ims+xml
P-Early-Media: supported
Supported: sec-agree,precondition,100rel
Allow: INVITE,ACK,OPTIONS,CANCEL,BYE,UPDATE,INFO,REFER,NOTIFY,MESSAGE,PRACK
Accept-Contact: *;+g.3gpp.icsi-ref="urn%3Aurn-7%3A3gpp-service.ims.icsi.mmtel"
Record-Route: <sip:192.168.0.42:5061;lr>,<sip:mavodi-0-177-1c7-1-1150000-@10.166.226.86:5060;lr>,<sip:mavodi-0-1a2-3fffffff-1-ffffffff-@10.166.226.87:5066;mavsipodi-0-1a9-35-1-1e36;lr>
P-Charging-Vector: icid-value=sgc3.daatf005.sip.t-mobile.com-1485-460188-67377;icid-generated-at=sgc3.daatf005.sip.t-mobile.com
P-Asserted-Identity: <sip:73990000004@msg.pc.t-mobile.com>
P-Access-Network-Info: 3GPP-E-UTRAN-FDD; utran-cell-id-3gpp=724112c340001815;local-time-zone="UTC-08:00";daylight-saving-time="00",3GPP-E-UTRAN; utran-cell-id-3gpp=724112c340001816
User-Agent: T-Mobile VoLTE-RCS-ussd SEC/N920TUVU4D 6.0.1 Mavenir UAG/v4.5 EATF/v4.5-14042501o
Priority: emergency
Content-Type: application/sdp
Call-ID: 00e03a72a978632ff180d3bd18736257@192.168.0.42
Content-Length: 68

v=0
o=SAMSUNG-IMS-UE 1485460186254456 0 IN IP4 10.161.118.70
s=SS VOIP`

	msg, err := ParseMessage( bufio.NewReader(bytes.NewBufferString(msg_txt)))

	if err != nil {
		t.Fail()
	}
	r := bytes.NewBufferString( s )
	config, err := loadConfigFromReader( r )
	if err != nil {
		t.Fail()
	}
	myName := NewMyName( config.Proxies[0].Name )

	if !myName.isMyMessage( msg ) {
		t.Fail()
	}
	
}

func TestIsMyMessage2(t *testing.T ) {
        s:= `admin:
  addr: "127.0.0.1:8899"
proxies:
- name: .+
  listens:
  - address: 192.168.0.25
    udp-port: 5060
    no-received: true
    backends:
    - udp://nls-lrf:5061
`
	msg_txt := `INVITE sip:194;phone-context=ims.mnc089.mcc724.3gppnetwork.org@ims.mnc089.mcc724.3gppnetwork.org;user=phone SIP/2.0
Via: SIP/2.0/UDP 10.234.126.14:5060;branch=z9hG4bKaqg951gs67674661gs8q79v68;Role=3;Hpt=8e52_16;TRC=3bf-ffffffff;srti=s3_2
Record-Route: <sip:10.234.126.14:5060;transport=udp;lr;Hpt=8e52_16;CxtId=4;TRC=3bf-ffffffff;srti=s3_2;X-HwB2bUaCookie=701>
Call-ID: ecscfasbcGOecbuV-B@10.234.35.201
From: <sip:+5521998988082@ims.mnc089.mcc724.3gppnetwork.org>;tag=HPecbuV
To: "194"<sip:194;phone-context=ims.mnc089.mcc724.3gppnetwork.org@ims.mnc089.mcc724.3gppnetwork.org;user=phone>
CSeq: 1 INVITE
Accept: application/sdp,application/3gpp-ims+xml
Allow: INVITE,ACK,BYE,CANCEL,UPDATE,INFO,PRACK,SUBSCRIBE,NOTIFY,REFER
Contact: <sip:724895730000158@10.234.126.14:5060;transport=udp;Hpt=8e52_16;CxtId=4;TRC=3bf-ffffffff;srti=s3_2>;+sip.instance="<urn:gsma:imei:86737602-519495-0>";+g.3gpp.icsi-ref="urn%3Aurn-7%3A3gpp-service.ims.icsi.mmtel";video;+g.3gpp.mid-call;+g.3gpp.srvcc-alerting
Max-Forwards: 68
Priority: emergency
Supported: 100rel,histinfo,join,norefersub,replaces,timer
User-Agent: IM-client/OMA1.0 HW-VxW/V1.0
Session-Expires: 1800
Min-SE: 90
P-Asserted-Identity: <sip:+5521998988082@ims.mnc089.mcc724.3gppnetwork.org>,<tel:+5521998988082>
Privacy: none;none
P-Visited-Network-ID: "ims.mnc089.mcc724.3gppnetwork.org"
P-Access-Network-Info: 3GPP-E-UTRAN-FDD;utran-cell-id-3gpp=7248926C3000C915;network-provided
P-Charging-Vector: icid-value="rjo-lab-pcscf01.192.44c.20210810113803"
P-Early-Media: supported,gated
P-Preferred-Service: urn:urn-7:3gpp-service.ims.icsi.mmtel
Accept-Contact: *;+g.3gpp.icsi-ref="urn%3Aurn-7%3A3gpp-service.ims.icsi.mmtel"
Content-Type: application/sdp
Content-Length: 4

test`

	msg, err := ParseMessage( bufio.NewReader(bytes.NewBufferString(msg_txt)))

        if err != nil {
                t.Fail()
        }
        r := bytes.NewBufferString( s )
        config, err := loadConfigFromReader( r )
        if err != nil {
                t.Fail()
        }
	fmt.Println(config.Proxies[0].Name)
        myName := NewMyName( config.Proxies[0].Name )

        if !myName.isMyMessage( msg ) {
                t.Fail()
        }

}
