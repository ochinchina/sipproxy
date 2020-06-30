package main

import (
	"bytes"
	"fmt"
	"strings"
)

type FromSpec struct {
	nameAddr *NameAddr
	params   []KeyValue
}

func ParseFromSpec(s string) (*FromSpec, error) {
	fromSpec := &FromSpec{nameAddr: nil,
		params: make([]KeyValue, 0)}

	for index, t := range strings.Split(s, ";") {
		if index == 0 {
			var err error
			fromSpec.nameAddr, err = ParseNameAddr(t)
			if err != nil {
				return nil, err
			}
		} else {
			param, err := ParseGenericParam(t)
			if err != nil {
				return nil, err
			}
			fromSpec.params = append(fromSpec.params, param)
		}
	}
	return fromSpec, nil
}

func (fs *FromSpec) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))

	fmt.Fprintf(buf, "%s", fs.nameAddr.String())
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
	return "", fmt.Errorf("No such param %s", name)
}

func (fs *FromSpec) GetTag() (string, error) {
	return fs.GetParam("tag")
}

func (fs *FromSpec) SetTag(tag string) {
	for _, param := range fs.params {
		if param.Key == "tag" {
			param.Value = tag
			return
		}
	}
	params := make([]KeyValue, 0)
	params = append(params, KeyValue{Key: "tag", Value: tag})
	fs.params = append(params, fs.params...)
}
