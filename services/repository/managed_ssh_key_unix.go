// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package repository

func restrictToCurrentUser(string) error {
	return nil // temp dirs are created with mode 0700
}
