package main

import (
	"io"
)

type AbsoluteURI struct {
}

func (au *AbsoluteURI) Writer(writer io.Writer) (int, error) {
	return 0, nil
}

func (au *AbsoluteURI) String() string {
	return ""
}
