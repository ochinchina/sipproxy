package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

type NameAddr struct {
	DisplayName string
	Addr        *AddrSpec
}

func ParseNameAddr(nameAddr string) (*NameAddr, error) {
	pos1 := strings.IndexByte(nameAddr, '<')
	pos2 := strings.IndexByte(nameAddr, '>')
	if pos1 == -1 || pos2 == -1 || pos2 < pos1 {
		return nil, errors.New("Malformatted name-addr")
	}

	addr, err := ParseAddrSpec(nameAddr[pos1+1 : pos2])
	if err != nil {
		return nil, err
	}

	return &NameAddr{DisplayName: nameAddr[0:pos1], Addr: addr}, nil
}

func (na *NameAddr) GetAddress() *AddrSpec {
	return na.Addr
}

func (na *NameAddr) Write(writer io.Writer) (int, error) {
	n, _ := fmt.Fprintf(writer, na.DisplayName)
	m, _ := fmt.Fprintf(writer, "<")
	n += m
	m, _ = na.Addr.Write(writer)
	n += m
	m, err := fmt.Fprintf(writer, ">")
	n += m
	return n, err
}

func (na *NameAddr) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))
	na.Write(buf)
	return buf.String()
}
