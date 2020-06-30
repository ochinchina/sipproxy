package main

import (
	"fmt"
	"testing"
)

func TestParseFromSpecWithDisplayName(t *testing.T) {
	fromSpec, err := ParseFromSpec("Bob <sips:bob@biloxi.example.com>;tag=a73kszlfl")
	if err != nil {
		t.Fail()
	}
	if tag, err := fromSpec.GetTag(); err != nil || tag != "a73kszlfl" {
		t.Fail()
	}
	fmt.Printf("%s\n", fromSpec.String())
}

func TestParseFromSpecWithoutDisplayName(t *testing.T) {
	fromSpec, err := ParseFromSpec("<sips:bob@biloxi.example.com>")
	if err != nil {
		t.Fail()
	}
	tag, err := CreateTag()
	if err != nil {
		t.Fail()
	}
	fromSpec.SetTag(tag)

	if tmp, err := fromSpec.GetTag(); err != nil || tmp != tag {
		t.Fail()
	}
	fmt.Printf("%s\n", fromSpec.String())
}
