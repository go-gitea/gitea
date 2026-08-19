// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import "gitea.dev/modules/web/middleware"

// NewBranchForm form for creating a new branch
type NewBranchForm struct {
	middleware.FormDefaultValidator
	NewBranchName string `binding:"Required;MaxSize(100);GitRefName"`
	CurrentPath   string
	CreateTag     bool
}

// RenameBranchForm form for rename a branch
type RenameBranchForm struct {
	middleware.FormDefaultValidator
	From string `binding:"Required;MaxSize(100);GitRefName"`
	To   string `binding:"Required;MaxSize(100);GitRefName"`
}
