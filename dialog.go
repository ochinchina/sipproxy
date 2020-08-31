package main

import (
	"fmt"
)

type Dialog struct {
	callID    string
	localTag  string
	remoteTag string
}

func NewDialog(callID string, localTag string, remoteTag string) *Dialog {
	return &Dialog{callID: callID, localTag: localTag, remoteTag: remoteTag}
}

func (d *Dialog) String() string {
	return fmt.Sprintf("%s-%s-%s", d.callID, d.localTag, d.remoteTag)
}
