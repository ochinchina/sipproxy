package main

import (
	"testing"
)

func TestParseGenericParamWithValue(t *testing.T) {
	kv, err := ParseGenericParam("test=value1")
	if err != nil {
		t.Fail()
	}
	if kv.Key != "test" || kv.Value != "value1" {
		t.Fail()
	}
}

func TestParseGenericParamWithoutValue(t *testing.T) {
	kv, err := ParseGenericParam("test")
	if err != nil {
		t.Fail()
	}
	if kv.Key != "test" || len(kv.Value) > 0 {
		t.Fail()
	}
}
