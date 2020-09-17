package main

import (
	"fmt"
)

type ClientTransactionMgr struct {
}

type ClientTransaction struct {
	// it is the client generated branch parameter in Via header
	TransId string
	// method in CSeq
	Method string
}

type ServerTransaction struct {
	TransId string
	SentBy  string
	Method  string
}

func NewClientTransactionMgr() *ClientTransactionMgr {
	return &ClientTransactionMgr{}
}
func NewClientTransaction(branch string, method string) *ClientTransaction {
	return &ClientTransaction{TransId: branch, Method: method}
}

func (ct *ClientTransaction) String() string {
	return fmt.Sprintf("%s-%s", ct.Method, ct.TransId)
}
