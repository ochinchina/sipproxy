package main

import (
	"errors"
	"github.com/google/uuid"
	"net"
	"strings"
)

func CreateBranch() (string, error) {
	uuid, err := uuid.NewRandom()
	if err == nil {
		tmp := strings.Split(uuid.String(), "-")
		return "z9hG4bK" + tmp[len(tmp)-1], nil
	}
	return "", err
}

func CreateTag() (string, error) {
	uuid, err := uuid.NewRandom()
	if err == nil {
		tmp := strings.Split(uuid.String(), "-")
		return tmp[len(tmp)-1], nil
	}
	return "", err
}

func inStrArray(s string, a []string) bool {
	for _, t := range a {
		if t == s {
			return true
		}
	}
	return false
}

func strArraySub(a1 []string, a2 []string) []string {
	r := make([]string, 0)
	for _, s := range a1 {
		if !inStrArray(s, a2) {
			r = append(r, s)
		}
	}
	return r
}

func splitAddr(addr string) (hostname string, port string, err error) {
	pos := strings.LastIndex(addr, ":")
	if pos == -1 {
		err = errors.New("not a valid address")
	} else {
		hostname = addr[0:pos]
		port = addr[pos+1:]
		err = nil
	}
	return

}

func isIPAddress(addr string) bool {
	if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") {
		addr = addr[1 : len(addr)-1]
	}
	return net.ParseIP(addr) != nil
}

func isIPv6( ip string) bool {
	return strings.Index( ip, ":" ) != -1
}
