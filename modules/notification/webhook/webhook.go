// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	api "code.gitea.io/gitea/modules/structs"
)

type webhookNotifier struct {
	base.NullNotifier
}

var (
	_ base.Notifier = &webhookNotifier{}
)

// NewNotifier create a new webhookNotifier notifier
func NewNotifier() base.Notifier {
	return &webhookNotifier{}
}

func (m *webhookNotifier) NotifyIssueClearLabels(doer *models.User, issue *models.Issue) {
	if err := issue.LoadPoster(); err != nil {
		log.Error("loadPoster: %v", err)
		return
	}

	if err := issue.LoadRepo(); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}

	mode, _ := models.AccessLevel(issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(); err != nil {
			log.Error("LoadPullRequest: %v", err)
			return
		}

		err = models.PrepareWebhooks(issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
			Action:      api.HookIssueLabelCleared,
			Index:       issue.Index,
			PullRequest: issue.PullRequest.APIFormat(),
			Repository:  issue.Repo.APIFormat(mode),
			Sender:      doer.APIFormat(),
		})
	} else {
		err = models.PrepareWebhooks(issue.Repo, models.HookEventIssues, &api.IssuePayload{
			Action:     api.HookIssueLabelCleared,
			Index:      issue.Index,
			Issue:      issue.APIFormat(),
			Repository: issue.Repo.APIFormat(mode),
			Sender:     doer.APIFormat(),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	} else {
		go models.HookQueue.Add(issue.RepoID)
	}
}

func (m *webhookNotifier) NotifyForkRepository(doer *models.User, oldRepo, repo *models.Repository) {
	oldMode, _ := models.AccessLevel(doer, oldRepo)
	mode, _ := models.AccessLevel(doer, repo)

	// forked webhook
	if err := models.PrepareWebhooks(oldRepo, models.HookEventFork, &api.ForkPayload{
		Forkee: oldRepo.APIFormat(oldMode),
		Repo:   repo.APIFormat(mode),
		Sender: doer.APIFormat(),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", oldRepo.ID, err)
	} else {
		go models.HookQueue.Add(oldRepo.ID)
	}

	u := repo.MustOwner()

	// Add to hook queue for created repo after session commit.
	if u.IsOrganization() {
		if err := models.PrepareWebhooks(repo, models.HookEventRepository, &api.RepositoryPayload{
			Action:       api.HookRepoCreated,
			Repository:   repo.APIFormat(models.AccessModeOwner),
			Organization: u.APIFormat(),
			Sender:       doer.APIFormat(),
		}); err != nil {
			log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
		} else {
			go models.HookQueue.Add(repo.ID)
		}
	}
}

func (m *webhookNotifier) NotifyCreateRepository(doer *models.User, u *models.User, repo *models.Repository) {
	// Add to hook queue for created repo after session commit.
	if u.IsOrganization() {
		if err := models.PrepareWebhooks(repo, models.HookEventRepository, &api.RepositoryPayload{
			Action:       api.HookRepoCreated,
			Repository:   repo.APIFormat(models.AccessModeOwner),
			Organization: u.APIFormat(),
			Sender:       doer.APIFormat(),
		}); err != nil {
			log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
		} else {
			go models.HookQueue.Add(repo.ID)
		}
	}
}

func (m *webhookNotifier) NotifyDeleteRepository(doer *models.User, repo *models.Repository) {
	u := repo.MustOwner()

	if u.IsOrganization() {
		if err := models.PrepareWebhooks(repo, models.HookEventRepository, &api.RepositoryPayload{
			Action:       api.HookRepoDeleted,
			Repository:   repo.APIFormat(models.AccessModeOwner),
			Organization: u.APIFormat(),
			Sender:       doer.APIFormat(),
		}); err != nil {
			log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
		}
		go models.HookQueue.Add(repo.ID)
	}
}
