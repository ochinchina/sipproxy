package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

type RecordRoute struct {
	recRoute []*RecRoute
}

type RecRoute struct {
	nameAddr *NameAddr
	rrParam  []KeyValue
}

func NewRecordRoute() *RecordRoute {
	return &RecordRoute{recRoute: make([]*RecRoute, 0)}
}

func ParseRecordRoute(s string) (*RecordRoute, error) {
	rr := NewRecordRoute()

	for _, t := range strings.Split(s, ",") {
		recRoute, err := ParseRecRoute(t)
		if err != nil {
			return nil, err
		}
		rr.AddRecRoute(recRoute)
	}
	if len(rr.recRoute) > 0 {
		return rr, nil
	} else {
		return nil, errors.New("empty Record-Route")
	}
}

func ParseRecRoute(s string) (*RecRoute, error) {
	recRoute := &RecRoute{nameAddr: nil, rrParam: make([]KeyValue, 0)}

	pos := strings.Index(s, ">")
	if pos == -1 {
		return nil, errors.New("invalid syntax of rec-route")
	}
	nameAddr, err := ParseNameAddr(s[0 : pos+1])
	if err != nil {
		return nil, err
	}
	recRoute.nameAddr = nameAddr
	s = strings.TrimSpace(s[pos+1:])

	if len(s) <= 0 {
		return recRoute, nil
	}
	if s[0] != ';' {
		return nil, errors.New("invalid rec-route syntax")
	}
	s = s[1:]
	for _, t := range strings.Split(s, ";") {
		param, err := ParseGenericParam(t)
		if err != nil {
			return nil, err
		}
		recRoute.rrParam = append(recRoute.rrParam, param)
	}
	return recRoute, nil
}

func (r *RecordRoute) GetRecRouteCount() int {
	return len(r.recRoute)
}

func (r *RecordRoute) GetRecRoute(index int) (*RecRoute, error) {
	if index < 0 || index >= len(r.recRoute) {
		return nil, fmt.Errorf("index %d is out of bound", index)
	}
	return r.recRoute[index], nil
}

func (r *RecordRoute) AddRecRoute(recRoute *RecRoute) {
	r.recRoute = append(r.recRoute, recRoute)
}

func (r *RecordRoute) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))
	for index, recRoute := range r.recRoute {
		if index != 0 {
			fmt.Fprintf(buf, ",")
		}
		fmt.Fprintf(buf, "%v", recRoute)
	}
	return buf.String()
}

func NewRecRoute(nameAddr *NameAddr) *RecRoute {
	return &RecRoute{nameAddr: nameAddr, rrParam: make([]KeyValue, 0)}
}

func (r *RecRoute) GetNameAddr() *NameAddr {
	return r.nameAddr
}

func (r *RecRoute) AddParam(name string, value string) {
	r.rrParam = append(r.rrParam, KeyValue{Key: name, Value: value})
}

func (r *RecRoute) GetParamCount() int {
	return len(r.rrParam)
}

func (r *RecRoute) GetParam(index int) (KeyValue, error) {
	if index < 0 || index >= len(r.rrParam) {
		return KeyValue{Key: "", Value: ""}, errors.New("out of bound")
	}
	return r.rrParam[index], nil
}

func (r *RecRoute) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))
	fmt.Fprintf(buf, "%v", r.nameAddr)
	for _, param := range r.rrParam {
		fmt.Fprintf(buf, ";%v", param)
	}
	return buf.String()
}
