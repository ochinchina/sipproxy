package main

import (
	"fmt"
	"io"
)

type AbsoluteURI struct {
	absURI string
}

func ParseAbsoluteURI(s string) (*AbsoluteURI, error) {
	return &AbsoluteURI{absURI: s}, nil
}

func (au *AbsoluteURI) Writer(writer io.Writer) (int, error) {
	return fmt.Fprintf(writer, au.absURI)
}

func (au *AbsoluteURI) String() string {
	return au.absURI
}
