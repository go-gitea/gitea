// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/webhook"
)

func sendLabelUpdatedWebhook(issue *models.Issue, doer *models.User) {
	var err error

	if err = issue.LoadRepo(); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}

	if err = issue.LoadPoster(); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}

	mode, _ := models.AccessLevel(issue.Poster, issue.Repo)
	if issue.IsPull {
		if err = issue.LoadPullRequest(); err != nil {
			log.Error("loadPullRequest: %v", err)
			return
		}
		if err = issue.PullRequest.LoadIssue(); err != nil {
			log.Error("LoadIssue: %v", err)
			return
		}
		err = webhook.PrepareWebhooks(issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
			Action:      api.HookIssueLabelUpdated,
			Index:       issue.Index,
			PullRequest: issue.PullRequest.APIFormat(),
			Repository:  issue.Repo.APIFormat(models.AccessModeNone),
			Sender:      doer.APIFormat(),
		})
	} else {
		err = webhook.PrepareWebhooks(issue.Repo, models.HookEventIssues, &api.IssuePayload{
			Action:     api.HookIssueLabelUpdated,
			Index:      issue.Index,
			Issue:      issue.APIFormat(),
			Repository: issue.Repo.APIFormat(mode),
			Sender:     doer.APIFormat(),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	} else {
		go webhook.HookQueue.Add(issue.RepoID)
	}
}

// ClearLabels clears all of an issue's labels
func ClearLabels(issue *models.Issue, doer *models.User) (err error) {
	if err = issue.ClearLabels(doer); err != nil {
		return
	}

	notification.NotifyIssueClearLabels(doer, issue)

	return nil
}

// AddLabel adds a new label to the issue.
func AddLabel(issue *models.Issue, doer *models.User, label *models.Label) error {
	if err := models.NewIssueLabel(issue, label, doer); err != nil {
		return err
	}

	sendLabelUpdatedWebhook(issue, doer)
	return nil
}

// AddLabels adds a list of new labels to the issue.
func AddLabels(issue *models.Issue, doer *models.User, labels []*models.Label) error {
	if err := models.NewIssueLabels(issue, labels, doer); err != nil {
		return err
	}

	sendLabelUpdatedWebhook(issue, doer)
	return nil
}

// RemoveLabel removes a label from issue by given ID.
func RemoveLabel(issue *models.Issue, doer *models.User, label *models.Label) error {
	if err := issue.LoadRepo(); err != nil {
		return err
	}

	perm, err := models.GetUserRepoPermission(issue.Repo, doer)
	if err != nil {
		return err
	}
	if !perm.CanWriteIssuesOrPulls(issue.IsPull) {
		return models.ErrLabelNotExist{}
	}

	if err := models.DeleteIssueLabel(issue, label, doer); err != nil {
		return err
	}

	sendLabelUpdatedWebhook(issue, doer)
	return nil
}
