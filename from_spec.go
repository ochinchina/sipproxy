package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

type FromSpec struct {
	nameAddr *NameAddr
	addrSpec *AddrSpec
	params   []KeyValue
}

func ParseFromSpec(s string) (*FromSpec, error) {
	r := &FromSpec{nameAddr: nil,
		addrSpec: nil,
		params:   make([]KeyValue, 0)}

	laquot_pos := strings.Index(s, "<")
	raquot_pos := -1
	if laquot_pos != -1 {
		raquot_pos = strings.Index(s, ">")
		if raquot_pos == -1 || raquot_pos < laquot_pos {
			return nil, fmt.Errorf("malformatted header From: %s", s)
		}
	}

	params := ""
	var err error = nil
	if raquot_pos != -1 {
		r.nameAddr, err = ParseNameAddr(s[0 : raquot_pos+1])
		if err != nil {
			return nil, err
		}
		pos := strings.IndexByte(s[raquot_pos+1:], ';')
		if pos != -1 {
			params = s[raquot_pos+1+pos+1:]
		}
	} else {
		pos := strings.IndexByte(s, ';')
		if pos == -1 {
			r.addrSpec, err = ParseAddrSpec(s)
			if err != nil {
				return nil, err
			}
		} else {
			r.addrSpec, err = ParseAddrSpec(s[0 : pos+1])
			if err != nil {
				return nil, err
			}
			params = s[pos+1:]
		}
	}
	if len(params) == 0 {
		return r, nil
	}
	for _, value := range strings.Split(params, ";") {
		kv, err := ParseGenericParam(value)
		if err != nil {
			return nil, err
		}
		r.params = append(r.params, kv)
	}
	return r, nil

}

func (fs *FromSpec) GetAddrSpec() (*AddrSpec, error) {
	if fs.nameAddr != nil {
		return fs.nameAddr.Addr, nil
	} else if fs.addrSpec != nil {
		return fs.addrSpec, nil
	}
	return nil, errors.New("no nameSpec and addrSpec")
}

func (fs *FromSpec) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))

	if fs.nameAddr != nil {
		fmt.Fprintf(buf, "%s", fs.nameAddr.String())
	} else if fs.addrSpec != nil {
		fmt.Fprintf(buf, "%s", fs.addrSpec.String())
	}
	for _, param := range fs.params {
		fmt.Fprintf(buf, ";%s", param.String())
	}
	return buf.String()
}

func (fs *FromSpec) GetParam(name string) (string, error) {
	for _, param := range fs.params {
		if param.Key == name {
			return param.Value, nil
		}
	}
	return "", fmt.Errorf("no such param %s", name)
}

func (fs *FromSpec) GetTag() (string, error) {
	return fs.GetParam("tag")
}

func (fs *FromSpec) SetTag(tag string) {
	for i, param := range fs.params {
		if param.Key == "tag" {
			fs.params[i].Value = tag
			return
		}
	}
	params := make([]KeyValue, 0)
	params = append(params, KeyValue{Key: "tag", Value: tag})
	fs.params = append(params, fs.params...)
}
