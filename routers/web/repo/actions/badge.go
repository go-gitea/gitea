// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"fmt"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

func GetWorkflowBadge(ctx *context.Context) {
	workflowFile := ctx.PathParamRaw("*")
	if !strings.HasSuffix(workflowFile, "/badge.svg") {
		ctx.NotFound("Not found", fmt.Errorf("%s not a badge request", ctx.Req.URL.Path))
		return
	}

	workflowFile = strings.TrimSuffix(workflowFile, "/badge.svg")
	run, err := actions_model.GetRepoBranchLastRun(ctx, ctx.Repo.Repository.ID,
		git.RefNameFromBranch(ctx.Repo.Repository.DefaultBranch).String(),
		workflowFile)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.NotFound("Not found", fmt.Errorf("%s not found", workflowFile))
			return
		}
		ctx.ServerError("GetWorkflowBadge", err)
		return
	}

	switch run.Status {
	case actions_model.StatusSuccess, actions_model.StatusSkipped:
		ctx.Redirect(setting.AbsoluteAssetURL + "/assets/img/svg/gitea_actions_pass.svg")
	case actions_model.StatusUnknown, actions_model.StatusFailure, actions_model.StatusCancelled:
		ctx.Redirect(setting.AbsoluteAssetURL + "/assets/img/svg/gitea_actions_failed.svg")
	case actions_model.StatusWaiting, actions_model.StatusRunning, actions_model.StatusBlocked:
		ctx.Redirect(setting.AbsoluteAssetURL + "/assets/img/svg/gitea_actions_pending.svg")
	default:
		ctx.NotFound("Not found", fmt.Errorf("unknown status %d", run.Status))
	}
}
