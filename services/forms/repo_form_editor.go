// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"net/http"

	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/context"

	"gitea.com/go-chi/binding"
)

type CommitCommonForm struct {
	TreePath      string `binding:"MaxSize(500)"`
	CommitSummary string `binding:"MaxSize(100)"`
	CommitMessage string
	CommitChoice  string `binding:"Required;MaxSize(50)"`
	NewBranchName string `binding:"GitRefName;MaxSize(100)"`
	LastCommit    string
	Signoff       bool
	CommitEmail   string
}

func (f *CommitCommonForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

type CommitCommonFormInterface interface {
	GetCommitCommonForm() *CommitCommonForm
}

func (f *CommitCommonForm) GetCommitCommonForm() *CommitCommonForm {
	return f
}

type EditRepoFileForm struct {
	CommitCommonForm
	Content optional.Option[string]
}

type DeleteRepoFileForm struct {
	CommitCommonForm
}

type UploadRepoFileForm struct {
	CommitCommonForm
	Files []string
}

type CherryPickForm struct {
	CommitCommonForm
	Revert bool
}
