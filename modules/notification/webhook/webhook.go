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

func (m *webhookNotifier) NotifyIssueChangeAssignee(doer *models.User, issue *models.Issue, assignee *models.User, removed bool, comment *models.Comment) {
	if issue.IsPull {
		mode, _ := models.AccessLevelUnit(doer, issue.Repo, models.UnitTypePullRequests)

		if err := issue.LoadPullRequest(); err != nil {
			log.Error("LoadPullRequest failed: %v", err)
			return
		}
		issue.PullRequest.Issue = issue
		apiPullRequest := &api.PullRequestPayload{
			Index:       issue.Index,
			PullRequest: issue.PullRequest.APIFormat(),
			Repository:  issue.Repo.APIFormat(mode),
			Sender:      doer.APIFormat(),
		}
		if removed {
			apiPullRequest.Action = api.HookIssueUnassigned
		} else {
			apiPullRequest.Action = api.HookIssueAssigned
		}
		// Assignee comment triggers a webhook
		if err := models.PrepareWebhooks(issue.Repo, models.HookEventPullRequest, apiPullRequest); err != nil {
			log.Error("PrepareWebhooks [is_pull: %v, remove_assignee: %v]: %v", issue.IsPull, removed, err)
			return
		}
	} else {
		mode, _ := models.AccessLevelUnit(doer, issue.Repo, models.UnitTypeIssues)
		apiIssue := &api.IssuePayload{
			Index:      issue.Index,
			Issue:      issue.APIFormat(),
			Repository: issue.Repo.APIFormat(mode),
			Sender:     doer.APIFormat(),
		}
		if removed {
			apiIssue.Action = api.HookIssueUnassigned
		} else {
			apiIssue.Action = api.HookIssueAssigned
		}
		// Assignee comment triggers a webhook
		if err := models.PrepareWebhooks(issue.Repo, models.HookEventIssues, apiIssue); err != nil {
			log.Error("PrepareWebhooks [is_pull: %v, remove_assignee: %v]: %v", issue.IsPull, removed, err)
			return
		}
	}

	go models.HookQueue.Add(issue.RepoID)
}

func (m *webhookNotifier) NotifyIssueChangeTitle(doer *models.User, issue *models.Issue, oldTitle string) {
	mode, _ := models.AccessLevel(issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(); err != nil {
			log.Error("LoadPullRequest failed: %v", err)
			return
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
}

func (m *webhookNotifier) NotifyIssueChangeStatus(doer *models.User, issue *models.Issue, isClosed bool) {
	mode, _ := models.AccessLevel(issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(); err != nil {
			log.Error("LoadPullRequest: %v", err)
			return
		}
		// Merge pull request calls issue.changeStatus so we need to handle separately.
		apiPullRequest := &api.PullRequestPayload{
			Index:       issue.Index,
			PullRequest: issue.PullRequest.APIFormat(),
			Repository:  issue.Repo.APIFormat(mode),
			Sender:      doer.APIFormat(),
		}
		if isClosed {
			apiPullRequest.Action = api.HookIssueClosed
		} else {
			apiPullRequest.Action = api.HookIssueReOpened
		}
		err = models.PrepareWebhooks(issue.Repo, models.HookEventPullRequest, apiPullRequest)
	} else {
		apiIssue := &api.IssuePayload{
			Index:      issue.Index,
			Issue:      issue.APIFormat(),
			Repository: issue.Repo.APIFormat(mode),
			Sender:     doer.APIFormat(),
		}
		if isClosed {
			apiIssue.Action = api.HookIssueClosed
		} else {
			apiIssue.Action = api.HookIssueReOpened
		}
		err = models.PrepareWebhooks(issue.Repo, models.HookEventIssues, apiIssue)
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v, is_closed: %v]: %v", issue.IsPull, isClosed, err)
	} else {
		go models.HookQueue.Add(issue.Repo.ID)
	}
}
