package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

type To struct {
	nameAddr *NameAddr
	addrSpec *AddrSpec
	params   []KeyValue
}

func ParseTo(s string) (*To, error) {
	r := &To{nameAddr: nil,
		addrSpec: nil,
		params:   make([]KeyValue, 0)}

	for index, value := range strings.Split(s, ";") {
		if index == 0 {
			if strings.Index(value, "<") != -1 {
				nameAddr, err := ParseNameAddr(value)
				if err != nil {
					return nil, err
				}
				r.nameAddr = nameAddr
			} else {
				addrSpec, err := ParseAddrSpec(value)
				if err != nil {
					return nil, err
				}
				r.addrSpec = addrSpec
			}
		} else {
			kv, err := ParseGenericParam(value)
			if err != nil {
				return nil, err
			}
			r.params = append(r.params, kv)
		}
	}
	return r, nil
}

func (t *To) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))

	if t.nameAddr != nil {
		fmt.Fprintf(buf, "%s", t.nameAddr)
	} else {
		fmt.Fprintf(buf, "%s", t.addrSpec)
	}

	for _, kv := range t.params {
		fmt.Fprintf(buf, ";%s", kv)
	}
	return buf.String()
}

func (t *To) GetParam(name string) (string, error) {
	for _, param := range t.params {
		if name == param.Key {
			return param.Value, nil
		}
	}
	return "", fmt.Errorf("No such param %s", name)
}

func (t *To) GetTag() (string, error) {
	return t.GetParam("tag")
}

func (t *To) AddParam(name, value string) {
	for _, param := range t.params {
		if param.Key == name {
			param.Value = value
			return
		}
	}
	t.params = append(t.params, KeyValue{Key: name, Value: value})
}

func (t *To) GetAddrSpec() (*AddrSpec, error) {
	addrSpec := t.addrSpec
	if t.nameAddr != nil {
		addrSpec = t.nameAddr.Addr
	}
	if addrSpec == nil {
		return nil, errors.New("no name-addr or addr-spec found")
	}
	return addrSpec, nil
}
func (t *To) GetAbsoluteURI() (string, error) {
	addrSpec, err := t.GetAddrSpec()
	if err != nil {
		return "", err
	}
	absURI, err := addrSpec.GetAbsoluteURI()
	if err != nil {
		return "", err
	}
	return absURI.String(), nil
}

// GetUserHost Get the user and host with user@host format
func (t *To) GetUserHost() (string, error) {
	addrSpec, err := t.GetAddrSpec()
	if err != nil {
		return "", err
	}
	sipUri, err := addrSpec.GetSIPURI()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s@%s", sipUri.User, sipUri.Host), nil
}

// GetHost Get the host
func (t *To) GetHost() (string, error) {
	addrSpec := t.addrSpec
	if t.nameAddr != nil {
		addrSpec = t.nameAddr.Addr
	}
	if addrSpec == nil {
		return "", errors.New("no host found")
	}
	sipUri, err := addrSpec.GetSIPURI()
	if err != nil {
		return "", err
	}
	return sipUri.Host, nil
}
