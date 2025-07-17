// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

type CleanUpFunc func()

func NewCleanUpFunc() CleanUpFunc {
	return func() {}
}

func (f CleanUpFunc) Append(newF CleanUpFunc) CleanUpFunc {
	return func() {
		f()
		newF()
	}
}
