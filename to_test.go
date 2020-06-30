package main

import (
	"fmt"
	"testing"
)

func TestParseToHeader(t *testing.T) {
	to, err := ParseTo("Bob <sip:bob@biloxi.example.com>;tag=8321234356")
	if err != nil {
		t.Fail()
	}
	fmt.Println(to)
	if tag, _ := to.GetTag(); tag != "8321234356" {
		t.Fail()
	}
}
