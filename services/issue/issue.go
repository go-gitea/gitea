// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// NewIssue creates new issue with labels for repository.
func NewIssue(repo *models.Repository, issue *models.Issue, labelIDs []int64, assigneeIDs []int64, uuids []string) error {
	if err := models.NewIssue(repo, issue, labelIDs, assigneeIDs, uuids); err != nil {
		return err
	}

	if err := models.NotifyWatchers(&models.Action{
		ActUserID: issue.Poster.ID,
		ActUser:   issue.Poster,
		OpType:    models.ActionCreateIssue,
		Content:   fmt.Sprintf("%d|%s", issue.Index, issue.Title),
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}

	mode, _ := models.AccessLevel(issue.Poster, issue.Repo)
	if err := models.PrepareWebhooks(repo, models.HookEventIssues, &api.IssuePayload{
		Action:     api.HookIssueOpened,
		Index:      issue.Index,
		Issue:      issue.APIFormat(),
		Repository: repo.APIFormat(mode),
		Sender:     issue.Poster.APIFormat(),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(issue.RepoID)
	}

	return nil
}

// ChangeTitle changes the title of this issue, as the given user.
func ChangeTitle(issue *models.Issue, doer *models.User, title string) (err error) {
	oldTitle := issue.Title
	issue.Title = title

	if err = issue.ChangeTitle(doer, oldTitle); err != nil {
		return
	}

	mode, _ := models.AccessLevel(issue.Poster, issue.Repo)
	if issue.IsPull {
		if err = issue.LoadPullRequest(); err != nil {
			return fmt.Errorf("loadPullRequest: %v", err)
		}
		issue.PullRequest.Issue = issue
		err = models.PrepareWebhooks(issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Title: &api.ChangesFromPayload{
					From: oldTitle,
				},
			},
			PullRequest: issue.PullRequest.APIFormat(),
			Repository:  issue.Repo.APIFormat(mode),
			Sender:      doer.APIFormat(),
		})
	} else {
		err = models.PrepareWebhooks(issue.Repo, models.HookEventIssues, &api.IssuePayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Title: &api.ChangesFromPayload{
					From: oldTitle,
				},
			},
			Issue:      issue.APIFormat(),
			Repository: issue.Repo.APIFormat(mode),
			Sender:     issue.Poster.APIFormat(),
		})
	}

	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	} else {
		go models.HookQueue.Add(issue.RepoID)
	}

	return nil
}
