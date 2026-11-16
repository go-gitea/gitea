// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"gitea.dev/modules/optional"
	"gitea.dev/modules/web/middleware"
)

type CommitCommonForm struct {
	middleware.FormDefaultValidator
	TreePath      string `binding:"MaxSize(500)"`
	CommitSummary string `binding:"MaxSize(100)"`
	CommitMessage string
	CommitChoice  string `binding:"Required;MaxSize(50)"`
	NewBranchName string `binding:"GitRefName;MaxSize(100)"`
	LastCommit    string
	Signoff       bool
	CommitEmail   string
}

type CommitCommonFormInterface interface {
	GetCommitCommonForm() *CommitCommonForm
}

func (f *CommitCommonForm) GetCommitCommonForm() *CommitCommonForm {
	return f
}

type EditRepoFileForm struct {
	middleware.FormDefaultValidator
	CommitCommonForm
	Content optional.Option[string]
}

type DeleteRepoFileForm struct {
	middleware.FormDefaultValidator
	CommitCommonForm
}

type UploadRepoFileForm struct {
	middleware.FormDefaultValidator
	CommitCommonForm
	Files []string
}

type CherryPickForm struct {
	middleware.FormDefaultValidator
	CommitCommonForm
	Revert bool
}
