// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package auth

import (
	"github.com/quasoft/websspi"
)

type SSPIUserInfo = websspi.UserInfo

func sspiAuthInit() error {
	var err error
	config := websspi.NewConfig()
	sspiAuth, err = websspi.New(config)
	return err
}
