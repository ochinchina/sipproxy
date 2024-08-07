package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Route struct {
	routeParams []*RouteParam
}

type RouteParam struct {
	nameAddr *NameAddr
	rrParam  []KeyValue
}

func NewRouteParam() *RouteParam {
	return &RouteParam{nameAddr: nil, rrParam: make([]KeyValue, 0)}
}
func (rp *RouteParam) GetAddress() *NameAddr {
	return rp.nameAddr
}

func ParseRoute(s string) (*Route, error) {
	route := &Route{}
	for _, routeParam := range strings.Split(s, ",") {
		param, err := parseRouteParam(routeParam)
		if err != nil {
			return nil, err
		}
		route.routeParams = append(route.routeParams, param)
	}
	return route, nil
}

func parseRouteParam(s string) (*RouteParam, error) {
	r := NewRouteParam()
	pos := strings.Index(s, ">")
	if pos == -1 {
		return nil, errors.New("route-param syntax error")
	}
	nameAddr, err := ParseNameAddr(s[0 : pos+1])
	if err != nil {
		return nil, err
	}
	r.nameAddr = nameAddr
	s = strings.TrimSpace(s[pos+1:])
	if len(s) <= 0 {
		return r, nil
	}
	if s[0] != ';' {
		return nil, errors.New("route-param syntax error")
	}
	s = s[1:]
	for _, t := range strings.Split(s, ";") {
		param, err := ParseGenericParam(t)
		if err != nil {
			return nil, err
		}
		r.rrParam = append(r.rrParam, param)
	}
	return r, nil
}

func (r *Route) GetRouteParam(index int) (*RouteParam, error) {
	if index < 0 || index >= len(r.routeParams) {
		return nil, fmt.Errorf("index %d is out of bound", index)
	}
	return r.routeParams[index], nil
}
func (r *Route) PopRouteParam() (*RouteParam, error) {
	if len(r.routeParams) > 0 {
		routeParam := r.routeParams[0]
		r.routeParams = r.routeParams[1:]
		return routeParam, nil
	}
	return nil, errors.New("no route-param")
}

func (r *Route) GetRouteParamCount() int {
	return len(r.routeParams)
}
func (r *Route) Write(writer io.Writer) (int, error) {
	n := 0
	for index, param := range r.routeParams {
		if index != 0 {
			m, err := fmt.Fprintf(writer, ",")
			n += m
			if err != nil {
				return n, err
			}
		}
		m, err := param.Write(writer)
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
func (r *Route) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))
	r.Write(buf)
	return buf.String()

}

func (r *RouteParam) Write(writer io.Writer) (int, error) {
	n, err := r.nameAddr.Write(writer)
	if err != nil {
		return n, err
	}
	for _, param := range r.rrParam {
		m, err := param.Write(writer)
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (r *RouteParam) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))
	r.Write(buf)
	return buf.String()
}
