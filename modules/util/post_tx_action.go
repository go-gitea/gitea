// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

// PostTxAction is a function that is executed after a database transaction
type PostTxAction func()

func NewPostTxAction() PostTxAction {
	return func() {}
}

func (f PostTxAction) Append(appendF PostTxAction) PostTxAction {
	return func() {
		f()
		appendF()
	}
}
