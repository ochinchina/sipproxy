package main

import (
	"errors"
	"io"
	"strings"
)

type AddrSpec struct {
	sipURI      *SIPURI
	absoluteURI *AbsoluteURI
}

func NewAddrSpec() *AddrSpec {
	return &AddrSpec{sipURI: nil, absoluteURI: nil}
}
func ParseAddrSpec(addrSpec string) (*AddrSpec, error) {
	if strings.HasPrefix(addrSpec, "sip:") || strings.HasPrefix(addrSpec, "sips:") {
		sipUri, err := ParseSipURI(addrSpec)
		if err == nil {
			return &AddrSpec{sipURI: sipUri, absoluteURI: nil}, nil
		} else {
			return nil, err
		}
	} else {
		return nil, errors.New("Unsupported aboluteURI scheme")
	}
}

func (as *AddrSpec) IsSIPURI() bool {
	return as.sipURI != nil
}

func (as *AddrSpec) GetSIPURI() (*SIPURI, error) {
	if as.sipURI == nil {
		return nil, errors.New("addr-spec is not SIP URI")
	}
	return as.sipURI, nil
}

func (as *AddrSpec) Write(writer io.Writer) (int, error) {
	if as.sipURI != nil {
		return as.sipURI.Write(writer)
	} else if as.absoluteURI != nil {
		return as.absoluteURI.Writer(writer)
	}
	return 0, nil
}
func (as *AddrSpec) String() string {
	if as.sipURI != nil {
		return as.sipURI.String()
	} else if as.absoluteURI != nil {
		return as.absoluteURI.String()
	}
	return ""
}
