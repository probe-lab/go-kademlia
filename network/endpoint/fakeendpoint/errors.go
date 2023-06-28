package fakeendpoint

import "fmt"

var (
	ErrInvalidResponseType = fmt.Errorf("invalid response type, expected MinKadResponseMessage")
)
