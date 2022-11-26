// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	webhook_services "code.gitea.io/gitea/services/webhook"
)

type webhookNotifier struct {
	base.NullNotifier
}

var _ base.Notifier = &webhookNotifier{}

// NewNotifier create a new webhookNotifier notifier
func NewNotifier() base.Notifier {
	return &webhookNotifier{}
}

func (m *webhookNotifier) NotifyIssueClearLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) {
	if err := issue.LoadPoster(ctx); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}

	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(ctx, issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(ctx); err != nil {
			log.Error("LoadPullRequest: %v", err)
			return
		}

		err = webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventPullRequestLabel, &api.PullRequestPayload{
			Action:      api.HookIssueLabelCleared,
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
			Repository:  convert.ToRepo(issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		})
	} else {
		err = webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventIssueLabel, &api.IssuePayload{
			Action:     api.HookIssueLabelCleared,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(ctx, issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	oldMode, _ := access_model.AccessLevel(ctx, doer, oldRepo)
	mode, _ := access_model.AccessLevel(ctx, doer, repo)

	// forked webhook
	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: oldRepo}, webhook.HookEventFork, &api.ForkPayload{
		Forkee: convert.ToRepo(oldRepo, oldMode),
		Repo:   convert.ToRepo(repo, mode),
		Sender: convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", oldRepo.ID, err)
	}

	u := repo.MustOwner(ctx)

	// Add to hook queue for created repo after session commit.
	if u.IsOrganization() {
		if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: repo}, webhook.HookEventRepository, &api.RepositoryPayload{
			Action:       api.HookRepoCreated,
			Repository:   convert.ToRepo(repo, perm.AccessModeOwner),
			Organization: convert.ToUser(u, nil),
			Sender:       convert.ToUser(doer, nil),
		}); err != nil {
			log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
		}
	}
}

func (m *webhookNotifier) NotifyCreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	// Add to hook queue for created repo after session commit.
	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: repo}, webhook.HookEventRepository, &api.RepositoryPayload{
		Action:       api.HookRepoCreated,
		Repository:   convert.ToRepo(repo, perm.AccessModeOwner),
		Organization: convert.ToUser(u, nil),
		Sender:       convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) NotifyDeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository) {
	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: repo}, webhook.HookEventRepository, &api.RepositoryPayload{
		Action:       api.HookRepoDeleted,
		Repository:   convert.ToRepo(repo, perm.AccessModeOwner),
		Organization: convert.ToUser(repo.MustOwner(ctx), nil),
		Sender:       convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) NotifyMigrateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	// Add to hook queue for created repo after session commit.
	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: repo}, webhook.HookEventRepository, &api.RepositoryPayload{
		Action:       api.HookRepoCreated,
		Repository:   convert.ToRepo(repo, perm.AccessModeOwner),
		Organization: convert.ToUser(u, nil),
		Sender:       convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) NotifyIssueChangeAssignee(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment) {
	if issue.IsPull {
		mode, _ := access_model.AccessLevelUnit(ctx, doer, issue.Repo, unit.TypePullRequests)

		if err := issue.LoadPullRequest(ctx); err != nil {
			log.Error("LoadPullRequest failed: %v", err)
			return
		}
		issue.PullRequest.Issue = issue
		apiPullRequest := &api.PullRequestPayload{
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
			Repository:  convert.ToRepo(issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		}
		if removed {
			apiPullRequest.Action = api.HookIssueUnassigned
		} else {
			apiPullRequest.Action = api.HookIssueAssigned
		}
		// Assignee comment triggers a webhook
		if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventPullRequestAssign, apiPullRequest); err != nil {
			log.Error("PrepareWebhooks [is_pull: %v, remove_assignee: %v]: %v", issue.IsPull, removed, err)
			return
		}
	} else {
		mode, _ := access_model.AccessLevelUnit(ctx, doer, issue.Repo, unit.TypeIssues)
		apiIssue := &api.IssuePayload{
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(ctx, issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		}
		if removed {
			apiIssue.Action = api.HookIssueUnassigned
		} else {
			apiIssue.Action = api.HookIssueAssigned
		}
		// Assignee comment triggers a webhook
		if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventIssueAssign, apiIssue); err != nil {
			log.Error("PrepareWebhooks [is_pull: %v, remove_assignee: %v]: %v", issue.IsPull, removed, err)
			return
		}
	}
}

func (m *webhookNotifier) NotifyIssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	mode, _ := access_model.AccessLevel(ctx, issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(ctx); err != nil {
			log.Error("LoadPullRequest failed: %v", err)
			return
		}
		issue.PullRequest.Issue = issue
		err = webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventPullRequest, &api.PullRequestPayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Title: &api.ChangesFromPayload{
					From: oldTitle,
				},
			},
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
			Repository:  convert.ToRepo(issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		})
	} else {
		err = webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventIssues, &api.IssuePayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Title: &api.ChangesFromPayload{
					From: oldTitle,
				},
			},
			Issue:      convert.ToAPIIssue(ctx, issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		})
	}

	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyIssueChangeStatus(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
	mode, _ := access_model.AccessLevel(ctx, issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(ctx); err != nil {
			log.Error("LoadPullRequest: %v", err)
			return
		}
		// Merge pull request calls issue.changeStatus so we need to handle separately.
		apiPullRequest := &api.PullRequestPayload{
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
			Repository:  convert.ToRepo(issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		}
		if isClosed {
			apiPullRequest.Action = api.HookIssueClosed
		} else {
			apiPullRequest.Action = api.HookIssueReOpened
		}
		err = webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventPullRequest, apiPullRequest)
	} else {
		apiIssue := &api.IssuePayload{
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(ctx, issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		}
		if isClosed {
			apiIssue.Action = api.HookIssueClosed
		} else {
			apiIssue.Action = api.HookIssueReOpened
		}
		err = webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventIssues, apiIssue)
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v, is_closed: %v]: %v", issue.IsPull, isClosed, err)
	}
}

func (m *webhookNotifier) NotifyNewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("issue.LoadRepo: %v", err)
		return
	}
	if err := issue.LoadPoster(ctx); err != nil {
		log.Error("issue.LoadPoster: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(ctx, issue.Poster, issue.Repo)
	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventIssues, &api.IssuePayload{
		Action:     api.HookIssueOpened,
		Index:      issue.Index,
		Issue:      convert.ToAPIIssue(ctx, issue),
		Repository: convert.ToRepo(issue.Repo, mode),
		Sender:     convert.ToUser(issue.Poster, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyNewPullRequest(ctx context.Context, pull *issues_model.PullRequest, mentions []*user_model.User) {
	if err := pull.LoadIssue(ctx); err != nil {
		log.Error("pull.LoadIssue: %v", err)
		return
	}
	if err := pull.Issue.LoadRepo(ctx); err != nil {
		log.Error("pull.Issue.LoadRepo: %v", err)
		return
	}
	if err := pull.Issue.LoadPoster(ctx); err != nil {
		log.Error("pull.Issue.LoadPoster: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(ctx, pull.Issue.Poster, pull.Issue.Repo)
	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: pull.Issue.Repo}, webhook.HookEventPullRequest, &api.PullRequestPayload{
		Action:      api.HookIssueOpened,
		Index:       pull.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(ctx, pull, nil),
		Repository:  convert.ToRepo(pull.Issue.Repo, mode),
		Sender:      convert.ToUser(pull.Issue.Poster, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyIssueChangeContent(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldContent string) {
	mode, _ := access_model.AccessLevel(ctx, issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		issue.PullRequest.Issue = issue
		err = webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventPullRequest, &api.PullRequestPayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Body: &api.ChangesFromPayload{
					From: oldContent,
				},
			},
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
			Repository:  convert.ToRepo(issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		})
	} else {
		err = webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventIssues, &api.IssuePayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Body: &api.ChangesFromPayload{
					From: oldContent,
				},
			},
			Issue:      convert.ToAPIIssue(ctx, issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyUpdateComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment, oldContent string) {
	if err := c.LoadPoster(ctx); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}
	if err := c.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}

	if err := c.Issue.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	var eventType webhook.HookEventType
	if c.Issue.IsPull {
		eventType = webhook.HookEventPullRequestComment
	} else {
		eventType = webhook.HookEventIssueComment
	}

	mode, _ := access_model.AccessLevel(ctx, doer, c.Issue.Repo)
	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: c.Issue.Repo}, eventType, &api.IssueCommentPayload{
		Action:  api.HookIssueCommentEdited,
		Issue:   convert.ToAPIIssue(ctx, c.Issue),
		Comment: convert.ToComment(c),
		Changes: &api.ChangesPayload{
			Body: &api.ChangesFromPayload{
				From: oldContent,
			},
		},
		Repository: convert.ToRepo(c.Issue.Repo, mode),
		Sender:     convert.ToUser(doer, nil),
		IsPull:     c.Issue.IsPull,
	}); err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", c.ID, err)
	}
}

func (m *webhookNotifier) NotifyCreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User,
) {
	var eventType webhook.HookEventType
	if issue.IsPull {
		eventType = webhook.HookEventPullRequestComment
	} else {
		eventType = webhook.HookEventIssueComment
	}

	mode, _ := access_model.AccessLevel(ctx, doer, repo)
	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, eventType, &api.IssueCommentPayload{
		Action:     api.HookIssueCommentCreated,
		Issue:      convert.ToAPIIssue(ctx, issue),
		Comment:    convert.ToComment(comment),
		Repository: convert.ToRepo(repo, mode),
		Sender:     convert.ToUser(doer, nil),
		IsPull:     issue.IsPull,
	}); err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	}
}

func (m *webhookNotifier) NotifyDeleteComment(ctx context.Context, doer *user_model.User, comment *issues_model.Comment) {
	var err error

	if err = comment.LoadPoster(ctx); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}
	if err = comment.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}

	if err = comment.Issue.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	var eventType webhook.HookEventType
	if comment.Issue.IsPull {
		eventType = webhook.HookEventPullRequestComment
	} else {
		eventType = webhook.HookEventIssueComment
	}

	mode, _ := access_model.AccessLevel(ctx, doer, comment.Issue.Repo)
	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: comment.Issue.Repo}, eventType, &api.IssueCommentPayload{
		Action:     api.HookIssueCommentDeleted,
		Issue:      convert.ToAPIIssue(ctx, comment.Issue),
		Comment:    convert.ToComment(comment),
		Repository: convert.ToRepo(comment.Issue.Repo, mode),
		Sender:     convert.ToUser(doer, nil),
		IsPull:     comment.Issue.IsPull,
	}); err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	}
}

func (m *webhookNotifier) NotifyNewWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string) {
	// Add to hook queue for created wiki page.
	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: repo}, webhook.HookEventWiki, &api.WikiPayload{
		Action:     api.HookWikiCreated,
		Repository: convert.ToRepo(repo, perm.AccessModeOwner),
		Sender:     convert.ToUser(doer, nil),
		Page:       page,
		Comment:    comment,
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) NotifyEditWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string) {
	// Add to hook queue for edit wiki page.
	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: repo}, webhook.HookEventWiki, &api.WikiPayload{
		Action:     api.HookWikiEdited,
		Repository: convert.ToRepo(repo, perm.AccessModeOwner),
		Sender:     convert.ToUser(doer, nil),
		Page:       page,
		Comment:    comment,
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) NotifyDeleteWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page string) {
	// Add to hook queue for edit wiki page.
	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: repo}, webhook.HookEventWiki, &api.WikiPayload{
		Action:     api.HookWikiDeleted,
		Repository: convert.ToRepo(repo, perm.AccessModeOwner),
		Sender:     convert.ToUser(doer, nil),
		Page:       page,
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) NotifyIssueChangeLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue,
	addedLabels, removedLabels []*issues_model.Label,
) {
	var err error

	if err = issue.LoadRepo(ctx); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}

	if err = issue.LoadPoster(ctx); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(ctx, issue.Poster, issue.Repo)
	if issue.IsPull {
		if err = issue.LoadPullRequest(ctx); err != nil {
			log.Error("loadPullRequest: %v", err)
			return
		}
		if err = issue.PullRequest.LoadIssue(ctx); err != nil {
			log.Error("LoadIssue: %v", err)
			return
		}
		err = webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventPullRequestLabel, &api.PullRequestPayload{
			Action:      api.HookIssueLabelUpdated,
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
			Repository:  convert.ToRepo(issue.Repo, perm.AccessModeNone),
			Sender:      convert.ToUser(doer, nil),
		})
	} else {
		err = webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventIssueLabel, &api.IssuePayload{
			Action:     api.HookIssueLabelUpdated,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(ctx, issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyIssueChangeMilestone(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldMilestoneID int64) {
	var hookAction api.HookIssueAction
	var err error
	if issue.MilestoneID > 0 {
		hookAction = api.HookIssueMilestoned
	} else {
		hookAction = api.HookIssueDemilestoned
	}

	if err = issue.LoadAttributes(ctx); err != nil {
		log.Error("issue.LoadAttributes failed: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(ctx, doer, issue.Repo)
	if issue.IsPull {
		err = issue.PullRequest.LoadIssue(ctx)
		if err != nil {
			log.Error("LoadIssue: %v", err)
			return
		}
		err = webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventPullRequestMilestone, &api.PullRequestPayload{
			Action:      hookAction,
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
			Repository:  convert.ToRepo(issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		})
	} else {
		err = webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventIssueMilestone, &api.IssuePayload{
			Action:     hookAction,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(ctx, issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	apiPusher := convert.ToUser(pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(ctx, repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: repo}, webhook.HookEventPush, &api.PushPayload{
		Ref:          opts.RefFullName,
		Before:       opts.OldCommitID,
		After:        opts.NewCommitID,
		CompareURL:   setting.AppURL + commits.CompareURL,
		Commits:      apiCommits,
		TotalCommits: commits.Len,
		HeadCommit:   apiHeadCommit,
		Repo:         convert.ToRepo(repo, perm.AccessModeOwner),
		Pusher:       apiPusher,
		Sender:       apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyAutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	// just redirect to the NotifyMergePullRequest
	m.NotifyMergePullRequest(ctx, doer, pr)
}

func (*webhookNotifier) NotifyMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	// Reload pull request information.
	if err := pr.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}

	if err := pr.Issue.LoadRepo(ctx); err != nil {
		log.Error("pr.Issue.LoadRepo: %v", err)
		return
	}

	mode, err := access_model.AccessLevel(ctx, doer, pr.Issue.Repo)
	if err != nil {
		log.Error("models.AccessLevel: %v", err)
		return
	}

	// Merge pull request calls issue.changeStatus so we need to handle separately.
	apiPullRequest := &api.PullRequestPayload{
		Index:       pr.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(ctx, pr, nil),
		Repository:  convert.ToRepo(pr.Issue.Repo, mode),
		Sender:      convert.ToUser(doer, nil),
		Action:      api.HookIssueClosed,
	}

	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: pr.Issue.Repo}, webhook.HookEventPullRequest, apiPullRequest); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyPullRequestChangeTargetBranch(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, oldBranch string) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}

	issue := pr.Issue

	mode, _ := access_model.AccessLevel(ctx, issue.Poster, issue.Repo)
	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: issue.Repo}, webhook.HookEventPullRequest, &api.PullRequestPayload{
		Action: api.HookIssueEdited,
		Index:  issue.Index,
		Changes: &api.ChangesPayload{
			Ref: &api.ChangesFromPayload{
				From: oldBranch,
			},
		},
		PullRequest: convert.ToAPIPullRequest(ctx, pr, nil),
		Repository:  convert.ToRepo(issue.Repo, mode),
		Sender:      convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [pr: %d]: %v", pr.ID, err)
	}
}

func (m *webhookNotifier) NotifyPullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	var reviewHookType webhook.HookEventType

	switch review.Type {
	case issues_model.ReviewTypeApprove:
		reviewHookType = webhook.HookEventPullRequestReviewApproved
	case issues_model.ReviewTypeComment:
		reviewHookType = webhook.HookEventPullRequestComment
	case issues_model.ReviewTypeReject:
		reviewHookType = webhook.HookEventPullRequestReviewRejected
	default:
		// unsupported review webhook type here
		log.Error("Unsupported review webhook type")
		return
	}

	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}

	mode, err := access_model.AccessLevel(ctx, review.Issue.Poster, review.Issue.Repo)
	if err != nil {
		log.Error("models.AccessLevel: %v", err)
		return
	}
	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: review.Issue.Repo}, reviewHookType, &api.PullRequestPayload{
		Action:      api.HookIssueReviewed,
		Index:       review.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(ctx, pr, nil),
		Repository:  convert.ToRepo(review.Issue.Repo, mode),
		Sender:      convert.ToUser(review.Reviewer, nil),
		Review: &api.ReviewPayload{
			Type:    string(reviewHookType),
			Content: review.Content,
		},
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyCreateRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
	apiPusher := convert.ToUser(pusher, nil)
	apiRepo := convert.ToRepo(repo, perm.AccessModeNone)
	refName := git.RefEndName(refFullName)

	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: repo}, webhook.HookEventCreate, &api.CreatePayload{
		Ref:     refName,
		Sha:     refID,
		RefType: refType,
		Repo:    apiRepo,
		Sender:  apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyPullRequestSynchronized(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}
	if err := pr.Issue.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: pr.Issue.Repo}, webhook.HookEventPullRequestSync, &api.PullRequestPayload{
		Action:      api.HookIssueSynchronized,
		Index:       pr.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(ctx, pr, nil),
		Repository:  convert.ToRepo(pr.Issue.Repo, perm.AccessModeNone),
		Sender:      convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [pull_id: %v]: %v", pr.ID, err)
	}
}

func (m *webhookNotifier) NotifyDeleteRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
	apiPusher := convert.ToUser(pusher, nil)
	apiRepo := convert.ToRepo(repo, perm.AccessModeNone)
	refName := git.RefEndName(refFullName)

	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: repo}, webhook.HookEventDelete, &api.DeletePayload{
		Ref:        refName,
		RefType:    refType,
		PusherType: api.PusherTypeUser,
		Repo:       apiRepo,
		Sender:     apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks.(delete %s): %v", refType, err)
	}
}

func sendReleaseHook(ctx context.Context, doer *user_model.User, rel *repo_model.Release, action api.HookReleaseAction) {
	if err := rel.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(ctx, doer, rel.Repo)
	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: rel.Repo}, webhook.HookEventRelease, &api.ReleasePayload{
		Action:     action,
		Release:    convert.ToRelease(rel),
		Repository: convert.ToRepo(rel.Repo, mode),
		Sender:     convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyNewRelease(ctx context.Context, rel *repo_model.Release) {
	sendReleaseHook(ctx, rel.Publisher, rel, api.HookReleasePublished)
}

func (m *webhookNotifier) NotifyUpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	sendReleaseHook(ctx, doer, rel, api.HookReleaseUpdated)
}

func (m *webhookNotifier) NotifyDeleteRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	sendReleaseHook(ctx, doer, rel, api.HookReleaseDeleted)
}

func (m *webhookNotifier) NotifySyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	apiPusher := convert.ToUser(pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(ctx, repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	if err := webhook_services.PrepareWebhooks(ctx, webhook_services.EventSource{Repository: repo}, webhook.HookEventPush, &api.PushPayload{
		Ref:          opts.RefFullName,
		Before:       opts.OldCommitID,
		After:        opts.NewCommitID,
		CompareURL:   setting.AppURL + commits.CompareURL,
		Commits:      apiCommits,
		TotalCommits: commits.Len,
		HeadCommit:   apiHeadCommit,
		Repo:         convert.ToRepo(repo, perm.AccessModeOwner),
		Pusher:       apiPusher,
		Sender:       apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifySyncCreateRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
	m.NotifyCreateRef(ctx, pusher, repo, refType, refFullName, refID)
}

func (m *webhookNotifier) NotifySyncDeleteRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
	m.NotifyDeleteRef(ctx, pusher, repo, refType, refFullName)
}

func (m *webhookNotifier) NotifyPackageCreate(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
	notifyPackage(ctx, doer, pd, api.HookPackageCreated)
}

func (m *webhookNotifier) NotifyPackageDelete(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
	notifyPackage(ctx, doer, pd, api.HookPackageDeleted)
}

func notifyPackage(ctx context.Context, sender *user_model.User, pd *packages_model.PackageDescriptor, action api.HookPackageAction) {
	source := webhook_services.EventSource{
		Repository: pd.Repository,
		Owner:      pd.Owner,
	}

	apiPackage, err := convert.ToPackage(ctx, pd, sender)
	if err != nil {
		log.Error("Error converting package: %v", err)
		return
	}

	if err := webhook_services.PrepareWebhooks(ctx, source, webhook.HookEventPackage, &api.PackagePayload{
		Action:  action,
		Package: apiPackage,
		Sender:  convert.ToUser(sender, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}
