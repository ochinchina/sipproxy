package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type SIPURI struct {
	Scheme     string
	User       string
	Password   string
	Host       string
	port       int
	Parameters []KeyValue
	Headers    []KeyValue
}

func ParseSipURI(uri string) (*SIPURI, error) {
	sipUri := &SIPURI{}
	var s string = ""
	if strings.HasPrefix(uri, "sip:") {
		sipUri.Scheme = "sip"
		s = uri[4:]
	} else if strings.HasPrefix(uri, "sips:") {
		sipUri.Scheme = "sips"
		s = uri[5:]
	} else {
		return nil, errors.New("not a valid sip uri")
	}
	// find '?'
	pos := strings.IndexByte(s, '?')
	if pos != -1 {
		parseUriHeader(s[pos+1:], sipUri)
		s = s[0:pos]
	}
	// find ';'
	pos = strings.IndexByte(s, ';')
	if pos != -1 {
		parseUriParameters(s[pos+1:], sipUri)
		s = s[0:pos]
	}
	//find '@'
	pos = strings.IndexByte(s, '@')
	if pos != -1 {
		parseUserInfo(s[0:pos], sipUri)
		parseHostPort(s[pos+1:], sipUri)
	} else {
		parseHostPort(s, sipUri)
	}
	return sipUri, nil
}

func parseUserInfo(s string, sipUri *SIPURI) {
	pos := strings.IndexByte(s, ':')
	if pos == -1 {
		sipUri.User = s
	} else {
		sipUri.User = s[0:pos]
		sipUri.Password = s[pos+1:]
	}
}

func parseHostPort(s string, sipUri *SIPURI) {
	pos := strings.IndexByte(s, ':')
	if pos == -1 {
		sipUri.Host = s
		sipUri.port = 0
	} else {
		sipUri.Host = s[0:pos]
		sipUri.port, _ = strconv.Atoi(s[pos+1:])
	}
}

func parseUriParameters(s string, sipUri *SIPURI) error {
	for _, param := range strings.Split(s, ";") {
		pos := strings.IndexByte(param, '=')
		if pos == -1 {
			if param == "lr" {
				sipUri.Parameters = append(sipUri.Parameters, KeyValue{Key: "lr", Value: ""})
			} else {
				return errors.New("invalid parameter format")
			}
		} else {
			name := param[0:pos]
			value := param[pos+1:]
			sipUri.Parameters = append(sipUri.Parameters, KeyValue{Key: name, Value: value})
		}
	}
	return nil
}

func parseUriHeader(s string, sipUri *SIPURI) error {
	for _, param := range strings.Split(s, "&") {
		pos := strings.IndexByte(param, '=')
		if pos == -1 {
			return errors.New("invalid parameter format")
		}
		name := param[0:pos]
		value := param[pos+1:]
		sipUri.Headers = append(sipUri.Headers, KeyValue{Key: name, Value: value})
	}

	return nil
}

func (s *SIPURI) GetParameter(name string) (string, error) {
	for _, param := range s.Parameters {
		if param.Key == name {
			return param.Value, nil
		}
	}
	return "", fmt.Errorf("no such parameter%s", name)
}

func (s *SIPURI) AddParameter(name string, value string) {
	s.Parameters = append(s.Parameters, KeyValue{Key: name, Value: value})
}

func (s *SIPURI) SetParameter(name string, value string) {
	for i, param := range s.Parameters {
		if param.Key == name {
			s.Parameters[i].Value = value
			return
		}
	}
	s.AddParameter(name, value)
}

func (s *SIPURI) GetTransport() string {
	transport, err := s.GetParameter("transport")
	if err == nil {
		return transport
	} else {
		return "udp"
	}
}

func (s *SIPURI) GetPort() int {
	if s.port != 0 {
		return s.port
	}
	if s.GetTransport() == "tls" {
		return 5061
	} else {
		return 5060
	}
}

func (s *SIPURI) GetHeader(name string) (string, error) {
	for _, header := range s.Headers {
		if header.Key == name {
			return header.Value, nil
		}
	}
	return "", fmt.Errorf("no such header %s", name)
}

func (s *SIPURI) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))
	s.Write(buf)
	return buf.String()
}

func (s *SIPURI) Write(writer io.Writer) (int, error) {
	return s._Write(writer, true, true)
}

func (s *SIPURI) _Write(writer io.Writer, withParams bool, withHeaders bool) (int, error) {
	n, _ := fmt.Fprintf(writer, "%s:", s.Scheme)
	// print userinfo
	if len(s.User) > 0 {
		if len(s.Password) > 0 {
			m, _ := fmt.Fprintf(writer, "%s:%s@", s.User, s.Password)
			n += m
		} else {
			m, _ := fmt.Fprintf(writer, "%s@", s.User)
			n += m
		}
	}
	//hostport
	if s.port != 0 {
		m, _ := fmt.Fprintf(writer, "%s:%d", s.Host, s.port)
		n += m
	} else {
		m, _ := fmt.Fprintf(writer, "%s", s.Host)
		n += m
	}

	//uri-parametres
	if withParams {
		for _, param := range s.Parameters {
			m, _ := fmt.Fprintf(writer, ";")
			n += m
			if len(param.Value) > 0 {
				m, _ = fmt.Fprintf(writer, "%s=%s", param.Key, param.Value)
			} else {
				m, _ = fmt.Fprintf(writer, "%s", param.Key)
			}
			n += m
		}
	}

	//headers
	if withHeaders {
		for i, header := range s.Headers {
			if i == 0 {
				m, _ := fmt.Fprintf(writer, "%s", "?")
				n += m
			} else {
				m, _ := fmt.Fprintf(writer, "%s", "&")
				n += m
			}
			m, err := fmt.Fprintf(writer, "%s=%s", header.Key, header.Value)
			n += m
			if err != nil {
				return n, err
			}
		}
	}
	return n, nil
}

func (s *SIPURI) ToString(withParams bool, withHeaders bool) string {
	writer := bytes.NewBuffer(make([]byte, 0))
	s._Write(writer, withParams, withHeaders)
	return writer.String()
}

