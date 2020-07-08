package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type RequestLine struct {
	method  string
	uri     string
	version string
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
	request  *RequestLine
	response *StatusLine
	headers  []*Header
	body     []byte
}

func NewMessage() *Message {
	return &Message{headers: make([]*Header, 0),
		body: make([]byte, 0)}
}

func NewRequest(method string, uri string, version string) *Message {
	return &Message{request: &RequestLine{method: method, uri: uri, version: version},
		response: nil,
		headers:  make([]*Header, 0),
		body:     make([]byte, 0)}
}

func NewResponseOf(request *Message, statusCode int, reason string) *Message {
	return &Message{request: nil,
		response: &StatusLine{version: request.request.version, statusCode: statusCode, reason: reason},
		headers:  make([]*Header, 0),
		body:     make([]byte, 0)}
}

func readLine(reader *bufio.Reader) ([]byte, error) {
	line, isPrefix, err := reader.ReadLine()
	if err != nil {
		return nil, err
	}
	if !isPrefix {
		return line, err
	}
	for {
		b, isPrefix, err := reader.ReadLine()
		if err != nil {
			return nil, err
		}
		line = append(line, b...)
		if !isPrefix {
			return line, nil
		}
	}
}

func isRequestLine(line string) bool {
	return !strings.HasPrefix(line, "SIP/")
}

func parseRequestLine(line string) (*RequestLine, error) {
	fields := strings.Fields(line)
	if len(fields) == 3 {
		return &RequestLine{method: fields[0], uri: fields[1], version: fields[2]}, nil
	} else {
		return nil, errors.New("Not a valid sip request")
	}
}

func parseStatusLine(line string) (*StatusLine, error) {
	fields := strings.Fields(line)
	if len( fields ) >= 3 {
		statusCode, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, err
		}
		return &StatusLine{version: fields[0], statusCode: statusCode, reason: strings.Join( fields[2:], " ") }, nil
	} else {
		return nil, errors.New("Not a valid sip response")
	}
}

func ParseMessage(b []byte) (*Message, error) {
	buf := bytes.NewBuffer(b)
	reader := bufio.NewReader(buf)
	msg := NewMessage()
	firstLine := true
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
					return nil, err
				}
				msg.response = response
			}
			firstLine = false
		} else {
			pos := strings.IndexByte(line, ':')
			if pos == -1 {
				return nil, errors.New("Not a valid sip request")
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
		return nil, errors.New("Invalid negative Content-Length field")
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

func (m *Message) GetHeader(name string) (*Header, error) {
	for _, header := range m.headers {
		if header.name == name {
			return header, nil
		}
	}
	return nil, fmt.Errorf("No such header %s", name)
}

func (m *Message) findHeaderPos(name string) (int, error) {
	for index, header := range m.headers {
		if header.name == name {
			return index, nil
		}
	}
	return 0, fmt.Errorf("No such header %s", name)

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
		if header.name == name {
			m.headers = append(m.headers[0:index], m.headers[index+1:]...)
			return header.value, nil
		}
	}
	return "", fmt.Errorf("No such header %s", name)
}

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
		if header.name == "v" || header.name == "Via" {
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
	return nil, errors.New("The haeder value type is not string or Via")
}

// remove first via
func (m *Message) PopVia() error {
	via, err := m.GetVia()
	if err != nil {
		return err
	}
	if via.Size() > 1 {
		_, err := via.PopViaParam()
		return err
	} else {
		_, err := m.RemoveHeader("Via")
		return err
	}
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

func (m *Message) encodeFirstLine(writer io.Writer) (int, error) {
	if m.request != nil {
		return fmt.Fprintf(writer, "%s %s %s\r\n", m.request.method, m.request.uri, m.request.version)
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

func (m *Message) isResponse() bool {
	return m.response != nil
}
