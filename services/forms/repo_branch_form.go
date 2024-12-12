// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"net/http"

	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/context"

	"gitea.com/go-chi/binding"
)

// NewBranchForm form for creating a new branch
type NewBranchForm struct {
	NewBranchName    string `binding:"Required;MaxSize(100);GitRefName"`
	RepoID           int64
	SourceBranchName string
	CurrentPath      string
	CreateTag        bool
}

// Validate validates the fields
func (f *NewBranchForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// RenameBranchForm form for rename a branch
type RenameBranchForm struct {
	From string `binding:"Required;MaxSize(100);GitRefName"`
	To   string `binding:"Required;MaxSize(100);GitRefName"`
}

// Validate validates the fields
func (f *RenameBranchForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}
