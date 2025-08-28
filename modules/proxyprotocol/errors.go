// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package proxyprotocol

import "fmt"

// ErrBadHeader is an error demonstrating a bad proxy header
type ErrBadHeader struct {
	Header []byte
}

func (e *ErrBadHeader) Error() string {
	return fmt.Sprintf("Unexpected proxy header: %v", e.Header)
}

// ErrBadAddressType is an error demonstrating a bad proxy header with bad Address type
type ErrBadAddressType struct {
	Address string
}

func (e *ErrBadAddressType) Error() string {
	return "Unexpected proxy header address type: " + e.Address
}

// ErrBadRemote is an error demonstrating a bad proxy header with bad Remote
type ErrBadRemote struct {
	IP   string
	Port string
}

func (e *ErrBadRemote) Error() string {
	return fmt.Sprintf("Unexpected proxy header remote IP and port: %s %s", e.IP, e.Port)
}

// ErrBadLocal is an error demonstrating a bad proxy header with bad Local
type ErrBadLocal struct {
	IP   string
	Port string
}

func (e *ErrBadLocal) Error() string {
	return fmt.Sprintf("Unexpected proxy header local IP and port: %s %s", e.IP, e.Port)
}
