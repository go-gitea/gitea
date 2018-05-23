// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/setting"

	api "code.gitea.io/sdk/gitea"
)

type webhookNotifier struct {
}

var (
	_ base.Notifier = &webhookNotifier{}
)

// NewNotifier returns a new webhookNotifier
func NewNotifier() base.Notifier {
	return &webhookNotifier{}
}

func (w *webhookNotifier) Run() {
}

func (w *webhookNotifier) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
	mode, _ := models.AccessLevel(doer.ID, repo)
	if err := models.PrepareWebhooks(repo, models.HookEventIssueComment, &api.IssueCommentPayload{
		Action:     api.HookIssueCommentCreated,
		Issue:      issue.APIFormat(),
		Comment:    comment.APIFormat(),
		Repository: repo.APIFormat(mode),
		Sender:     doer.APIFormat(),
	}); err != nil {
		log.Error(2, "PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	} else {
		go models.HookQueue.Add(repo.ID)
	}
}

// NotifyNewIssue implements notification.Receiver
func (w *webhookNotifier) NotifyNewIssue(issue *models.Issue) {
	mode, _ := models.AccessLevel(issue.Poster.ID, issue.Repo)
	if err := models.PrepareWebhooks(issue.Repo, models.HookEventIssues, &api.IssuePayload{
		Action:     api.HookIssueOpened,
		Index:      issue.Index,
		Issue:      issue.APIFormat(),
		Repository: issue.Repo.APIFormat(mode),
		Sender:     issue.Poster.APIFormat(),
	}); err != nil {
		log.Error(4, "PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(issue.RepoID)
	}
}

// NotifyCloseIssue implements notification.Receiver
func (w *webhookNotifier) NotifyCloseIssue(issue *models.Issue, doer *models.User) {
	panic("not implements")
}

func (w *webhookNotifier) NotifyMergePullRequest(pr *models.PullRequest, doer *models.User, baseGitRepo *git.Repository) {
	mode, _ := models.AccessLevel(doer.ID, pr.Issue.Repo)
	if err := models.PrepareWebhooks(pr.Issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
		Action:      api.HookIssueClosed,
		Index:       pr.Index,
		PullRequest: pr.APIFormat(),
		Repository:  pr.Issue.Repo.APIFormat(mode),
		Sender:      doer.APIFormat(),
	}); err != nil {
		log.Error(4, "PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(pr.Issue.Repo.ID)
	}

	l, err := baseGitRepo.CommitsBetweenIDs(pr.MergedCommitID, pr.MergeBase)
	if err != nil {
		log.Error(4, "CommitsBetweenIDs: %v", err)
		return
	}

	// It is possible that head branch is not fully sync with base branch for merge commits,
	// so we need to get latest head commit and append merge commit manually
	// to avoid strange diff commits produced.
	mergeCommit, err := baseGitRepo.GetBranchCommit(pr.BaseBranch)
	if err != nil {
		log.Error(4, "GetBranchCommit: %v", err)
		return
	}

	p := &api.PushPayload{
		Ref:        git.BranchPrefix + pr.BaseBranch,
		Before:     pr.MergeBase,
		After:      mergeCommit.ID.String(),
		CompareURL: setting.AppURL + pr.BaseRepo.ComposeCompareURL(pr.MergeBase, pr.MergedCommitID),
		Commits:    models.ListToPushCommits(l).ToAPIPayloadCommits(pr.BaseRepo.HTMLURL()),
		Repo:       pr.BaseRepo.APIFormat(mode),
		Pusher:     pr.HeadRepo.MustOwner().APIFormat(),
		Sender:     doer.APIFormat(),
	}
	if err := models.PrepareWebhooks(pr.BaseRepo, models.HookEventPush, p); err != nil {
		log.Error(4, "PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(pr.BaseRepo.ID)
	}
}

func (w *webhookNotifier) NotifyNewPullRequest(pr *models.PullRequest) {
	mode, _ := models.AccessLevel(pr.Issue.Poster.ID, pr.Issue.Repo)
	if err := models.PrepareWebhooks(pr.Issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
		Action:      api.HookIssueOpened,
		Index:       pr.Issue.Index,
		PullRequest: pr.APIFormat(),
		Repository:  pr.Issue.Repo.APIFormat(mode),
		Sender:      pr.Issue.Poster.APIFormat(),
	}); err != nil {
		log.Error(4, "PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(pr.Issue.Repo.ID)
	}
}

func (w *webhookNotifier) NotifyUpdateComment(doer *models.User, c *models.Comment, oldContent string) {
	if err := c.LoadIssue(); err != nil {
		log.Error(2, "LoadIssue [comment_id: %d]: %v", c.ID, err)
		return
	}
	if err := c.Issue.LoadAttributes(); err != nil {
		log.Error(2, "Issue.LoadAttributes [comment_id: %d]: %v", c.ID, err)
		return
	}

	mode, _ := models.AccessLevel(doer.ID, c.Issue.Repo)
	if err := models.PrepareWebhooks(c.Issue.Repo, models.HookEventIssueComment, &api.IssueCommentPayload{
		Action:  api.HookIssueCommentEdited,
		Issue:   c.Issue.APIFormat(),
		Comment: c.APIFormat(),
		Changes: &api.ChangesPayload{
			Body: &api.ChangesFromPayload{
				From: oldContent,
			},
		},
		Repository: c.Issue.Repo.APIFormat(mode),
		Sender:     doer.APIFormat(),
	}); err != nil {
		log.Error(2, "PrepareWebhooks [comment_id: %d]: %v", c.ID, err)
	} else {
		go models.HookQueue.Add(c.Issue.Repo.ID)
	}
}

func (w *webhookNotifier) NotifyDeleteComment(doer *models.User, comment *models.Comment) {
	mode, _ := models.AccessLevel(doer.ID, comment.Issue.Repo)

	if err := models.PrepareWebhooks(comment.Issue.Repo, models.HookEventIssueComment, &api.IssueCommentPayload{
		Action:     api.HookIssueCommentDeleted,
		Issue:      comment.Issue.APIFormat(),
		Comment:    comment.APIFormat(),
		Repository: comment.Issue.Repo.APIFormat(mode),
		Sender:     doer.APIFormat(),
	}); err != nil {
		log.Error(2, "PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	} else {
		go models.HookQueue.Add(comment.Issue.Repo.ID)
	}
}

func (w *webhookNotifier) NotifyDeleteRepository(doer *models.User, repo *models.Repository) {
	org, err := models.GetUserByID(repo.OwnerID)
	if err != nil {
		log.Error(2, "GetUserByID [repo_id: %d]: %v", repo.ID, err)
		return
	}

	if org.IsOrganization() {
		if err := models.PrepareWebhooks(repo, models.HookEventRepository, &api.RepositoryPayload{
			Action:       api.HookRepoDeleted,
			Repository:   repo.APIFormat(models.AccessModeOwner),
			Organization: org.APIFormat(),
			Sender:       doer.APIFormat(),
		}); err != nil {
			log.Error(2, "PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
		} else {
			go models.HookQueue.Add(repo.ID)
		}
	}
}

func (w *webhookNotifier) NotifyForkRepository(doer *models.User, oldRepo, repo *models.Repository) {
	oldMode, _ := models.AccessLevel(doer.ID, oldRepo)
	mode, _ := models.AccessLevel(doer.ID, repo)

	if err := models.PrepareWebhooks(oldRepo, models.HookEventFork, &api.ForkPayload{
		Forkee: oldRepo.APIFormat(oldMode),
		Repo:   repo.APIFormat(mode),
		Sender: doer.APIFormat(),
	}); err != nil {
		log.Error(2, "PrepareWebhooks [repo_id: %d]: %v", oldRepo.ID, err)
	} else {
		go models.HookQueue.Add(oldRepo.ID)
	}
}

func (w *webhookNotifier) NotifyNewRelease(rel *models.Release) {
	if rel.IsDraft {
		return
	}

	if err := rel.LoadAttributes(); err != nil {
		log.Error(2, "LoadAttributes: %v", err)
	} else {
		mode, _ := models.AccessLevel(rel.PublisherID, rel.Repo)
		if err := models.PrepareWebhooks(rel.Repo, models.HookEventRelease, &api.ReleasePayload{
			Action:     api.HookReleasePublished,
			Release:    rel.APIFormat(),
			Repository: rel.Repo.APIFormat(mode),
			Sender:     rel.Publisher.APIFormat(),
		}); err != nil {
			log.Error(2, "PrepareWebhooks: %v", err)
		} else {
			go models.HookQueue.Add(rel.Repo.ID)
		}
	}
}

func (w *webhookNotifier) NotifyUpdateRelease(doer *models.User, rel *models.Release) {
	mode, _ := models.AccessLevel(doer.ID, rel.Repo)
	if err := models.PrepareWebhooks(rel.Repo, models.HookEventRelease, &api.ReleasePayload{
		Action:     api.HookReleaseUpdated,
		Release:    rel.APIFormat(),
		Repository: rel.Repo.APIFormat(mode),
		Sender:     rel.Publisher.APIFormat(),
	}); err != nil {
		log.Error(2, "PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(rel.Repo.ID)
	}
}

func (w *webhookNotifier) NotifyDeleteRelease(doer *models.User, rel *models.Release) {
	if err := rel.LoadAttributes(); err != nil {
		log.Error(2, "rel.LoadAttributes: %v", err)
		return
	}

	mode, _ := models.AccessLevel(doer.ID, rel.Repo)
	if err := models.PrepareWebhooks(rel.Repo, models.HookEventRelease, &api.ReleasePayload{
		Action:     api.HookReleaseDeleted,
		Release:    rel.APIFormat(),
		Repository: rel.Repo.APIFormat(mode),
		Sender:     rel.Publisher.APIFormat(),
	}); err != nil {
		log.Error(2, "PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(rel.Repo.ID)
	}
}

func (w *webhookNotifier) NotifyChangeMilestone(doer *models.User, issue *models.Issue) {
	var hookAction api.HookIssueAction
	if issue.MilestoneID > 0 {
		hookAction = api.HookIssueMilestoned
	} else {
		hookAction = api.HookIssueDemilestoned
	}

	var err error
	if err = issue.LoadAttributes(); err != nil {
		log.Error(2, "LoadAttributes: %v", err)
		return
	}

	mode, _ := models.AccessLevel(doer.ID, issue.Repo)
	if issue.IsPull {
		err = issue.PullRequest.LoadIssue()
		if err != nil {
			log.Error(2, "LoadIssue: %v", err)
			return
		}
		err = models.PrepareWebhooks(issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
			Action:      hookAction,
			Index:       issue.Index,
			PullRequest: issue.PullRequest.APIFormat(),
			Repository:  issue.Repo.APIFormat(mode),
			Sender:      doer.APIFormat(),
		})
	} else {
		err = models.PrepareWebhooks(issue.Repo, models.HookEventIssues, &api.IssuePayload{
			Action:     hookAction,
			Index:      issue.Index,
			Issue:      issue.APIFormat(),
			Repository: issue.Repo.APIFormat(mode),
			Sender:     doer.APIFormat(),
		})
	}
	if err != nil {
		log.Error(2, "PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	} else {
		go models.HookQueue.Add(issue.RepoID)
	}
}

func (w *webhookNotifier) NotifyIssueChangeContent(doer *models.User, issue *models.Issue, oldContent string) {
	mode, _ := models.AccessLevel(doer.ID, issue.Repo)
	var err error
	if issue.IsPull {
		issue.PullRequest.Issue = issue
		err = models.PrepareWebhooks(issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Body: &api.ChangesFromPayload{
					From: oldContent,
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
				Body: &api.ChangesFromPayload{
					From: oldContent,
				},
			},
			Issue:      issue.APIFormat(),
			Repository: issue.Repo.APIFormat(mode),
			Sender:     doer.APIFormat(),
		})
	}
	if err != nil {
		log.Error(4, "PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	} else {
		go models.HookQueue.Add(issue.RepoID)
	}
}

func (w *webhookNotifier) NotifyIssueClearLabels(doer *models.User, issue *models.Issue) {
	var err error
	mode, _ := models.AccessLevel(doer.ID, issue.Repo)
	if issue.IsPull {
		err = issue.PullRequest.LoadIssue()
		if err != nil {
			log.Error(4, "LoadIssue: %v", err)
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
		log.Error(4, "PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	} else {
		go models.HookQueue.Add(issue.RepoID)
	}
}

func (w *webhookNotifier) NotifyIssueChangeTitle(doer *models.User, issue *models.Issue, oldTitle string) {
	mode, _ := models.AccessLevel(doer.ID, issue.Repo)
	var err error
	if issue.IsPull {
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
		log.Error(4, "PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	} else {
		go models.HookQueue.Add(issue.RepoID)
	}
}

/*
func (w *webhookNotifier) NotifyIssueChangeStatus(doer *models.User, issue *models.Issue, isClosed bool) {
	mode, _ := models.AccessLevel(doer.ID, issue.Repo)
	var err error
	if issue.IsPull {
		// Merge pull request calls issue.changeStatus so we need to handle separately.
		issue.PullRequest.Issue = issue
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
		log.Error(4, "PrepareWebhooks [is_pull: %v, is_closed: %v]: %v", issue.IsPull, isClosed, err)
	} else {
		go models.HookQueue.Add(issue.Repo.ID)
	}
}*/

func (w *webhookNotifier) NotifyIssueChangeLabels(doer *models.User, issue *models.Issue,
	addedLabels []*models.Label, removedLabels []*models.Label) {
	var err error
	if err = issue.LoadRepo(); err != nil {
		log.Error(4, "LoadRepo: %v", err)
		return
	}

	mode, _ := models.AccessLevel(doer.ID, issue.Repo)
	if issue.IsPull {
		if err = issue.LoadPullRequest(); err != nil {
			log.Error(4, "LoadPullRequest: %v", err)
			return
		}
		if err = issue.PullRequest.LoadIssue(); err != nil {
			log.Error(4, "LoadIssue: %v", err)
			return
		}
		err = models.PrepareWebhooks(issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
			Action:      api.HookIssueLabelUpdated,
			Index:       issue.Index,
			PullRequest: issue.PullRequest.APIFormat(),
			Repository:  issue.Repo.APIFormat(mode),
			Sender:      doer.APIFormat(),
		})
	} else {
		err = models.PrepareWebhooks(issue.Repo, models.HookEventIssues, &api.IssuePayload{
			Action:     api.HookIssueLabelUpdated,
			Index:      issue.Index,
			Issue:      issue.APIFormat(),
			Repository: issue.Repo.APIFormat(mode),
			Sender:     doer.APIFormat(),
		})
	}
	if err != nil {
		log.Error(4, "PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	} else {
		go models.HookQueue.Add(issue.RepoID)
	}
}
