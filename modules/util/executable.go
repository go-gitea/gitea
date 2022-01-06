// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"os"
	"path/filepath"
	"time"
)

// GetExecutableModTime get executable file modified time of current process.
func GetExecutableModTime() (time.Time, error) {
	exePath, err := os.Executable()
	if err != nil {
		return time.Time{}, err
	}

	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return time.Time{}, err
	}

	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return time.Time{}, err
	}

	st, err := os.Stat(exePath)
	if err != nil {
		return time.Time{}, err
	}

	return st.ModTime(), nil
}
