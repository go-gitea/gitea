// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package auth

import (
	"github.com/quasoft/websspi"
)

type UserInfo = websspi.UserInfo

func sspiAuthInit() {
	var err error
	config := websspi.NewConfig()
	if sspiAuth, err = websspi.New(config); err != nil {
		panic(err) // this init is called by a sync.Once, maybe "panic" is the simplest way to handle errors
	}
}
