package main

import ()

type TransactionMgr struct {
}

type ClientTransaction struct {
	TransId string
	Method  string
}

type ServerTransaction struct {
	TransId string
	SentBy  string
	Method  string
}
