// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/services/convert"
	notify_service "code.gitea.io/gitea/services/notify"
)

func init() {
	notify_service.RegisterNotifier(NewNotifier())
}

type webhookNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &webhookNotifier{}

// NewNotifier create a new webhookNotifier notifier
func NewNotifier() notify_service.Notifier {
	return &webhookNotifier{}
}

func (m *webhookNotifier) IssueClearLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) {
	if err := issue.LoadPoster(ctx); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}

	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}

	permission, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, issue.Poster)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(ctx); err != nil {
			log.Error("LoadPullRequest: %v", err)
			return
		}

		err = PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventPullRequestLabel, &api.PullRequestPayload{
			Action:      api.HookIssueLabelCleared,
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, doer),
			Repository:  convert.ToRepo(ctx, issue.Repo, permission),
			Sender:      convert.ToUser(ctx, doer, nil),
		})
	} else {
		err = PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventIssueLabel, &api.IssuePayload{
			Action:     api.HookIssueLabelCleared,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(ctx, doer, issue),
			Repository: convert.ToRepo(ctx, issue.Repo, permission),
			Sender:     convert.ToUser(ctx, doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) ForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	oldPermission, _ := access_model.GetUserRepoPermission(ctx, oldRepo, doer)
	permission, _ := access_model.GetUserRepoPermission(ctx, repo, doer)

	// forked webhook
	if err := PrepareWebhooks(ctx, EventSource{Repository: oldRepo}, webhook_module.HookEventFork, &api.ForkPayload{
		Forkee: convert.ToRepo(ctx, oldRepo, oldPermission),
		Repo:   convert.ToRepo(ctx, repo, permission),
		Sender: convert.ToUser(ctx, doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", oldRepo.ID, err)
	}

	u := repo.MustOwner(ctx)

	// Add to hook queue for created repo after session commit.
	if u.IsOrganization() {
		if err := PrepareWebhooks(ctx, EventSource{Repository: repo}, webhook_module.HookEventRepository, &api.RepositoryPayload{
			Action:       api.HookRepoCreated,
			Repository:   convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}),
			Organization: convert.ToUser(ctx, u, nil),
			Sender:       convert.ToUser(ctx, doer, nil),
		}); err != nil {
			log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
		}
	}
}

func (m *webhookNotifier) CreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	// Add to hook queue for created repo after session commit.
	if err := PrepareWebhooks(ctx, EventSource{Repository: repo}, webhook_module.HookEventRepository, &api.RepositoryPayload{
		Action:       api.HookRepoCreated,
		Repository:   convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}),
		Organization: convert.ToUser(ctx, u, nil),
		Sender:       convert.ToUser(ctx, doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) DeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository) {
	if err := PrepareWebhooks(ctx, EventSource{Repository: repo}, webhook_module.HookEventRepository, &api.RepositoryPayload{
		Action:       api.HookRepoDeleted,
		Repository:   convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}),
		Organization: convert.ToUser(ctx, repo.MustOwner(ctx), nil),
		Sender:       convert.ToUser(ctx, doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) MigrateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	// Add to hook queue for created repo after session commit.
	if err := PrepareWebhooks(ctx, EventSource{Repository: repo}, webhook_module.HookEventRepository, &api.RepositoryPayload{
		Action:       api.HookRepoCreated,
		Repository:   convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}),
		Organization: convert.ToUser(ctx, u, nil),
		Sender:       convert.ToUser(ctx, doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) IssueChangeAssignee(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment) {
	if issue.IsPull {
		permission, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, doer)

		if err := issue.LoadPullRequest(ctx); err != nil {
			log.Error("LoadPullRequest failed: %v", err)
			return
		}
		apiPullRequest := &api.PullRequestPayload{
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, doer),
			Repository:  convert.ToRepo(ctx, issue.Repo, permission),
			Sender:      convert.ToUser(ctx, doer, nil),
		}
		if removed {
			apiPullRequest.Action = api.HookIssueUnassigned
		} else {
			apiPullRequest.Action = api.HookIssueAssigned
		}
		// Assignee comment triggers a webhook
		if err := PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventPullRequestAssign, apiPullRequest); err != nil {
			log.Error("PrepareWebhooks [is_pull: %v, remove_assignee: %v]: %v", issue.IsPull, removed, err)
			return
		}
	} else {
		permission, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, doer)
		apiIssue := &api.IssuePayload{
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(ctx, doer, issue),
			Repository: convert.ToRepo(ctx, issue.Repo, permission),
			Sender:     convert.ToUser(ctx, doer, nil),
		}
		if removed {
			apiIssue.Action = api.HookIssueUnassigned
		} else {
			apiIssue.Action = api.HookIssueAssigned
		}
		// Assignee comment triggers a webhook
		if err := PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventIssueAssign, apiIssue); err != nil {
			log.Error("PrepareWebhooks [is_pull: %v, remove_assignee: %v]: %v", issue.IsPull, removed, err)
			return
		}
	}
}

func (m *webhookNotifier) IssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	permission, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, issue.Poster)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(ctx); err != nil {
			log.Error("LoadPullRequest failed: %v", err)
			return
		}
		err = PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventPullRequest, &api.PullRequestPayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Title: &api.ChangesFromPayload{
					From: oldTitle,
				},
			},
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, doer),
			Repository:  convert.ToRepo(ctx, issue.Repo, permission),
			Sender:      convert.ToUser(ctx, doer, nil),
		})
	} else {
		err = PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventIssues, &api.IssuePayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Title: &api.ChangesFromPayload{
					From: oldTitle,
				},
			},
			Issue:      convert.ToAPIIssue(ctx, doer, issue),
			Repository: convert.ToRepo(ctx, issue.Repo, permission),
			Sender:     convert.ToUser(ctx, doer, nil),
		})
	}

	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
	permission, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, issue.Poster)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(ctx); err != nil {
			log.Error("LoadPullRequest: %v", err)
			return
		}
		// Merge pull request calls issue.changeStatus so we need to handle separately.
		apiPullRequest := &api.PullRequestPayload{
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, doer),
			Repository:  convert.ToRepo(ctx, issue.Repo, permission),
			Sender:      convert.ToUser(ctx, doer, nil),
			CommitID:    commitID,
		}
		if isClosed {
			apiPullRequest.Action = api.HookIssueClosed
		} else {
			apiPullRequest.Action = api.HookIssueReOpened
		}
		err = PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventPullRequest, apiPullRequest)
	} else {
		apiIssue := &api.IssuePayload{
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(ctx, doer, issue),
			Repository: convert.ToRepo(ctx, issue.Repo, permission),
			Sender:     convert.ToUser(ctx, doer, nil),
			CommitID:   commitID,
		}
		if isClosed {
			apiIssue.Action = api.HookIssueClosed
		} else {
			apiIssue.Action = api.HookIssueReOpened
		}
		err = PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventIssues, apiIssue)
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v, is_closed: %v]: %v", issue.IsPull, isClosed, err)
	}
}

func (m *webhookNotifier) NewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("issue.LoadRepo: %v", err)
		return
	}
	if err := issue.LoadPoster(ctx); err != nil {
		log.Error("issue.LoadPoster: %v", err)
		return
	}

	permission, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, issue.Poster)
	if err := PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventIssues, &api.IssuePayload{
		Action:     api.HookIssueOpened,
		Index:      issue.Index,
		Issue:      convert.ToAPIIssue(ctx, issue.Poster, issue),
		Repository: convert.ToRepo(ctx, issue.Repo, permission),
		Sender:     convert.ToUser(ctx, issue.Poster, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NewPullRequest(ctx context.Context, pull *issues_model.PullRequest, mentions []*user_model.User) {
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

	permission, _ := access_model.GetUserRepoPermission(ctx, pull.Issue.Repo, pull.Issue.Poster)
	if err := PrepareWebhooks(ctx, EventSource{Repository: pull.Issue.Repo}, webhook_module.HookEventPullRequest, &api.PullRequestPayload{
		Action:      api.HookIssueOpened,
		Index:       pull.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(ctx, pull, pull.Issue.Poster),
		Repository:  convert.ToRepo(ctx, pull.Issue.Repo, permission),
		Sender:      convert.ToUser(ctx, pull.Issue.Poster, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) IssueChangeContent(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldContent string) {
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}

	permission, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, issue.Poster)
	var err error
	if issue.IsPull {
		if err := issue.LoadPullRequest(ctx); err != nil {
			log.Error("LoadPullRequest: %v", err)
			return
		}
		err = PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventPullRequest, &api.PullRequestPayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Body: &api.ChangesFromPayload{
					From: oldContent,
				},
			},
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, doer),
			Repository:  convert.ToRepo(ctx, issue.Repo, permission),
			Sender:      convert.ToUser(ctx, doer, nil),
		})
	} else {
		err = PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventIssues, &api.IssuePayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Body: &api.ChangesFromPayload{
					From: oldContent,
				},
			},
			Issue:      convert.ToAPIIssue(ctx, doer, issue),
			Repository: convert.ToRepo(ctx, issue.Repo, permission),
			Sender:     convert.ToUser(ctx, doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) UpdateComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment, oldContent string) {
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

	var eventType webhook_module.HookEventType
	var pullRequest *api.PullRequest
	if c.Issue.IsPull {
		eventType = webhook_module.HookEventPullRequestComment
		pullRequest = convert.ToAPIPullRequest(ctx, c.Issue.PullRequest, doer)
	} else {
		eventType = webhook_module.HookEventIssueComment
	}

	permission, _ := access_model.GetUserRepoPermission(ctx, c.Issue.Repo, doer)
	if err := PrepareWebhooks(ctx, EventSource{Repository: c.Issue.Repo}, eventType, &api.IssueCommentPayload{
		Action:      api.HookIssueCommentEdited,
		Issue:       convert.ToAPIIssue(ctx, doer, c.Issue),
		PullRequest: pullRequest,
		Comment:     convert.ToAPIComment(ctx, c.Issue.Repo, c),
		Changes: &api.ChangesPayload{
			Body: &api.ChangesFromPayload{
				From: oldContent,
			},
		},
		Repository: convert.ToRepo(ctx, c.Issue.Repo, permission),
		Sender:     convert.ToUser(ctx, doer, nil),
		IsPull:     c.Issue.IsPull,
	}); err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", c.ID, err)
	}
}

func (m *webhookNotifier) CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User,
) {
	var eventType webhook_module.HookEventType
	var pullRequest *api.PullRequest
	if issue.IsPull {
		eventType = webhook_module.HookEventPullRequestComment
		if err := issue.LoadPullRequest(ctx); err != nil {
			log.Error("LoadPullRequest: %v", err)
			return
		}
		pullRequest = convert.ToAPIPullRequest(ctx, issue.PullRequest, doer)
	} else {
		eventType = webhook_module.HookEventIssueComment
	}

	permission, _ := access_model.GetUserRepoPermission(ctx, repo, doer)
	if err := PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, eventType, &api.IssueCommentPayload{
		Action:      api.HookIssueCommentCreated,
		Issue:       convert.ToAPIIssue(ctx, doer, issue),
		PullRequest: pullRequest,
		Comment:     convert.ToAPIComment(ctx, repo, comment),
		Repository:  convert.ToRepo(ctx, repo, permission),
		Sender:      convert.ToUser(ctx, doer, nil),
		IsPull:      issue.IsPull,
	}); err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	}
}

func (m *webhookNotifier) DeleteComment(ctx context.Context, doer *user_model.User, comment *issues_model.Comment) {
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

	var eventType webhook_module.HookEventType
	var pullRequest *api.PullRequest
	if comment.Issue.IsPull {
		eventType = webhook_module.HookEventPullRequestComment
		pullRequest = convert.ToAPIPullRequest(ctx, comment.Issue.PullRequest, doer)
	} else {
		eventType = webhook_module.HookEventIssueComment
	}

	permission, _ := access_model.GetUserRepoPermission(ctx, comment.Issue.Repo, doer)
	if err := PrepareWebhooks(ctx, EventSource{Repository: comment.Issue.Repo}, eventType, &api.IssueCommentPayload{
		Action:      api.HookIssueCommentDeleted,
		Issue:       convert.ToAPIIssue(ctx, doer, comment.Issue),
		PullRequest: pullRequest,
		Comment:     convert.ToAPIComment(ctx, comment.Issue.Repo, comment),
		Repository:  convert.ToRepo(ctx, comment.Issue.Repo, permission),
		Sender:      convert.ToUser(ctx, doer, nil),
		IsPull:      comment.Issue.IsPull,
	}); err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	}
}

func (m *webhookNotifier) NewWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string) {
	// Add to hook queue for created wiki page.
	if err := PrepareWebhooks(ctx, EventSource{Repository: repo}, webhook_module.HookEventWiki, &api.WikiPayload{
		Action:     api.HookWikiCreated,
		Repository: convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}),
		Sender:     convert.ToUser(ctx, doer, nil),
		Page:       page,
		Comment:    comment,
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) EditWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string) {
	// Add to hook queue for edit wiki page.
	if err := PrepareWebhooks(ctx, EventSource{Repository: repo}, webhook_module.HookEventWiki, &api.WikiPayload{
		Action:     api.HookWikiEdited,
		Repository: convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}),
		Sender:     convert.ToUser(ctx, doer, nil),
		Page:       page,
		Comment:    comment,
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) DeleteWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page string) {
	// Add to hook queue for edit wiki page.
	if err := PrepareWebhooks(ctx, EventSource{Repository: repo}, webhook_module.HookEventWiki, &api.WikiPayload{
		Action:     api.HookWikiDeleted,
		Repository: convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}),
		Sender:     convert.ToUser(ctx, doer, nil),
		Page:       page,
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) IssueChangeLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue,
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

	permission, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, issue.Poster)
	if issue.IsPull {
		if err = issue.LoadPullRequest(ctx); err != nil {
			log.Error("loadPullRequest: %v", err)
			return
		}
		if err = issue.PullRequest.LoadIssue(ctx); err != nil {
			log.Error("LoadIssue: %v", err)
			return
		}
		err = PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventPullRequestLabel, &api.PullRequestPayload{
			Action:      api.HookIssueLabelUpdated,
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, doer),
			Repository:  convert.ToRepo(ctx, issue.Repo, access_model.Permission{AccessMode: perm.AccessModeOwner}),
			Sender:      convert.ToUser(ctx, doer, nil),
		})
	} else {
		err = PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventIssueLabel, &api.IssuePayload{
			Action:     api.HookIssueLabelUpdated,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(ctx, doer, issue),
			Repository: convert.ToRepo(ctx, issue.Repo, permission),
			Sender:     convert.ToUser(ctx, doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) IssueChangeMilestone(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldMilestoneID int64) {
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

	permission, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, doer)
	if issue.IsPull {
		err = issue.PullRequest.LoadIssue(ctx)
		if err != nil {
			log.Error("LoadIssue: %v", err)
			return
		}
		err = PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventPullRequestMilestone, &api.PullRequestPayload{
			Action:      hookAction,
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, doer),
			Repository:  convert.ToRepo(ctx, issue.Repo, permission),
			Sender:      convert.ToUser(ctx, doer, nil),
		})
	} else {
		err = PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventIssueMilestone, &api.IssuePayload{
			Action:     hookAction,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(ctx, doer, issue),
			Repository: convert.ToRepo(ctx, issue.Repo, permission),
			Sender:     convert.ToUser(ctx, doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) PushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	apiPusher := convert.ToUser(ctx, pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(ctx, repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	if err := PrepareWebhooks(ctx, EventSource{Repository: repo}, webhook_module.HookEventPush, &api.PushPayload{
		Ref:          opts.RefFullName.String(),
		Before:       opts.OldCommitID,
		After:        opts.NewCommitID,
		CompareURL:   setting.AppURL + commits.CompareURL,
		Commits:      apiCommits,
		TotalCommits: commits.Len,
		HeadCommit:   apiHeadCommit,
		Repo:         convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}),
		Pusher:       apiPusher,
		Sender:       apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) AutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	// just redirect to the MergePullRequest
	m.MergePullRequest(ctx, doer, pr)
}

func (*webhookNotifier) MergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
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

	permission, err := access_model.GetUserRepoPermission(ctx, pr.Issue.Repo, doer)
	if err != nil {
		log.Error("models.GetUserRepoPermission: %v", err)
		return
	}

	// Merge pull request calls issue.changeStatus so we need to handle separately.
	apiPullRequest := &api.PullRequestPayload{
		Index:       pr.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(ctx, pr, doer),
		Repository:  convert.ToRepo(ctx, pr.Issue.Repo, permission),
		Sender:      convert.ToUser(ctx, doer, nil),
		Action:      api.HookIssueClosed,
	}

	if err := PrepareWebhooks(ctx, EventSource{Repository: pr.Issue.Repo}, webhook_module.HookEventPullRequest, apiPullRequest); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) PullRequestChangeTargetBranch(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, oldBranch string) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}

	issue := pr.Issue

	mode, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, issue.Poster)
	if err := PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventPullRequest, &api.PullRequestPayload{
		Action: api.HookIssueEdited,
		Index:  issue.Index,
		Changes: &api.ChangesPayload{
			Ref: &api.ChangesFromPayload{
				From: oldBranch,
			},
		},
		PullRequest: convert.ToAPIPullRequest(ctx, pr, doer),
		Repository:  convert.ToRepo(ctx, issue.Repo, mode),
		Sender:      convert.ToUser(ctx, doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [pr: %d]: %v", pr.ID, err)
	}
}

func (m *webhookNotifier) PullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	var reviewHookType webhook_module.HookEventType

	switch review.Type {
	case issues_model.ReviewTypeApprove:
		reviewHookType = webhook_module.HookEventPullRequestReviewApproved
	case issues_model.ReviewTypeComment:
		reviewHookType = webhook_module.HookEventPullRequestReviewComment
	case issues_model.ReviewTypeReject:
		reviewHookType = webhook_module.HookEventPullRequestReviewRejected
	default:
		// unsupported review webhook type here
		log.Error("Unsupported review webhook type")
		return
	}

	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}

	permission, err := access_model.GetUserRepoPermission(ctx, review.Issue.Repo, review.Issue.Poster)
	if err != nil {
		log.Error("models.GetUserRepoPermission: %v", err)
		return
	}
	if err := PrepareWebhooks(ctx, EventSource{Repository: review.Issue.Repo}, reviewHookType, &api.PullRequestPayload{
		Action:            api.HookIssueReviewed,
		Index:             review.Issue.Index,
		PullRequest:       convert.ToAPIPullRequest(ctx, pr, review.Reviewer),
		RequestedReviewer: convert.ToUser(ctx, review.Reviewer, nil),
		Repository:        convert.ToRepo(ctx, review.Issue.Repo, permission),
		Sender:            convert.ToUser(ctx, review.Reviewer, nil),
		Review: &api.ReviewPayload{
			Type:    string(reviewHookType),
			Content: review.Content,
		},
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) PullRequestReviewRequest(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment) {
	if !issue.IsPull {
		log.Warn("PullRequestReviewRequest: issue is not a pull request: %v", issue.ID)
		return
	}
	permission, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, doer)
	if err := issue.LoadPullRequest(ctx); err != nil {
		log.Error("LoadPullRequest failed: %v", err)
		return
	}
	apiPullRequest := &api.PullRequestPayload{
		Index:             issue.Index,
		PullRequest:       convert.ToAPIPullRequest(ctx, issue.PullRequest, doer),
		RequestedReviewer: convert.ToUser(ctx, reviewer, nil),
		Repository:        convert.ToRepo(ctx, issue.Repo, permission),
		Sender:            convert.ToUser(ctx, doer, nil),
	}
	if isRequest {
		apiPullRequest.Action = api.HookIssueReviewRequested
	} else {
		apiPullRequest.Action = api.HookIssueReviewRequestRemoved
	}
	if err := PrepareWebhooks(ctx, EventSource{Repository: issue.Repo}, webhook_module.HookEventPullRequestReviewRequest, apiPullRequest); err != nil {
		log.Error("PrepareWebhooks [review_requested: %v]: %v", isRequest, err)
		return
	}
}

func (m *webhookNotifier) CreateRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
	apiPusher := convert.ToUser(ctx, pusher, nil)
	apiRepo := convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeNone})
	if err := PrepareWebhooks(ctx, EventSource{Repository: repo}, webhook_module.HookEventCreate, &api.CreatePayload{
		Ref:     refFullName.ShortName(), // FIXME: should it be a full ref name? But it will break the existing webhooks?
		Sha:     refID,
		RefType: string(refFullName.RefType()),
		Repo:    apiRepo,
		Sender:  apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) PullRequestSynchronized(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}
	if err := pr.Issue.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := PrepareWebhooks(ctx, EventSource{Repository: pr.Issue.Repo}, webhook_module.HookEventPullRequestSync, &api.PullRequestPayload{
		Action:      api.HookIssueSynchronized,
		Index:       pr.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(ctx, pr, doer),
		Repository:  convert.ToRepo(ctx, pr.Issue.Repo, access_model.Permission{AccessMode: perm.AccessModeOwner}),
		Sender:      convert.ToUser(ctx, doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [pull_id: %v]: %v", pr.ID, err)
	}
}

func (m *webhookNotifier) DeleteRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	apiPusher := convert.ToUser(ctx, pusher, nil)
	apiRepo := convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner})
	if err := PrepareWebhooks(ctx, EventSource{Repository: repo}, webhook_module.HookEventDelete, &api.DeletePayload{
		Ref:        refFullName.ShortName(), // FIXME: should it be a full ref name? But it will break the existing webhooks?
		RefType:    string(refFullName.RefType()),
		PusherType: api.PusherTypeUser,
		Repo:       apiRepo,
		Sender:     apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks.(delete %s): %v", refFullName.RefType(), err)
	}
}

func sendReleaseHook(ctx context.Context, doer *user_model.User, rel *repo_model.Release, action api.HookReleaseAction) {
	if err := rel.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	permission, _ := access_model.GetUserRepoPermission(ctx, rel.Repo, doer)
	if err := PrepareWebhooks(ctx, EventSource{Repository: rel.Repo}, webhook_module.HookEventRelease, &api.ReleasePayload{
		Action:     action,
		Release:    convert.ToAPIRelease(ctx, rel.Repo, rel),
		Repository: convert.ToRepo(ctx, rel.Repo, permission),
		Sender:     convert.ToUser(ctx, doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NewRelease(ctx context.Context, rel *repo_model.Release) {
	sendReleaseHook(ctx, rel.Publisher, rel, api.HookReleasePublished)
}

func (m *webhookNotifier) UpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	sendReleaseHook(ctx, doer, rel, api.HookReleaseUpdated)
}

func (m *webhookNotifier) DeleteRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	sendReleaseHook(ctx, doer, rel, api.HookReleaseDeleted)
}

func (m *webhookNotifier) SyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	apiPusher := convert.ToUser(ctx, pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(ctx, repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	if err := PrepareWebhooks(ctx, EventSource{Repository: repo}, webhook_module.HookEventPush, &api.PushPayload{
		Ref:          opts.RefFullName.String(),
		Before:       opts.OldCommitID,
		After:        opts.NewCommitID,
		CompareURL:   setting.AppURL + commits.CompareURL,
		Commits:      apiCommits,
		TotalCommits: commits.Len,
		HeadCommit:   apiHeadCommit,
		Repo:         convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}),
		Pusher:       apiPusher,
		Sender:       apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) CreateCommitStatus(ctx context.Context, repo *repo_model.Repository, commit *repository.PushCommit, sender *user_model.User, status *git_model.CommitStatus) {
	apiSender := convert.ToUser(ctx, sender, nil)
	apiCommit, err := repository.ToAPIPayloadCommit(ctx, map[string]*user_model.User{}, repo.RepoPath(), repo.HTMLURL(), commit)
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	// as a webhook url, target should be an absolute url. But for internal actions target url
	// the target url is a url path with no host and port to make it easy to be visited
	// from multiple hosts. So we need to convert it to an absolute url here.
	target := httplib.MakeAbsoluteURL(ctx, status.TargetURL)

	payload := api.CommitStatusPayload{
		Context:     status.Context,
		CreatedAt:   status.CreatedUnix.AsTime().UTC(),
		Description: status.Description,
		ID:          status.ID,
		SHA:         commit.Sha1,
		State:       status.State.String(),
		TargetURL:   target,

		Commit: apiCommit,
		Repo:   convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}),
		Sender: apiSender,
	}
	if !status.UpdatedUnix.IsZero() {
		t := status.UpdatedUnix.AsTime().UTC()
		payload.UpdatedAt = &t
	}
	if err := PrepareWebhooks(ctx, EventSource{Repository: repo}, webhook_module.HookEventStatus, &payload); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) SyncCreateRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
	m.CreateRef(ctx, pusher, repo, refFullName, refID)
}

func (m *webhookNotifier) SyncDeleteRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	m.DeleteRef(ctx, pusher, repo, refFullName)
}

func (m *webhookNotifier) PackageCreate(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
	notifyPackage(ctx, doer, pd, api.HookPackageCreated)
}

func (m *webhookNotifier) PackageDelete(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
	notifyPackage(ctx, doer, pd, api.HookPackageDeleted)
}

func notifyPackage(ctx context.Context, sender *user_model.User, pd *packages_model.PackageDescriptor, action api.HookPackageAction) {
	source := EventSource{
		Repository: pd.Repository,
		Owner:      pd.Owner,
	}

	apiPackage, err := convert.ToPackage(ctx, pd, sender)
	if err != nil {
		log.Error("Error converting package: %v", err)
		return
	}

	var org *api.Organization
	if pd.Owner.IsOrganization() {
		org = convert.ToOrganization(ctx, organization.OrgFromUser(pd.Owner))
	}

	if err := PrepareWebhooks(ctx, source, webhook_module.HookEventPackage, &api.PackagePayload{
		Action:       action,
		Package:      apiPackage,
		Organization: org,
		Sender:       convert.ToUser(ctx, sender, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (*webhookNotifier) WorkflowJobStatusUpdate(ctx context.Context, repo *repo_model.Repository, sender *user_model.User, job *actions_model.ActionRunJob, task *actions_model.ActionTask) {
	source := EventSource{
		Repository: repo,
		Owner:      repo.Owner,
	}

	var org *api.Organization
	if repo.Owner.IsOrganization() {
		org = convert.ToOrganization(ctx, organization.OrgFromUser(repo.Owner))
	}

	err := job.LoadAttributes(ctx)
	if err != nil {
		log.Error("Error loading job attributes: %v", err)
		return
	}

	jobIndex := 0
	jobs, err := actions_model.GetRunJobsByRunID(ctx, job.RunID)
	if err != nil {
		log.Error("Error loading getting run jobs: %v", err)
		return
	}
	for i, j := range jobs {
		if j.ID == job.ID {
			jobIndex = i
			break
		}
	}

	status, conclusion := toActionStatus(job.Status)
	var runnerID int64
	var runnerName string
	var steps []*api.ActionWorkflowStep

	if task != nil {
		runnerID = task.RunnerID
		if runner, ok, _ := db.GetByID[actions_model.ActionRunner](ctx, runnerID); ok {
			runnerName = runner.Name
		}
		for i, step := range task.Steps {
			stepStatus, stepConclusion := toActionStatus(job.Status)
			steps = append(steps, &api.ActionWorkflowStep{
				Name:        step.Name,
				Number:      int64(i),
				Status:      stepStatus,
				Conclusion:  stepConclusion,
				StartedAt:   step.Started.AsTime().UTC(),
				CompletedAt: step.Stopped.AsTime().UTC(),
			})
		}
	}

	if err := PrepareWebhooks(ctx, source, webhook_module.HookEventWorkflowJob, &api.WorkflowJobPayload{
		Action: status,
		WorkflowJob: &api.ActionWorkflowJob{
			ID: job.ID,
			// missing api endpoint for this location
			URL:     fmt.Sprintf("%s/actions/runs/%d/jobs/%d", repo.APIURL(), job.RunID, job.ID),
			HTMLURL: fmt.Sprintf("%s/jobs/%d", job.Run.HTMLURL(), jobIndex),
			RunID:   job.RunID,
			// Missing api endpoint for this location, artifacts are available under a nested url
			RunURL:      fmt.Sprintf("%s/actions/runs/%d", repo.APIURL(), job.RunID),
			Name:        job.Name,
			Labels:      job.RunsOn,
			RunAttempt:  job.Attempt,
			HeadSha:     job.Run.CommitSHA,
			HeadBranch:  git.RefName(job.Run.Ref).BranchName(),
			Status:      status,
			Conclusion:  conclusion,
			RunnerID:    runnerID,
			RunnerName:  runnerName,
			Steps:       steps,
			CreatedAt:   job.Created.AsTime().UTC(),
			StartedAt:   job.Started.AsTime().UTC(),
			CompletedAt: job.Stopped.AsTime().UTC(),
		},
		Organization: org,
		Repo:         convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}),
		Sender:       convert.ToUser(ctx, sender, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func toActionStatus(status actions_model.Status) (string, string) {
	var action string
	var conclusion string
	switch status {
	// This is a naming conflict of the webhook between Gitea and GitHub Actions
	case actions_model.StatusWaiting:
		action = "queued"
	case actions_model.StatusBlocked:
		action = "waiting"
	case actions_model.StatusRunning:
		action = "in_progress"
	}
	if status.IsDone() {
		action = "completed"
		switch status {
		case actions_model.StatusSuccess:
			conclusion = "success"
		case actions_model.StatusCancelled:
			conclusion = "cancelled"
		case actions_model.StatusFailure:
			conclusion = "failure"
		}
	}
	return action, conclusion
}
