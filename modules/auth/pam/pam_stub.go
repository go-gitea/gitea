// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !pam

package pam

import (
	"errors"
)

// Supported is false when built without PAM
var Supported = false

// Auth not supported lack of pam tag
func Auth(serviceName, userName, passwd string) (string, error) {
	// bypass the lint on callers: SA4023: this comparison is always true (staticcheck)
	if !Supported {
		return "", errors.New("PAM not supported")
	}
	return "", nil
}
