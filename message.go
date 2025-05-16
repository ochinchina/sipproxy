package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

type RequestLine struct {
	method string
	// request-uri and addr-spec have same syntax
	requestURI *AddrSpec
	version    string
}

type StatusLine struct {
	version    string
	statusCode int
	reason     string
}

type Header struct {
	name  string
	value interface{}
}

type Message struct {
	request      *RequestLine
	response     *StatusLine
	headers      []*Header
	body         []byte
	ReceivedFrom ServerTransport
}

type compactHeaderNames struct {
	compactHeaders map[string]string
}

func NewCompactHeaderNames() *compactHeaderNames {
	return &compactHeaderNames{compactHeaders: make(map[string]string)}
}

func (ch *compactHeaderNames) AddCompact(name, compactName string) {
	name = strings.ToLower(name)
	compactName = strings.ToLower(compactName)
	ch.compactHeaders[name] = compactName
	ch.compactHeaders[compactName] = name
}

func (ch *compactHeaderNames) GetCompact(name string) (string, bool) {
	name = strings.ToLower(name)
	v, ok := ch.compactHeaders[name]
	return v, ok

}

var compactHdrNames *compactHeaderNames = nil
var finalResponseStatusCodes map[int]bool = map[int]bool{2: true, 3: true, 4: true, 5: true, 6: true}

func init() {
	compactHdrNames = NewCompactHeaderNames()
	compactHdrNames.AddCompact("Accept-Contact", "a")
	compactHdrNames.AddCompact("Referred-By", "b")
	compactHdrNames.AddCompact("Content-Type", "c")
	compactHdrNames.AddCompact("Content-Encoding", "e")
	compactHdrNames.AddCompact("From", "f")
	compactHdrNames.AddCompact("Call-ID", "i")
	compactHdrNames.AddCompact("Supported", "k")
	compactHdrNames.AddCompact("Content-Length", "l")
	compactHdrNames.AddCompact("Contact", "m")
	compactHdrNames.AddCompact("Event", "o")
	compactHdrNames.AddCompact("Refer-To", "r")
	compactHdrNames.AddCompact("Subject", "s")
	compactHdrNames.AddCompact("To", "t")
	compactHdrNames.AddCompact("Allow-Events", "u")
	compactHdrNames.AddCompact("Via", "v")
}
func NewMessage() *Message {
	return &Message{request: nil,
		response:     nil,
		headers:      make([]*Header, 0),
		body:         make([]byte, 0),
		ReceivedFrom: nil}
}

func NewRequest(method string, uri string, version string) (*Message, error) {
	requestURI, err := ParseAddrSpec(uri)
	if err != nil {
		return nil, err
	}
	return &Message{request: &RequestLine{method: method, requestURI: requestURI, version: version},
		response:     nil,
		headers:      make([]*Header, 0),
		body:         make([]byte, 0),
		ReceivedFrom: nil}, nil
}

func NewResponseOf(request *Message, statusCode int, reason string) *Message {
	return &Message{request: nil,
		response:     &StatusLine{version: request.request.version, statusCode: statusCode, reason: reason},
		headers:      make([]*Header, 0),
		body:         make([]byte, 0),
		ReceivedFrom: nil}
}

func readLine(reader *bufio.Reader) ([]byte, error) {
	line, err := reader.ReadBytes('\n')

	if err != nil {
		return nil, err
	}
	n := len(line)
	if n > 0 && line[n-1] == '\n' {
		if n > 1 && line[n-2] == '\r' {
			line = line[0 : n-2]
		} else {
			line = line[0 : n-1]
		}
	}
	return line, nil
}

func isWhiteSpace(b byte) bool {
	return b == '\t' || b == '\n' || b == '\v' || b == '\f' || b == '\r' || b == ' '
}

func skipWhiteSpace(reader *bufio.Reader) error {
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return err
		}
		if !isWhiteSpace(b) {
			err = reader.UnreadByte()
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func isRequestLine(line string) bool {
	return !strings.HasPrefix(line, "SIP/")
}

func parseRequestLine(line string) (*RequestLine, error) {
	fields := strings.Fields(line)
	if len(fields) == 3 {
		requestURI, err := ParseAddrSpec(fields[1])
		if err != nil {
			return nil, err
		}
		return &RequestLine{method: fields[0], requestURI: requestURI, version: fields[2]}, nil
	} else {
		return nil, errors.New("not a valid sip request")
	}
}

func parseStatusLine(line string) (*StatusLine, error) {
	fields := strings.Fields(line)
	if len(fields) >= 3 {
		statusCode, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, err
		}
		return &StatusLine{version: fields[0], statusCode: statusCode, reason: strings.Join(fields[2:], " ")}, nil
	} else {
		return nil, errors.New("not a valid sip response")
	}
}

func ParseMessage(reader *bufio.Reader) (*Message, error) {
	msg := NewMessage()
	firstLine := true
	skipWhiteSpace(reader)
	for {
		bLine, err := readLine(reader)
		if err != nil {
			return nil, err
		}
		if len(bLine) == 0 {
			break
		}
		line := string(bLine)
		if firstLine {
			if isRequestLine(line) {
				request, err := parseRequestLine(line)
				if err != nil {
					return nil, err
				}
				msg.request = request
			} else {
				response, err := parseStatusLine(line)
				if err != nil {
					zap.L().Error("Fail to parse status line", zap.String("line", line), zap.String("error", err.Error()))
					return nil, err
				}
				msg.response = response
			}
			firstLine = false
		} else {
			pos := strings.IndexByte(line, ':')
			if pos == -1 {
				return nil, errors.New("not a valid sip request")
			}
			name := line[0:pos]
			value := strings.TrimSpace(line[pos+1:])
			msg.AddHeader(name, value)
		}
	}
	contentLength, err := msg.GetHeaderInt("Content-Length")
	if err != nil {
		return nil, err
	}
	if contentLength < 0 {
		return nil, errors.New("invalid negative Content-Length field")
	}
	msg.body = make([]byte, contentLength)
	if _, err = io.ReadFull(reader, msg.body); err != nil {
		return nil, err
	}
	return msg, nil
}

func (m *Message) AddHeader(name string, value string) {
	m.headers = append(m.headers, &Header{name: name, value: value})
}

func (m *Message) isSameHeader(name_1, name_2 string) bool {
	if strings.EqualFold(name_1, name_2) {
		return true
	}
	compact, ok := compactHdrNames.GetCompact(name_2)
	return ok && strings.EqualFold(name_1, compact)

}

func (m *Message) GetHeader(name string) (*Header, error) {
	for _, header := range m.headers {
		if m.isSameHeader(header.name, name) {
			return header, nil
		}
	}
	return nil, fmt.Errorf("no such header %s", name)
}

func (m *Message) findHeaderPos(name string) (int, error) {
	for index, header := range m.headers {
		if m.isSameHeader(header.name, name) {
			return index, nil
		}
	}
	return 0, fmt.Errorf("no such header %s", name)

}

// GetHeader get the header value by name
func (m *Message) GetHeaderValue(name string) (interface{}, error) {
	header, err := m.GetHeader(name)
	if err != nil {
		return nil, err
	}
	return header.value, nil
}

// GetHeaderInt get the header value as integer
func (m *Message) GetHeaderInt(name string) (int, error) {
	v, err := m.GetHeaderValue(name)
	if err == nil {
		if s, ok := v.(string); ok {
			return strconv.Atoi(s)
		}
		return 0, errors.New("not a string type header")
	}
	return 0, err
}

// RemoveHeader remove the first header whose name is name and
// return the value of the header
func (m *Message) RemoveHeader(name string) (interface{}, error) {
	for index, header := range m.headers {
		if m.isSameHeader(header.name, name) {
			m.headers = append(m.headers[0:index], m.headers[index+1:]...)
			return header.value, nil
		}
	}
	return "", fmt.Errorf("no such header %s", name)
}

// Get the first From header
// If the header is not a FromSpec, parse it and set the value to FromSpec
func (m *Message) GetFrom() (*FromSpec, error) {
	header, err := m.GetHeader("From")
	if err != nil {
		return nil, err
	}
	if t, ok := header.value.(*FromSpec); ok {
		return t, nil
	}
	if s, ok := header.value.(string); ok {
		t, err := ParseFromSpec(s)
		if err != nil {
			return nil, err
		}
		header.value = t
		return t, nil
	}
	return nil, errors.New("type of the From header is not string or From")
}

// Get the To header
// If the header is not a To, parse it and set the value to To
func (m *Message) GetTo() (*To, error) {
	header, err := m.GetHeader("To")
	if err != nil {
		return nil, err
	}
	if t, ok := header.value.(*To); ok {
		return t, nil
	}
	if s, ok := header.value.(string); ok {
		t, err := ParseTo(s)
		if err != nil {
			return nil, err
		}
		header.value = t
		return t, nil
	}
	return nil, errors.New("type of the To header is not string or To")
}

// Get first route item
func (m *Message) GetRoute() (*Route, error) {
	header, err := m.GetHeader("Route")
	if err != nil {
		return nil, err
	}
	if r, ok := header.value.(*Route); ok {
		return r, nil
	}
	if s, ok := header.value.(string); ok {
		r, err := ParseRoute(s)
		if err != nil {
			return nil, err
		}
		header.value = r
		return r, nil
	}
	return nil, errors.New("type of Route header is not string or Route")
}

// PopRoute remove first route
func (m *Message) PopRoute() error {
	route, err := m.GetRoute()
	if err != nil {
		return err
	}

	if route.GetRouteParamCount() > 1 {
		_, err = route.PopRouteParam()
	} else {
		_, err = m.RemoveHeader("Route")
	}
	return err
}

func (m *Message) findViaInsertPos() int {
	for index, header := range m.headers {
		if m.isSameHeader(header.name, "Via") {
			return index
		}
	}
	return 0
}

// Add the Via header to the top of Via headers
func (m *Message) AddVia(via *Via) {
	pos := m.findViaInsertPos()
	headers := make([]*Header, 0)
	headers = append(headers, m.headers[0:pos]...)
	headers = append(headers, &Header{name: "Via", value: via})
	headers = append(headers, m.headers[pos:]...)
	m.headers = headers
}

// Get the top Via header
func (m *Message) GetVia() (*Via, error) {
	header, err := m.GetHeader("Via")
	if err != nil {
		return nil, err
	}

	if v, ok := header.value.(*Via); ok {
		return v, nil
	}
	if s, ok := header.value.(string); ok {
		v, err := ParseVia(s)
		if err != nil {
			return nil, err
		}
		header.value = v
		return v, nil

	}
	return nil, errors.New("the header value type is not string or Via")
}

func (m *Message) GetCSeq() (*CSeq, error) {
	header, err := m.GetHeader("CSeq")
	if err != nil {
		return nil, err
	}

	if v, ok := header.value.(*CSeq); ok {
		return v, nil
	}
	if s, ok := header.value.(string); ok {
		v, err := ParseCSeq(s)
		if err != nil {
			return nil, err
		}
		header.value = v
		return v, nil

	}
	return nil, errors.New("the header value type is not string or CSeq")
}

// PopVia remove first via
func (m *Message) PopVia() (*Via, error) {
	via, err := m.GetVia()
	if err != nil {
		return nil, err
	}
	if via.Size() > 1 {
		viaParam, err := via.PopViaParam()
		via = NewVia()
		via.AddViaParam(viaParam)
		return via, err
	} else {
		_, err := m.RemoveHeader("Via")
		return via, err
	}
}

func (m *Message) ForEachVia(viaProcessor func(*Via)) {
	for _, header := range m.headers {
		if !m.isSameHeader(header.name, "Via") {
			continue
		}
		if v, ok := header.value.(*Via); ok {
			viaProcessor(v)
		}
		if s, ok := header.value.(string); ok {
			v, err := ParseVia(s)
			if err != nil {
				continue
			}
			header.value = v
			viaProcessor(v)
		}
	}

}

func (m *Message) ForEachViaParam(handler func(*ViaParam)) {
	m.ForEachVia(func(via *Via) {
		n := via.Size()
		for i := 0; i < n; i++ {
			param, err := via.GetParam(i)
			if err == nil {
				handler(param)
			}
		}
	})
}

func (m *Message) findRecordRoutePos() int {
	pos, err := m.findHeaderPos("Record-Route")
	if err == nil {
		return pos
	}
	pos_1, err_1 := m.findHeaderPos("From")
	pos_2, err_2 := m.findHeaderPos("Max-Forwards")
	if err_1 == nil && err_2 == nil {
		if pos_1 < pos_2 {
			return pos_1
		}
		return pos_2
	}
	if err_1 == nil {
		return pos_1
	} else if err_2 == nil {
		return pos_2
	}
	return 0
}

func (m *Message) AddRecordRoute(recordRoute *RecordRoute) {
	pos := m.findRecordRoutePos()
	headers := make([]*Header, 0)
	headers = append(headers, m.headers[0:pos]...)
	headers = append(headers, &Header{name: "Record-Route", value: recordRoute})
	headers = append(headers, m.headers[pos:]...)
	m.headers = headers
}

func (m *Message) GetRecordRoute() (*RecordRoute, error) {
	header, err := m.GetHeader("Record-Route")
	if err != nil {
		return nil, err
	}
	if rr, ok := header.value.(*RecordRoute); ok {
		return rr, nil
	}
	if s, ok := header.value.(string); ok {
		rr, err := ParseRecordRoute(s)
		if err != nil {
			return nil, err
		}
		header.value = rr
		return rr, nil
	}
	return nil, errors.New("the header value type is not string or Record-Route")
}

func (m *Message) PopRecordRoute() (*RecordRoute, error) {
	header, err := m.RemoveHeader("Record-Route")
	if err != nil {
		return nil, err
	}
	if rr, ok := header.(*RecordRoute); ok {
		return rr, nil
	}
	if s, ok := header.(string); ok {
		rr, err := ParseRecordRoute(s)
		if err == nil {
			return rr, nil
		} else {
			return nil, err
		}
	}
	return nil, errors.New("the header value type is not string or Record-Route")
}

func (m *Message) Write(writer io.Writer) (int, error) {
	n, _ := m.encodeFirstLine(writer)
	k, _ := m.encodeHeader(writer)
	n += k
	k, _ = fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(m.body))
	n += k
	k, err := writer.Write(m.body)
	n += k
	return n, err
}

func (m *Message) Bytes() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0))
	_, err := m.Write(buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (m *Message) encodeFirstLine(writer io.Writer) (int, error) {
	if m.request != nil {
		return fmt.Fprintf(writer, "%s %v %s\r\n", m.request.method, m.request.requestURI, m.request.version)
	} else if m.response != nil {
		return fmt.Fprintf(writer, "%s %d %s\r\n", m.response.version, m.response.statusCode, m.response.reason)
	}
	return 0, nil
}

func (m *Message) encodeHeader(writer io.Writer) (int, error) {
	n := 0
	for _, header := range m.headers {
		if header.name == "Content-Length" {
			continue
		}
		k, err := fmt.Fprintf(writer, "%s: %v\r\n", header.name, header.value)
		n += k
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (m *Message) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))
	m.Write(buf)
	return buf.String()
}

// IsRequest return true if the message is request message
func (m *Message) IsRequest() bool {
	return m.request != nil
}

func (m *Message) GetMethod() (string, error) {
	if m.request != nil {
		return m.request.method, nil
	} else {
		cseq, err := m.GetCSeq()
		if err != nil {
			return "", err
		}
		return cseq.Method, nil
	}
}

func (m *Message) GetRequestURI() (*AddrSpec, error) {
	if m.request == nil {
		return nil, errors.New("not a request")
	}
	return m.request.requestURI, nil
}

func (m *Message) IsResponse() bool {
	return m.response != nil
}

// All 2xx, 3xx, 4xx, 5xx and 6xx responses are final.
func (m *Message) IsFinalResponse() bool {
	if m.response == nil {
		return false
	}

	value, ok := finalResponseStatusCodes[m.response.statusCode/100]
	return ok && value
}

func (m *Message) SetReceived(peerAddr string, peerPort int) error {
	via, err := m.GetVia()
	if err != nil {
		return err
	}
	viaParam, err := via.GetParam(0)
	if err != nil {
		return err
	}
	viaParam.SetReceived(peerAddr)
	if viaParam.HasParam("rport") {
		viaParam.SetParam("rport", fmt.Sprintf("%d", peerPort))
	}
	return nil

}

func (m *Message) TryRemoveTopRoute(myAddr string, myPort int) error {
	route, err := m.GetRoute()
	if err != nil {
		return err
	}
	routeParam, err := route.GetRouteParam(0)
	if err != nil {
		return err
	}
	sipUri, err := routeParam.GetAddress().GetAddress().GetSIPURI()
	if err == nil && sipUri.Host == myAddr && sipUri.GetPort() == myPort {
		zap.L().Info("remove top route item because the top item is my address", zap.String("route-param", routeParam.String()))
		m.PopRoute()
		return nil
	}
	return fmt.Errorf("the top route item is not my address")
}

// Get the Call-ID header
func (m *Message) GetCallID() (string, error) {
	v, err := m.GetHeaderValue("Call-ID")
	if err != nil {
		return "", err
	}
	if s, ok := v.(string); ok {
		return s, nil
	}

	return "", errors.New("Call-ID is not a string")
}

// Get the Dialog:
// The combination of the To tag, From tag,
// and Call-ID completely defines a peer-to-peer SIP relationship
// between Alice and Bob and is referred to as a dialog.
func (m *Message) GetDialog() (string, error) {
	callId, err := m.GetCallID()
	if err != nil {
		return "", err
	}
	fromSpec, err := m.GetFrom()
	if err != nil {
		return "", err
	}

	from_tag, _ := fromSpec.GetTag()

	//if err != nil {
	//	return "", err
	//}

	to, err := m.GetTo()

	if err != nil {
		return "", err
	}

	to_tag, _ := to.GetTag()

	//if err != nil {
	//	return "", err
	//}
	from_addr, err := fromSpec.GetAddrSpec()
	if err != nil {
		return "", err
	}
	to_addr, err := to.GetAddrSpec()
	if err != nil {
		return "", err
	}
	from_addr_s, err := m.getDialogAddr(from_addr)
	if err != nil {
		return "", err
	}
	to_addr_s, err := m.getDialogAddr(to_addr)
	if err != nil {
		return "", err
	}
	if from_addr_s < to_addr_s {
		return NewDialog(callId,
			fmt.Sprintf("%s-%s", from_tag, from_addr_s),
			fmt.Sprintf("%s-%s", to_tag, to_addr_s)).String(), nil
	} else {
		return NewDialog(callId,
			fmt.Sprintf("%s-%s", to_tag, to_addr_s),
			fmt.Sprintf("%s-%s", from_tag, from_addr_s)).String(), nil
	}
}

func (m *Message) getDialogAddr(addr *AddrSpec) (string, error) {
	if addr.IsSIPURI() {
		sip_uri, err := addr.GetSIPURI()
		if err != nil {
			return "", err
		}
		return sip_uri.ToString(false, false), nil
	} else {
		return addr.String(), nil
	}
}
func (m *Message) GetTopViaBranch() (string, error) {
	// Get the Branch
	via, err := m.GetVia()

	if err != nil {
		return "", err
	}

	param, err := via.GetParam(0)

	if err != nil {
		return "", err
	}

	return param.GetBranch()
}

func (m *Message) GetTopViaSentBy() (string, error) {
	// Get the Branch
	via, err := m.GetVia()

	if err != nil {
		return "", err
	}
	if via.Size() <= 0 {
		return "", fmt.Errorf("no Via header is available")
	}

	param, err := via.GetParam(0)

	if err != nil {
		return "", err
	}

	return param.GetSentBy(), nil
}

// GetClientTransaction get the client transaction from message
// When the transport layer in the client receives a response, it has to
// determine which client transaction will handle the response, so that
// the processing of Sections 17.1.1 and 17.1.2 can take place.  The
// branch parameter in the top Via header field is used for this
// purpose.  A response matches a client transaction under two
// conditions:
//
//    1.  If the response has the same value of the branch parameter in
// 	   the top Via header field as the branch parameter in the top
// 	   Via header field of the request that created the transaction.
//
//    2.  If the method parameter in the CSeq header field matches the
// 	   method of the request that created the transaction.  The
// 	   method is needed since a CANCEL request constitutes a
// 	   different transaction, but shares the same value of the branch
// 	   parameter.

func (m *Message) GetClientTransaction() (string, error) {
	cseq, err := m.GetCSeq()
	if err != nil {
		return "", err
	}

	// There is no client transaction for ACK
	if cseq.Method == "ACK" {
		return "", fmt.Errorf("no client transaction for ACK")
	}
	// Get the TopVia Branch
	branch, err := m.GetTopViaBranch()

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%s", cseq.Method, branch), nil

}

// When a request is received from the network by the server, it has to
// be matched to an existing transaction.  This is accomplished in the
// following manner.
//
// The branch parameter in the topmost Via header field of the request
// is examined.  If it is present and begins with the magic cookie
// "z9hG4bK", the request was generated by a client transaction
// compliant to this specification.  Therefore, the branch parameter
// will be unique across all transactions sent by that client.  The
// request matches a transaction if:
//
//  1. the branch parameter in the request is equal to the one in the
//     top Via header field of the request that created the
//     transaction, and
//
//  2. the sent-by value in the top Via of the request is equal to the
//     one in the request that created the transaction, and
//
//  3. the method of the request matches the one that created the
//     transaction, except for ACK, where the method of the request
//     that created the transaction is INVITE.
func (m *Message) GetServerTransaction() (string, error) {
	//if m.IsResponse() {
	//	return "", fmt.Errorf("no server transaction for response")
	//}

	// Get the top Via Branch
	branch, err := m.GetTopViaBranch()

	if err != nil {
		return "", err
	}

	//Get the top Via Sent-By
	sentBy, err := m.GetTopViaSentBy()
	if err != nil {
		return "", err
	}

	method, err := m.GetMethod()

	if err != nil {
		return "", err
	}

	// the method of the request matches the one that created the
	// transaction, except for ACK, where the method of the request
	// that created the transaction is INVITE.

	// If the method is CANCEL, the method of the request that created the transaction is INVITE.
	// The method of the request that created the transaction is INVITE.
	if method == "CANCEL" || method == "ACK" {
		method = "INVITE"
	}

	return fmt.Sprintf("%s-%s-%s", method, sentBy, branch), nil
}

// Get the value of Expires
func (m *Message) GetExpires(defValue int) int {
	expires, err := m.GetHeaderInt("Expires")
	if err == nil {
		return expires
	}
	return defValue
}

func (m *Message) Clone() *Message {
	headers := make([]*Header, len(m.headers))
	copy(headers, m.headers)

	return &Message{request: m.request,
		response:     m.response,
		headers:      headers,
		body:         m.body,
		ReceivedFrom: m.ReceivedFrom}
}

