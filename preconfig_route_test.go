package main

import (
	"testing"
)

func TestFindRoute(t *testing.T) {
	pre_route := NewPreConfigRoute()
	pre_route.AddRouteItem("udp", "test.com", "10.0.0.1:3456")
	pre_route.AddRouteItem("tcp", "*.example.com", "10.0.0.2:3456")
	_, _, _, err := pre_route.FindRoute("test.com")
	if err != nil {
		t.Fail()
	}

	_, _, _, err = pre_route.FindRoute("test1.example.com")
	if err != nil {
		t.Fail()
	}

	_, _, _, err = pre_route.FindRoute("test2.com")
	if err == nil {
		t.Fail()
	}

	_, _, _, err = pre_route.FindRoute("mytestexample.com")
	if err == nil {
		t.Fail()
	}
}
