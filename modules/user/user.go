// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"os"
	"os/user"
	"strings"

	"gitea.dev/modules/consts"
)

// CurrentUsername return current login OS username
func CurrentUsername() string {
	userinfo, err := user.Current()
	if err != nil {
		return fallbackCurrentUsername()
	}
	username := userinfo.Username
	if consts.IsWindows {
		parts := strings.Split(username, "\\") // remove domain if present
		username = parts[len(parts)-1]
	}
	return username
}

// Old method, used if new method doesn't work on your OS for some reason
func fallbackCurrentUsername() string {
	curUserName := os.Getenv("USER")
	if len(curUserName) > 0 {
		return curUserName
	}

	return os.Getenv("USERNAME")
}
