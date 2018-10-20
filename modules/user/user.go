// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"os"
	"os/user"
	"runtime"
	"strings"
)

// CurrentUsername return current login OS user name
func CurrentUsername() string {
	userinfo, err := user.Current()
	if err != nil {
		return fallbackCurrentUsername()
	}
	username := userinfo.Username
	if runtime.GOOS == "windows" {
		parts := strings.Split(username, "\\")
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
