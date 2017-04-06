// +build !windows,!go1.7 go1.7

// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import "os"

// RemoveAll files from Go version 1.7 onward
func RemoveAll(path string) error {
	return os.RemoveAll(path)
}
