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

func TestParseFromSepcWithTel(t *testing.T) {
	s := "<tel:+5521967014706;phone-context=ims.mnc010.mcc724.3gppnetwork.org>;tag=rnxyur0z"

	fromSpec, err := ParseFromSpec(s)

	if err != nil {
		t.Errorf("fail to parse From: %s", s)
	}
	if tag, err := fromSpec.GetTag(); err != nil || tag != "rnxyur0z" {
		t.Errorf("Fail to get the tag in header From: %s", s)
	}
}

func TestParseFromSepcWithTelWithoutTag(t *testing.T) {
	s := "Test <tel:+5521967014706;phone-context=ims.mnc010.mcc724.3gppnetwork.org>"

	fromSpec, err := ParseFromSpec(s)

	if err != nil {
		t.Errorf("fail to parse From: %s", s)
	}
	fmt.Println(fromSpec)
	if _, err := fromSpec.GetTag(); err == nil {
		t.Errorf("There should be no tag in header From: %s", s)
	}
}
