// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
)

// GetBranchData set branch information
func GetBranchData(ctx *context.Context, issue *issues_model.Issue) {
	ctx.Data["BaseBranch"] = nil
	ctx.Data["HeadBranch"] = nil
	ctx.Data["HeadUserName"] = nil
	ctx.Data["BaseName"] = ctx.Repo.Repository.OwnerName
	if issue.IsPull {
		pull := issue.PullRequest
		ctx.Data["BaseBranch"] = pull.BaseBranch
		ctx.Data["HeadBranch"] = pull.HeadBranch
		ctx.Data["HeadUserName"] = pull.MustHeadUserName()
	}
}
