package main

import (
	"fmt"
	"testing"
)

func TestParseRecordRoute(t *testing.T) {
	s := "<sip:ss2.biloxi.example.com;lr>,<sip:ss1.atlanta.example.com;lr>"
	recordRoute, err := ParseRecordRoute(s)
	if err != nil || recordRoute == nil {
		t.Fail()
	}
	fmt.Printf("%v\n", recordRoute)

}
