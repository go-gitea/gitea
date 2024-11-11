// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/badge"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

func GetWorkflowBadge(ctx *context.Context) {
	workflowFile := ctx.PathParam("workflow_name")
	branch := ctx.Req.URL.Query().Get("branch")
	tag := ctx.Req.URL.Query().Get("tag")
	useLatestTag := ctx.Req.URL.Query().Has("latest_tag")
	var ref string
	switch {
	case useLatestTag:
		tags, _, err := ctx.Repo.GitRepo.GetTagInfos(0, 1)
		if err != nil {
			ctx.ServerError("GetTagInfos", err)
			return
		}
		if len(tags) != 0 {
			tag = tags[0].Name
		}
		ref = fmt.Sprintf("refs/tags/%s", tag)
	case tag != "":
		ref = fmt.Sprintf("refs/tags/%s", tag)
	case branch != "":
		ref = fmt.Sprintf("refs/heads/%s", branch)
	default:
		branch = ctx.Repo.Repository.DefaultBranch
		ref = fmt.Sprintf("refs/heads/%s", branch)
	}
	event := ctx.Req.URL.Query().Get("event")

	badge, err := getWorkflowBadge(ctx, workflowFile, ref, event)
	if err != nil {
		ctx.ServerError("GetWorkflowBadge", err)
		return
	}

	ctx.Data["Badge"] = badge
	ctx.RespHeader().Set("Content-Type", "image/svg+xml")
	ctx.HTML(http.StatusOK, "shared/actions/runner_badge")
}

func getWorkflowBadge(ctx *context.Context, workflowFile, ref, event string) (badge.Badge, error) {
	extension := filepath.Ext(workflowFile)
	workflowName := strings.TrimSuffix(workflowFile, extension)

	run, err := actions_model.GetWorkflowLatestRun(ctx, ctx.Repo.Repository.ID, workflowFile, ref, event)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			return badge.GenerateBadge(workflowName, "no status", badge.DefaultColor), nil
		}
		return badge.Badge{}, err
	}

	color, ok := badge.StatusColorMap[run.Status]
	if !ok {
		return badge.GenerateBadge(workflowName, "unknown status", badge.DefaultColor), nil
	}
	return badge.GenerateBadge(workflowName, run.Status.String(), color), nil
}
