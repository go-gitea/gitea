// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package badges

import (
	"fmt"
	"net/url"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	context_module "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

func getBadgeURL(ctx *context_module.Context, label, text, color string) string {
	sb := &strings.Builder{}
	_ = setting.Badges.GeneratorURLTemplateTemplate.Execute(sb, map[string]string{
		"label": url.PathEscape(label),
		"text":  url.PathEscape(text),
		"color": url.PathEscape(color),
	})

	badgeURL := sb.String()
	q := ctx.Req.URL.Query()
	// Remove any `branch` or `event` query parameters. They're used by the
	// workflow badge route, and do not need forwarding to the badge generator.
	delete(q, "branch")
	delete(q, "event")
	if len(q) > 0 {
		return fmt.Sprintf("%s?%s", badgeURL, q.Encode())
	}
	return badgeURL
}

func redirectToBadge(ctx *context_module.Context, label, text, color string) {
	ctx.Redirect(getBadgeURL(ctx, label, text, color))
}

func errorBadge(ctx *context_module.Context, label, text string) {
	ctx.Redirect(getBadgeURL(ctx, label, text, "crimson"))
}

func GetWorkflowBadge(ctx *context_module.Context) {
	branch := ctx.Req.URL.Query().Get("branch")
	if branch == "" {
		branch = ctx.Repo.Repository.DefaultBranch
	}
	branch = fmt.Sprintf("refs/heads/%s", branch)
	event := ctx.Req.URL.Query().Get("event")

	workflowFile := ctx.Params("workflow_name")
	run, err := actions_model.GetLatestRunForBranchAndWorkflow(ctx, ctx.Repo.Repository.ID, branch, workflowFile, event)
	if err != nil {
		errorBadge(ctx, workflowFile, "Not found")
		return
	}

	var color string
	switch run.Status {
	case actions_model.StatusUnknown:
		color = "lightgrey"
	case actions_model.StatusWaiting:
		color = "lightgrey"
	case actions_model.StatusRunning:
		color = "gold"
	case actions_model.StatusSuccess:
		color = "brightgreen"
	case actions_model.StatusFailure:
		color = "crimson"
	case actions_model.StatusCancelled:
		color = "orange"
	case actions_model.StatusSkipped:
		color = "blue"
	case actions_model.StatusBlocked:
		color = "yellow"
	default:
		color = "lightgrey"
	}

	redirectToBadge(ctx, workflowFile, run.Status.String(), color)
}

func getIssueOrPullBadge(ctx *context_module.Context, label, variant string, num int) {
	var text string
	if len(variant) > 0 {
		text = fmt.Sprintf("%d %s", num, variant)
	} else {
		text = fmt.Sprintf("%d", num)
	}
	redirectToBadge(ctx, label, text, "blue")
}

func getIssueBadge(ctx *context_module.Context, variant string, num int) {
	if !ctx.Repo.CanRead(unit.TypeIssues) &&
		!ctx.Repo.CanRead(unit.TypeExternalTracker) {
		errorBadge(ctx, "issues", "Not found")
		return
	}

	_, err := ctx.Repo.Repository.GetUnit(ctx, unit.TypeExternalTracker)
	if err == nil {
		errorBadge(ctx, "issues", "Not found")
		return
	}

	getIssueOrPullBadge(ctx, "issues", variant, num)
}

func getPullBadge(ctx *context_module.Context, variant string, num int) {
	if !ctx.Repo.Repository.CanEnablePulls() || !ctx.Repo.CanRead(unit.TypePullRequests) {
		errorBadge(ctx, "pulls", "Not found")
		return
	}

	getIssueOrPullBadge(ctx, "pulls", variant, num)
}

func GetOpenIssuesBadge(ctx *context_module.Context) {
	getIssueBadge(ctx, "open", ctx.Repo.Repository.NumOpenIssues)
}

func GetClosedIssuesBadge(ctx *context_module.Context) {
	getIssueBadge(ctx, "closed", ctx.Repo.Repository.NumClosedIssues)
}

func GetTotalIssuesBadge(ctx *context_module.Context) {
	getIssueBadge(ctx, "", ctx.Repo.Repository.NumIssues)
}

func GetOpenPullsBadge(ctx *context_module.Context) {
	getPullBadge(ctx, "open", ctx.Repo.Repository.NumOpenPulls)
}

func GetClosedPullsBadge(ctx *context_module.Context) {
	getPullBadge(ctx, "closed", ctx.Repo.Repository.NumClosedPulls)
}

func GetTotalPullsBadge(ctx *context_module.Context) {
	getPullBadge(ctx, "", ctx.Repo.Repository.NumPulls)
}

func GetStarsBadge(ctx *context_module.Context) {
	redirectToBadge(ctx, "stars", fmt.Sprintf("%d", ctx.Repo.Repository.NumStars), "blue")
}

func GetLatestReleaseBadge(ctx *context_module.Context) {
	release, err := repo_model.GetLatestReleaseByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			errorBadge(ctx, "release", "Not found")
			return
		}
		ctx.ServerError("GetLatestReleaseByRepoID", err)
	}

	if err := release.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	redirectToBadge(ctx, "release", release.TagName, "blue")
}
