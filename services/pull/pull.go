// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// NewPullRequest creates new pull request with labels for repository.
func NewPullRequest(repo *models.Repository, pull *models.Issue, labelIDs []int64, uuids []string, pr *models.PullRequest, patch []byte, assigneeIDs []int64) error {
	if err := models.NewPullRequest(repo, pull, labelIDs, uuids, pr, patch, assigneeIDs); err != nil {
		return err
	}

	if err := models.NotifyWatchers(&models.Action{
		ActUserID: pull.Poster.ID,
		ActUser:   pull.Poster,
		OpType:    models.ActionCreatePullRequest,
		Content:   fmt.Sprintf("%d|%s", pull.Index, pull.Title),
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}

	pr.Issue = pull
	pull.PullRequest = pr
	mode, _ := models.AccessLevel(pull.Poster, repo)
	if err := models.PrepareWebhooks(repo, models.HookEventPullRequest, &api.PullRequestPayload{
		Action:      api.HookIssueOpened,
		Index:       pull.Index,
		PullRequest: pr.APIFormat(),
		Repository:  repo.APIFormat(mode),
		Sender:      pull.Poster.APIFormat(),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(repo.ID)
	}

	return nil
}
