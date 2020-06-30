package main

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type ViaParam struct {
	ProtocolName    string
	ProtocolVersion string
	Transport       string
	Host            string
	port            int
	Params          []KeyValue
}

type Via struct {
	params []*ViaParam
}

func NewVia() *Via {
	return &Via{params: make([]*ViaParam, 0)}
}

func (v *Via) AddViaParam(viaParam *ViaParam) {
	v.params = append(v.params, viaParam)
}

func (v *Via) PopViaParam() (*ViaParam, error) {
	if len(v.params) <= 0 {
		return nil, errors.New("No via-param")
	}
	r := v.params[0]
	v.params = v.params[1:]
	return r, nil
}

func (v *Via) Size() int {
	return len(v.params)
}

func (v *Via) GetParam(index int) (*ViaParam, error) {
	if index < 0 || index >= len(v.params) {
		return nil, errors.New("index is outof bound")
	}
	return v.params[index], nil
}

func (v *Via) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))
	for index, param := range v.params {
		if index != 0 {
			fmt.Fprintf(buf, ",")
		}
		fmt.Fprintf(buf, "%s", param.String())
	}
	return buf.String()
}

func (vp *ViaParam) SetParam(name string, value string) {
	for _, param := range vp.Params {
		if param.Key == name {
			param.Value = value
			return
		}
	}
	vp.Params = append(vp.Params, KeyValue{Key: name, Value: value})
}

func (vp *ViaParam) GetParam(name string) (string, error) {
	for _, param := range vp.Params {
		if param.Key == name {
			return param.Value, nil
		}
	}

	return "", fmt.Errorf("No such param %s", name)
}

func (vp *ViaParam) HasParam(name string) bool {
	for _, param := range vp.Params {
		if param.Key == name {
			return true
		}
	}

	return false
}
func (vp *ViaParam) GetBranch() (string, error) {
	return vp.GetParam("branch")
}

func (vp *ViaParam) SetBranch(branch string) {
	vp.SetParam("branch", branch)
}
func (vp *ViaParam) GetPort() int {
	if vp.port != 0 {
		return vp.port
	}
	if vp.Transport == "TLS" {
		return 5061
	}
	return 5060
}

func (vp *ViaParam) GetReceived() (string, error) {
	return vp.GetParam("received")
}

func (vp *ViaParam) SetReceived(received string) {
	vp.SetParam("received", received)
}

func (vp *ViaParam) GetRPort() (int, error) {
	rport, err := vp.GetParam("rport")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(rport)
}

func (vp *ViaParam) GetTTL() (int, error) {
	ttl, err := vp.GetParam("ttl")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(ttl)
}

func (vp *ViaParam) String() string {
	buf := bytes.NewBuffer(make([]byte, 0))

	fmt.Fprintf(buf, "%s/%s/%s %s:%d", vp.ProtocolName, vp.ProtocolVersion, vp.Transport, vp.Host, vp.GetPort())
	for _, param := range vp.Params {
		fmt.Fprintf(buf, ";%s", param.String())
	}
	return buf.String()
}

func ParseVia(via string) (*Via, error) {
	result := &Via{}

	for _, param := range strings.Split(via, ",") {
		viaParam, err := parseViaParam(param)
		if err != nil {
			return nil, err
		}
		result.params = append(result.params, viaParam)
	}
	return result, nil
}

func CreateViaParam(transport string, host string, port int) *ViaParam {
	return &ViaParam{ProtocolName: "SIP",
		ProtocolVersion: "2.0",
		Transport:       transport,
		Host:            host,
		port:            port,
		Params:          make([]KeyValue, 0)}
}

func parseViaParam(viaParam string) (*ViaParam, error) {
	t := strings.Split(viaParam, ";")
	sentInfo := strings.Fields(t[0])

	if len(sentInfo) != 2 {
		return nil, errors.New("Malformatted Via header")
	}
	sentProtocol := strings.Split(sentInfo[0], "/")
	if len(sentProtocol) != 3 {
		return nil, errors.New("Malformatted sent-protocol")
	}
	sentBy := strings.Split(sentInfo[1], ":")

	if len(sentBy) > 2 {
		return nil, errors.New("Malformatted sent-by")
	}

	via := &ViaParam{ProtocolName: sentProtocol[0], ProtocolVersion: sentProtocol[1], Transport: sentProtocol[2], Host: sentBy[0]}
	if len(sentBy) == 2 {
		port, err := strconv.Atoi(sentBy[1])
		if err != nil {
			return nil, err
		}
		via.port = port
	} else {
		via.port = 0
	}

	for i, param := range t {
		if i != 0 {
			pos := strings.IndexByte(param, '=')
			if pos == -1 {
				return nil, errors.New("Malformatted via params")
			}
			name := param[0:pos]
			value := param[pos+1:]
			via.Params = append(via.Params, KeyValue{Key: name, Value: value})
		}
	}
	return via, nil

}
