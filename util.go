package main

import (
	"github.com/google/uuid"
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
