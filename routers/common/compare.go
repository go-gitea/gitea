// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	pull_service "code.gitea.io/gitea/services/pull"
)

// CompareInfo represents the collected results from ParseCompareInfo
type CompareInfo struct {
	HeadUser         *user_model.User
	HeadRepo         *repo_model.Repository
	HeadGitRepo      *git.Repository
	CompareInfo      *pull_service.CompareInfo
	BaseBranch       string
	HeadBranch       string
	DirectComparison bool
}
