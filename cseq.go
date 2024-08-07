package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type CSeq struct {
	Seq    int
	Method string
}

func ParseCSeq(s string) (*CSeq, error) {
	fields := strings.Fields(s)
	if len(fields) == 2 {
		seq, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, err
		}
		return &CSeq{Seq: seq, Method: fields[1]}, nil
	}
	return nil, errors.New("malformatted CSeq header")
}

func (cs *CSeq) Write(writer io.Writer) (int, error) {
	return fmt.Fprintf(writer, "%d %s", cs.Seq, cs.Method)
}

func (cs *CSeq) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))
	cs.Write(buf)
	return buf.String()
}
