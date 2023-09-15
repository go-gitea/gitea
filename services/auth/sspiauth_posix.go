// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package auth

import (
	"errors"
	"net/http"
)

type SSPIUserInfo struct {
	Username string   // Name of user, usually in the form DOMAIN\User
	Groups   []string // The global groups the user is a member of
}

type sspiAuthMock struct{}

func (s sspiAuthMock) AppendAuthenticateHeader(w http.ResponseWriter, data string) {
}

func (s sspiAuthMock) Authenticate(r *http.Request, w http.ResponseWriter) (userInfo *SSPIUserInfo, outToken string, err error) {
	return nil, "", errors.New("not implemented")
}

func sspiAuthInit() error {
	sspiAuth = &sspiAuthMock{} // TODO: we can mock the SSPI auth in tests
	return nil
}
