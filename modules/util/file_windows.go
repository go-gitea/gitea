// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package util

import (
	"os"
)

func ApplyUmask(f string, newMode os.FileMode) error {
	// do nothing for Windows, because Windows doesn't use umask
	return nil
}
