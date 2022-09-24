// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build windows

package util

func ApplyUmask(f string) error {
	// do nothing for Windows
	return nil
}
