// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build !windows

package util

import (
	"os"

	"golang.org/x/sys/unix"
)

var defaultUmask int

func init() {
	// at the moment, the umask could only be gotten by calling unix.Umask(newUmask)
	// use 0o077 as temp new umask to reduce the risks if this umask is used anywhere else before the correct umask is recovered
	tempUmask := 0o077
	defaultUmask = unix.Umask(tempUmask)
	unix.Umask(defaultUmask)
}

func ApplyUmask(f string, newMode os.FileMode) error {
	mod := newMode & ^os.FileMode(defaultUmask)
	return os.Chmod(f, mod)
}
