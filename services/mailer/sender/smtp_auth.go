// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sender

import (
	"errors"
	"fmt"
	"net/smtp"

	"github.com/Azure/go-ntlmssp"
)

type loginAuth struct {
	username, password string
}

// LoginAuth SMTP AUTH LOGIN Auth Handler
func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

// Start start SMTP login auth
func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

// Next next step of SMTP login auth
func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, fmt.Errorf("unknown fromServer: %s", string(fromServer))
		}
	}
	return nil, nil
}

type ntlmAuth struct {
	username, password string
}

// NtlmAuth SMTP AUTH NTLM Auth Handler
func NtlmAuth(username, password string) smtp.Auth {
	return &ntlmAuth{username, password}
}

// Start starts SMTP NTLM Auth
func (a *ntlmAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	negotiateMessage, err := ntlmssp.NewNegotiateMessage("", "")
	return "NTLM", negotiateMessage, err
}

// Next next step of SMTP ntlm auth
func (a *ntlmAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		if len(fromServer) == 0 {
			return nil, errors.New("ntlm ChallengeMessage is empty")
		}
		authenticateMessage, err := ntlmssp.NewAuthenticateMessage(fromServer, a.username, a.password, nil)
		return authenticateMessage, err
	}
	return nil, nil
}
