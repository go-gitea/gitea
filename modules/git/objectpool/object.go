// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package objectpool

type Object struct {
	ID   string
	Type string
	Size int64
}
