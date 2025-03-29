// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/badge"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

func GetWorkflowBadge(ctx *context.Context) {
	workflowFile := ctx.PathParam("workflow_name")
	branch := ctx.Req.URL.Query().Get("branch")
	if branch == "" {
		branch = ctx.Repo.Repository.DefaultBranch
	}
	branchRef := fmt.Sprintf("refs/heads/%s", branch)
	event := ctx.Req.URL.Query().Get("event")

	style := ctx.Req.URL.Query().Get("style")
	if !slices.Contains([]badge.Style{badge.StyleFlat, badge.StyleFlatSquare}, badge.Style(style)) {
		style = badge.DefaultStyle
	}

	b, err := getWorkflowBadge(ctx, workflowFile, branchRef, event)
	if err != nil {
		ctx.ServerError("GetWorkflowBadge", err)
		return
	}

	ctx.Data["Badge"] = b
	ctx.RespHeader().Set("Content-Type", "image/svg+xml")
	switch ctx.Req.URL.Query().Get("style") {
	case badge.StyleFlat:
		ctx.HTML(http.StatusOK, "shared/actions/runner_badge_flat")
	case badge.StyleFlatSquare:
		ctx.HTML(http.StatusOK, "shared/actions/runner_badge_flat-square")
	}
}

func getWorkflowBadge(ctx *context.Context, workflowFile, branchName, event string) (badge.Badge, error) {
	extension := filepath.Ext(workflowFile)
	workflowName := strings.TrimSuffix(workflowFile, extension)

	run, err := actions_model.GetWorkflowLatestRun(ctx, ctx.Repo.Repository.ID, workflowFile, branchName, event)
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
