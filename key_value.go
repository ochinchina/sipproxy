package main

import (
	"bytes"
	"fmt"
	"io"
)

type KeyValue struct {
	Key   string
	Value string
}

func (kv KeyValue) Write(writer io.Writer) (int, error) {
	n, err := fmt.Fprintf(writer, kv.Key)
	if len(kv.Value) > 0 {
		m, _ := fmt.Fprintf(writer, "=")
		n += m
		m, err = fmt.Fprintf(writer, kv.Value)
		n += m
	}
	return n, err
}
func (kv KeyValue) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))
	kv.Write(buf)
	return buf.String()
}
