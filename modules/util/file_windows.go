// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build windows

package util

import (
	"os"
)

func ApplyUmask(f string, newMode os.FileMode) error {
	// do nothing for Windows, because Windows doesn't use umask
	return nil
}
