package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"strconv"
)

type CSeq struct {
	Seq int
	Method string
}


func ParseCSeq( s string )(*CSeq, error ) {
	fields := strings.Fields( s )
	if len( fields ) == 2 {
		seq, err := strconv.Atoi( fields[0] )
		if err != nil {
			return nil, err
		}
		return &CSeq{ Seq: seq, Method: fields[1] }, nil
	}
	return nil, errors.New( "Malformatted CSeq header" )
}

func (cs *CSeq) Write( writer io.Writer ) (int, error ) {
	n, _ := fmt.Fprintf( writer, "%d", cs.Seq )
	t, err := fmt.Fprintf( writer, "%s", cs.Method )
	if err != nil {
		return 0, err
	}
	return n + t, nil
}


func (cs *CSeq)String() string {
	buf := bytes.NewBuffer( make([]byte, 0 ) )
	cs.Write( buf )
	return buf.String()
}
