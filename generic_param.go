package main

import (
	"errors"
	"strings"
)

func ParseGenericParam(s string) (KeyValue, error) {
	if len(s) <= 0 {
		return KeyValue{Key: "", Value: ""}, errors.New("Invalid generic-param syntax")
	}
	pos := strings.IndexByte(s, '=')
	if pos == -1 {
		return KeyValue{Key: s, Value: ""}, nil
	} else {
		return KeyValue{Key: s[0:pos], Value: s[pos+1:]}, nil
	}
}
