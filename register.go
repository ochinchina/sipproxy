package main

import (
	"bytes"
)

type Register struct {
	transport ClientTransport
	//register server address in domain:port or ip:port
	registerServer string
	// my address in domain:port or ip:port
	from *NameAddr
}

func NewRegister(transport ClientTransport,
	registerServer string,
	from *NameAddr) *Register {
	return &Register{transport: transport, registerServer: registerServer, from: from}
}

func (r *Register) Start() {
	message, err := NewRequest("REGISTER", "sip:"+r.registerServer, "2.0")
	if err == nil {
		buf := bytes.NewBuffer(make([]byte, 0))
		message.Write(buf)
	}
}
