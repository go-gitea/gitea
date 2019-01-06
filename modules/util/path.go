// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import "path/filepath"

// EnsureAbsolutePath ensure that a path is absolute, making it
// relative to absoluteBase if necessary
func EnsureAbsolutePath(path string, absoluteBase string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(absoluteBase, path)
}
