// Copyright 2017 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/sdk/gitea"
)

// NewCommitStatus creates a new CommitStatus
func NewCommitStatus(ctx *context.APIContext, form api.CreateStatusOption) {
	sha := ctx.Params("sha")
	if len(sha) == 0 {
		sha = ctx.Params("ref")
	}
	if len(sha) == 0 {
		ctx.Error(400, "ref/sha not given", nil)
		return
	}
	status := &models.CommitStatus{
		State:       models.CommitStatusState(form.State),
		TargetURL:   form.TargetURL,
		Description: form.Description,
		Context:     form.Context,
	}
	if err := models.NewCommitStatus(ctx.Repo.Repository, ctx.User, sha, status); err != nil {
		ctx.Error(500, "NewCommitStatus", err)
		return
	}

	newStatus, err := models.GetCommitStatus(ctx.Repo.Repository, sha, status)
	if err != nil {
		ctx.Error(500, "GetCommitStatus", err)
		return
	}
	ctx.JSON(201, newStatus.APIFormat())
}

// GetCommitStatuses returns all statuses for any given commit hash
func GetCommitStatuses(ctx *context.APIContext) {
	sha := ctx.Params("sha")
	if len(sha) == 0 {
		sha = ctx.Params("ref")
	}
	if len(sha) == 0 {
		ctx.Error(400, "ref/sha not given", nil)
		return
	}
	repo := ctx.Repo.Repository

	page := ctx.ParamsInt("page")

	statuses, err := models.GetCommitStatuses(repo, sha, page)
	if err != nil {
		ctx.Error(500, "GetCommitStatuses", fmt.Errorf("GetCommitStatuses[%s, %s, %d]: %v", repo.FullName(), sha, page, err))
	}

	apiStatuses := make([]*api.Status, 0, len(statuses))
	for _, status := range statuses {
		apiStatuses = append(apiStatuses, status.APIFormat())
	}

	ctx.JSON(200, apiStatuses)
}

type combinedCommitStatus struct {
	State      models.CommitStatusState `json:"state"`
	SHA        string                   `json:"sha"`
	TotalCount int                      `json:"total_count"`
	Statuses   []*api.Status            `json:"statuses"`
	Repo       *api.Repository          `json:"repository"`
	CommitURL  string                   `json:"commit_url"`
	URL        string                   `json:"url"`
}

// GetCombinedCommitStatus returns the combined status for any given commit hash
func GetCombinedCommitStatus(ctx *context.APIContext) {
	sha := ctx.Params("sha")
	if len(sha) == 0 {
		sha = ctx.Params("ref")
	}
	if len(sha) == 0 {
		ctx.Error(400, "ref/sha not given", nil)
		return
	}
	repo := ctx.Repo.Repository

	page := ctx.ParamsInt("page")

	statuses, err := models.GetLatestCommitStatus(repo, sha, page)
	if err != nil {
		ctx.Error(500, "GetLatestCommitStatus", fmt.Errorf("GetLatestCommitStatus[%s, %s, %d]: %v", repo.FullName(), sha, page, err))
		return
	}

	if len(statuses) == 0 {
		ctx.Status(200)
		return
	}

	acl, err := models.AccessLevel(ctx.User.ID, repo)
	if err != nil {
		ctx.Error(500, "AccessLevel", fmt.Errorf("AccessLevel[%d, %s]: %v", ctx.User.ID, repo.FullName(), err))
		return
	}
	retStatus := &combinedCommitStatus{
		SHA:        sha,
		TotalCount: len(statuses),
		Repo:       repo.APIFormat(acl),
		URL:        "",
	}

	retStatus.Statuses = make([]*api.Status, 0, len(statuses))
	for _, status := range statuses {
		retStatus.Statuses = append(retStatus.Statuses, status.APIFormat())
		if status.State.IsWorseThan(retStatus.State) {
			retStatus.State = status.State
		}
	}

	ctx.JSON(200, retStatus)
}
