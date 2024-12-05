// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sender

import (
	"fmt"

	"github.com/Azure/go-ntlmssp"
	"github.com/wneessen/go-mail/smtp"
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
	username, password, domain string
	domainNeeded               bool
}

// NtlmAuth SMTP AUTH NTLM Auth Handler
func NtlmAuth(username, password string) smtp.Auth {
	user, domain, domainNeeded := ntlmssp.GetDomain(username)
	return &ntlmAuth{user, password, domain, domainNeeded}
}

// Start starts SMTP NTLM Auth
func (a *ntlmAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	negotiateMessage, err := ntlmssp.NewNegotiateMessage(a.domain, "")
	return "NTLM", negotiateMessage, err
}

// Next next step of SMTP ntlm auth
func (a *ntlmAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		if len(fromServer) == 0 {
			return nil, fmt.Errorf("ntlm ChallengeMessage is empty")
		}
		authenticateMessage, err := ntlmssp.ProcessChallenge(fromServer, a.username, a.password, a.domainNeeded)
		return authenticateMessage, err
	}
	return nil, nil
}
