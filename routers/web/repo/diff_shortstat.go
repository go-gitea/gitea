// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"net/url"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/gitdiff"
)

const (
	tplDiffShortStatDetail templates.TplName = "repo/diff/shortstat_detail"
	tplDiffShortStatTab    templates.TplName = "repo/diff/shortstat_tab"
)

func buildDiffShortStatURL(baseURL, beforeCommitID, afterCommitID, target string) string {
	values := url.Values{}
	values.Set("before", beforeCommitID)
	values.Set("after", afterCommitID)
	values.Set("target", target)
	return baseURL + "?" + values.Encode()
}

func setDiffShortStatPlaceholderData(ctx *context.Context, numFiles int, detailURL, tabURL string) {
	ctx.Data["DiffShortStat"] = &gitdiff.DiffShortStat{NumFiles: numFiles}
	if detailURL != "" {
		ctx.Data["DiffShortStatDetailURL"] = detailURL
	}
	if tabURL != "" {
		ctx.Data["DiffShortStatTabURL"] = tabURL
	}
}

func setComputedDiffShortStatData(ctx *context.Context, repoStorage gitrepo.Repository, beforeCommitID, afterCommitID string) bool {
	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, repoStorage)
	if err != nil {
		ctx.ServerError("RepositoryFromContextOrOpen", err)
		return false
	}
	defer closer.Close()

	diffShortStat, err := gitdiff.GetDiffShortStat(ctx, repoStorage, gitRepo, beforeCommitID, afterCommitID)
	if err != nil {
		ctx.ServerError("GetDiffShortStat", err)
		return false
	}
	ctx.Data["DiffShortStat"] = diffShortStat
	return true
}

func renderDiffShortStat(ctx *context.Context, repoStorage gitrepo.Repository, beforeCommitID, afterCommitID, target string) {
	if afterCommitID == "" {
		ctx.HTTPError(http.StatusBadRequest, "missing after commit")
		return
	}

	var tpl templates.TplName
	switch target {
	case "detail":
		tpl = tplDiffShortStatDetail
	case "tab":
		tpl = tplDiffShortStatTab
	default:
		ctx.HTTPError(http.StatusBadRequest, "unknown diff shortstat target")
		return
	}

	if !setComputedDiffShortStatData(ctx, repoStorage, beforeCommitID, afterCommitID) {
		return
	}
	ctx.HTML(http.StatusOK, tpl)
}

func DiffShortStat(ctx *context.Context) {
	renderDiffShortStat(ctx, ctx.Repo.Repository, ctx.FormString("before"), ctx.FormString("after"), ctx.FormString("target"))
}

func WikiDiffShortStat(ctx *context.Context) {
	renderDiffShortStat(ctx, ctx.Repo.Repository.WikiStorageRepo(), ctx.FormString("before"), ctx.FormString("after"), ctx.FormString("target"))
}

func PullDiffShortStat(ctx *context.Context) {
	issue, ok := getPullInfoForDiffShortStat(ctx)
	if !ok {
		return
	}
	pull := issue.PullRequest

	mergeBaseCommitID := GetMergedBaseCommitID(ctx, issue)
	if mergeBaseCommitID == "" {
		ctx.HTTPError(http.StatusBadRequest, "missing merge base")
		return
	}

	headCommitID, err := ctx.Repo.GitRepo.GetRefCommitID(pull.GetGitHeadRefName())
	if err != nil {
		ctx.ServerError("GetRefCommitID", err)
		return
	}

	beforeCommitID := ctx.FormString("before")
	afterCommitID := ctx.FormString("after")
	if beforeCommitID == "" && afterCommitID == "" {
		beforeCommitID = mergeBaseCommitID
		afterCommitID = headCommitID
	}
	if beforeCommitID == "" || afterCommitID == "" {
		ctx.HTTPError(http.StatusBadRequest, "missing commit range")
		return
	}

	target := ctx.FormString("target")
	ok, err = isPullDiffShortStatRangeValid(ctx.Repo.GitRepo, mergeBaseCommitID, headCommitID, beforeCommitID, afterCommitID)
	if err != nil {
		ctx.ServerError("isPullDiffShortStatRangeValid", err)
		return
	}
	if !ok {
		ctx.HTTPError(http.StatusBadRequest, "invalid pull diff shortstat range")
		return
	}

	renderDiffShortStat(ctx, ctx.Repo.Repository, beforeCommitID, afterCommitID, target)
}

func getPullInfoForDiffShortStat(ctx *context.Context) (issue *issues_model.Issue, ok bool) {
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetIssueByIndex", err)
		}
		return nil, false
	}
	if !issue.IsPull {
		ctx.NotFound(nil)
		return nil, false
	}
	if err = issue.LoadPullRequest(ctx); err != nil {
		ctx.ServerError("LoadPullRequest", err)
		return nil, false
	}
	return issue, true
}

func isPullDiffShortStatRangeValid(gitRepo *git.Repository, mergeBaseCommitID, headCommitID, beforeCommitID, afterCommitID string) (bool, error) {
	beforeInRange, err := isCommitInPullDiffShortStatRange(gitRepo, mergeBaseCommitID, headCommitID, beforeCommitID)
	if err != nil || !beforeInRange {
		return beforeInRange, err
	}
	afterInRange, err := isCommitInPullDiffShortStatRange(gitRepo, mergeBaseCommitID, headCommitID, afterCommitID)
	if err != nil || !afterInRange {
		return afterInRange, err
	}

	return true, nil
}

func isCommitInPullDiffShortStatRange(gitRepo *git.Repository, mergeBaseCommitID, headCommitID, commitID string) (bool, error) {
	commit, err := gitRepo.GetCommit(commitID)
	if err != nil {
		return false, err
	}
	if commit.ID.String() == mergeBaseCommitID || commit.ID.String() == headCommitID {
		return true, nil
	}

	mergeBaseObjectID, err := git.NewIDFromString(mergeBaseCommitID)
	if err != nil {
		return false, err
	}
	hasMergeBase, err := commit.HasPreviousCommit(mergeBaseObjectID)
	if err != nil || !hasMergeBase {
		return hasMergeBase, err
	}

	headCommit, err := gitRepo.GetCommit(headCommitID)
	if err != nil {
		return false, err
	}
	return headCommit.HasPreviousCommit(commit.ID)
}
