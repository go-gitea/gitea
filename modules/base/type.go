// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

// IsError checks if an error is a T error.
func IsError[T any](err error) bool {
	_, ok := err.(T)
	return ok
}
