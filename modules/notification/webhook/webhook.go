// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
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
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/process"
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

func (m *webhookNotifier) NotifyIssueClearLabels(doer *user_model.User, issue *issues_model.Issue) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifyIssueClearLabels User: %s[%d] Issue[%d] #%d in [%d]", doer.Name, doer.ID, issue.ID, issue.Index, issue.RepoID))
	defer finished()

	if err := issue.LoadPoster(); err != nil {
		log.Error("loadPoster: %v", err)
		return
	}

	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(); err != nil {
			log.Error("LoadPullRequest: %v", err)
			return
		}

		err = webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventPullRequestLabel, &api.PullRequestPayload{
			Action:      api.HookIssueLabelCleared,
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
			Repository:  convert.ToRepo(issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		})
	} else {
		err = webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventIssueLabel, &api.IssuePayload{
			Action:     api.HookIssueLabelCleared,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyForkRepository(doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	oldMode, _ := access_model.AccessLevel(doer, oldRepo)
	mode, _ := access_model.AccessLevel(doer, repo)

	// forked webhook
	if err := webhook_services.PrepareWebhooks(oldRepo, webhook.HookEventFork, &api.ForkPayload{
		Forkee: convert.ToRepo(oldRepo, oldMode),
		Repo:   convert.ToRepo(repo, mode),
		Sender: convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", oldRepo.ID, err)
	}

	u := repo.MustOwner()

	// Add to hook queue for created repo after session commit.
	if u.IsOrganization() {
		if err := webhook_services.PrepareWebhooks(repo, webhook.HookEventRepository, &api.RepositoryPayload{
			Action:       api.HookRepoCreated,
			Repository:   convert.ToRepo(repo, perm.AccessModeOwner),
			Organization: convert.ToUser(u, nil),
			Sender:       convert.ToUser(doer, nil),
		}); err != nil {
			log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
		}
	}
}

func (m *webhookNotifier) NotifyCreateRepository(doer, u *user_model.User, repo *repo_model.Repository) {
	// Add to hook queue for created repo after session commit.
	if err := webhook_services.PrepareWebhooks(repo, webhook.HookEventRepository, &api.RepositoryPayload{
		Action:       api.HookRepoCreated,
		Repository:   convert.ToRepo(repo, perm.AccessModeOwner),
		Organization: convert.ToUser(u, nil),
		Sender:       convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) NotifyDeleteRepository(doer *user_model.User, repo *repo_model.Repository) {
	u := repo.MustOwner()

	if err := webhook_services.PrepareWebhooks(repo, webhook.HookEventRepository, &api.RepositoryPayload{
		Action:       api.HookRepoDeleted,
		Repository:   convert.ToRepo(repo, perm.AccessModeOwner),
		Organization: convert.ToUser(u, nil),
		Sender:       convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) NotifyMigrateRepository(doer, u *user_model.User, repo *repo_model.Repository) {
	// Add to hook queue for created repo after session commit.
	if err := webhook_services.PrepareWebhooks(repo, webhook.HookEventRepository, &api.RepositoryPayload{
		Action:       api.HookRepoCreated,
		Repository:   convert.ToRepo(repo, perm.AccessModeOwner),
		Organization: convert.ToUser(u, nil),
		Sender:       convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) NotifyIssueChangeAssignee(doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifyIssueChangeAssignee User: %s[%d] Issue[%d] #%d in [%d] Assignee %s[%d] removed: %t", doer.Name, doer.ID, issue.ID, issue.Index, issue.RepoID, assignee.Name, assignee.ID, removed))
	defer finished()

	if issue.IsPull {
		mode, _ := access_model.AccessLevelUnit(doer, issue.Repo, unit.TypePullRequests)

		if err := issue.LoadPullRequest(); err != nil {
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
		if err := webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventPullRequestAssign, apiPullRequest); err != nil {
			log.Error("PrepareWebhooks [is_pull: %v, remove_assignee: %v]: %v", issue.IsPull, removed, err)
			return
		}
	} else {
		mode, _ := access_model.AccessLevelUnit(doer, issue.Repo, unit.TypeIssues)
		apiIssue := &api.IssuePayload{
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		}
		if removed {
			apiIssue.Action = api.HookIssueUnassigned
		} else {
			apiIssue.Action = api.HookIssueAssigned
		}
		// Assignee comment triggers a webhook
		if err := webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventIssueAssign, apiIssue); err != nil {
			log.Error("PrepareWebhooks [is_pull: %v, remove_assignee: %v]: %v", issue.IsPull, removed, err)
			return
		}
	}
}

func (m *webhookNotifier) NotifyIssueChangeTitle(doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifyIssueChangeTitle User: %s[%d] Issue[%d] #%d in [%d]", doer.Name, doer.ID, issue.ID, issue.Index, issue.RepoID))
	defer finished()

	mode, _ := access_model.AccessLevel(issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(); err != nil {
			log.Error("LoadPullRequest failed: %v", err)
			return
		}
		issue.PullRequest.Issue = issue
		err = webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventPullRequest, &api.PullRequestPayload{
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
		err = webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventIssues, &api.IssuePayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Title: &api.ChangesFromPayload{
					From: oldTitle,
				},
			},
			Issue:      convert.ToAPIIssue(issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		})
	}

	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyIssueChangeStatus(doer *user_model.User, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifyIssueChangeStatus User: %s[%d] Issue[%d] #%d in [%d]", doer.Name, doer.ID, issue.ID, issue.Index, issue.RepoID))
	defer finished()

	mode, _ := access_model.AccessLevel(issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(); err != nil {
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
		err = webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventPullRequest, apiPullRequest)
	} else {
		apiIssue := &api.IssuePayload{
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		}
		if isClosed {
			apiIssue.Action = api.HookIssueClosed
		} else {
			apiIssue.Action = api.HookIssueReOpened
		}
		err = webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventIssues, apiIssue)
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v, is_closed: %v]: %v", issue.IsPull, isClosed, err)
	}
}

func (m *webhookNotifier) NotifyNewIssue(issue *issues_model.Issue, mentions []*user_model.User) {
	if err := issue.LoadRepo(db.DefaultContext); err != nil {
		log.Error("issue.LoadRepo: %v", err)
		return
	}
	if err := issue.LoadPoster(); err != nil {
		log.Error("issue.LoadPoster: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(issue.Poster, issue.Repo)
	if err := webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventIssues, &api.IssuePayload{
		Action:     api.HookIssueOpened,
		Index:      issue.Index,
		Issue:      convert.ToAPIIssue(issue),
		Repository: convert.ToRepo(issue.Repo, mode),
		Sender:     convert.ToUser(issue.Poster, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyNewPullRequest(pull *issues_model.PullRequest, mentions []*user_model.User) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifyNewPullRequest Pull[%d] #%d in [%d]", pull.ID, pull.Index, pull.BaseRepoID))
	defer finished()

	if err := pull.LoadIssue(); err != nil {
		log.Error("pull.LoadIssue: %v", err)
		return
	}
	if err := pull.Issue.LoadRepo(ctx); err != nil {
		log.Error("pull.Issue.LoadRepo: %v", err)
		return
	}
	if err := pull.Issue.LoadPoster(); err != nil {
		log.Error("pull.Issue.LoadPoster: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(pull.Issue.Poster, pull.Issue.Repo)
	if err := webhook_services.PrepareWebhooks(pull.Issue.Repo, webhook.HookEventPullRequest, &api.PullRequestPayload{
		Action:      api.HookIssueOpened,
		Index:       pull.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(ctx, pull, nil),
		Repository:  convert.ToRepo(pull.Issue.Repo, mode),
		Sender:      convert.ToUser(pull.Issue.Poster, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyIssueChangeContent(doer *user_model.User, issue *issues_model.Issue, oldContent string) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifyIssueChangeContent User: %s[%d] Issue[%d] #%d in [%d]", doer.Name, doer.ID, issue.ID, issue.Index, issue.RepoID))
	defer finished()

	mode, _ := access_model.AccessLevel(issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		issue.PullRequest.Issue = issue
		err = webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventPullRequest, &api.PullRequestPayload{
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
		err = webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventIssues, &api.IssuePayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Body: &api.ChangesFromPayload{
					From: oldContent,
				},
			},
			Issue:      convert.ToAPIIssue(issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyUpdateComment(doer *user_model.User, c *issues_model.Comment, oldContent string) {
	var err error

	if err = c.LoadPoster(); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}
	if err = c.LoadIssue(); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}

	if err = c.Issue.LoadAttributes(db.DefaultContext); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(doer, c.Issue.Repo)
	if c.Issue.IsPull {
		err = webhook_services.PrepareWebhooks(c.Issue.Repo, webhook.HookEventPullRequestComment, &api.IssueCommentPayload{
			Action:  api.HookIssueCommentEdited,
			Issue:   convert.ToAPIIssue(c.Issue),
			Comment: convert.ToComment(c),
			Changes: &api.ChangesPayload{
				Body: &api.ChangesFromPayload{
					From: oldContent,
				},
			},
			Repository: convert.ToRepo(c.Issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
			IsPull:     true,
		})
	} else {
		err = webhook_services.PrepareWebhooks(c.Issue.Repo, webhook.HookEventIssueComment, &api.IssueCommentPayload{
			Action:  api.HookIssueCommentEdited,
			Issue:   convert.ToAPIIssue(c.Issue),
			Comment: convert.ToComment(c),
			Changes: &api.ChangesPayload{
				Body: &api.ChangesFromPayload{
					From: oldContent,
				},
			},
			Repository: convert.ToRepo(c.Issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
			IsPull:     false,
		})
	}

	if err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", c.ID, err)
	}
}

func (m *webhookNotifier) NotifyCreateIssueComment(doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User,
) {
	mode, _ := access_model.AccessLevel(doer, repo)

	var err error
	if issue.IsPull {
		err = webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventPullRequestComment, &api.IssueCommentPayload{
			Action:     api.HookIssueCommentCreated,
			Issue:      convert.ToAPIIssue(issue),
			Comment:    convert.ToComment(comment),
			Repository: convert.ToRepo(repo, mode),
			Sender:     convert.ToUser(doer, nil),
			IsPull:     true,
		})
	} else {
		err = webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventIssueComment, &api.IssueCommentPayload{
			Action:     api.HookIssueCommentCreated,
			Issue:      convert.ToAPIIssue(issue),
			Comment:    convert.ToComment(comment),
			Repository: convert.ToRepo(repo, mode),
			Sender:     convert.ToUser(doer, nil),
			IsPull:     false,
		})
	}

	if err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	}
}

func (m *webhookNotifier) NotifyDeleteComment(doer *user_model.User, comment *issues_model.Comment) {
	var err error

	if err = comment.LoadPoster(); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}
	if err = comment.LoadIssue(); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}

	if err = comment.Issue.LoadAttributes(db.DefaultContext); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(doer, comment.Issue.Repo)

	if comment.Issue.IsPull {
		err = webhook_services.PrepareWebhooks(comment.Issue.Repo, webhook.HookEventPullRequestComment, &api.IssueCommentPayload{
			Action:     api.HookIssueCommentDeleted,
			Issue:      convert.ToAPIIssue(comment.Issue),
			Comment:    convert.ToComment(comment),
			Repository: convert.ToRepo(comment.Issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
			IsPull:     true,
		})
	} else {
		err = webhook_services.PrepareWebhooks(comment.Issue.Repo, webhook.HookEventIssueComment, &api.IssueCommentPayload{
			Action:     api.HookIssueCommentDeleted,
			Issue:      convert.ToAPIIssue(comment.Issue),
			Comment:    convert.ToComment(comment),
			Repository: convert.ToRepo(comment.Issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
			IsPull:     false,
		})
	}

	if err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	}
}

func (m *webhookNotifier) NotifyIssueChangeLabels(doer *user_model.User, issue *issues_model.Issue,
	addedLabels, removedLabels []*issues_model.Label,
) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifyIssueChangeLabels User: %s[%d] Issue[%d] #%d in [%d]", doer.Name, doer.ID, issue.ID, issue.Index, issue.RepoID))
	defer finished()

	var err error

	if err = issue.LoadRepo(ctx); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}

	if err = issue.LoadPoster(); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(issue.Poster, issue.Repo)
	if issue.IsPull {
		if err = issue.LoadPullRequest(); err != nil {
			log.Error("loadPullRequest: %v", err)
			return
		}
		if err = issue.PullRequest.LoadIssue(); err != nil {
			log.Error("LoadIssue: %v", err)
			return
		}
		err = webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventPullRequestLabel, &api.PullRequestPayload{
			Action:      api.HookIssueLabelUpdated,
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
			Repository:  convert.ToRepo(issue.Repo, perm.AccessModeNone),
			Sender:      convert.ToUser(doer, nil),
		})
	} else {
		err = webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventIssueLabel, &api.IssuePayload{
			Action:     api.HookIssueLabelUpdated,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyIssueChangeMilestone(doer *user_model.User, issue *issues_model.Issue, oldMilestoneID int64) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifyIssueChangeMilestone User: %s[%d] Issue[%d] #%d in [%d]", doer.Name, doer.ID, issue.ID, issue.Index, issue.RepoID))
	defer finished()

	var hookAction api.HookIssueAction
	var err error
	if issue.MilestoneID > 0 {
		hookAction = api.HookIssueMilestoned
	} else {
		hookAction = api.HookIssueDemilestoned
	}

	if err = issue.LoadAttributes(db.DefaultContext); err != nil {
		log.Error("issue.LoadAttributes failed: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(doer, issue.Repo)
	if issue.IsPull {
		err = issue.PullRequest.LoadIssue()
		if err != nil {
			log.Error("LoadIssue: %v", err)
			return
		}
		err = webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventPullRequestMilestone, &api.PullRequestPayload{
			Action:      hookAction,
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
			Repository:  convert.ToRepo(issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		})
	} else {
		err = webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventIssueMilestone, &api.IssuePayload{
			Action:     hookAction,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyPushCommits(pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifyPushCommits User: %s[%d] in %s[%d]", pusher.Name, pusher.ID, repo.FullName(), repo.ID))
	defer finished()

	apiPusher := convert.ToUser(pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(ctx, repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	if err := webhook_services.PrepareWebhooks(repo, webhook.HookEventPush, &api.PushPayload{
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

func (*webhookNotifier) NotifyMergePullRequest(pr *issues_model.PullRequest, doer *user_model.User) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifyMergePullRequest Pull[%d] #%d in [%d]", pr.ID, pr.Index, pr.BaseRepoID))
	defer finished()

	// Reload pull request information.
	if err := pr.LoadAttributes(); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := pr.LoadIssue(); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := pr.Issue.LoadRepo(ctx); err != nil {
		log.Error("pr.Issue.LoadRepo: %v", err)
		return
	}

	mode, err := access_model.AccessLevel(doer, pr.Issue.Repo)
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

	err = webhook_services.PrepareWebhooks(pr.Issue.Repo, webhook.HookEventPullRequest, apiPullRequest)
	if err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyPullRequestChangeTargetBranch(doer *user_model.User, pr *issues_model.PullRequest, oldBranch string) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifyPullRequestChangeTargetBranch Pull[%d] #%d in [%d]", pr.ID, pr.Index, pr.BaseRepoID))
	defer finished()

	issue := pr.Issue
	if !issue.IsPull {
		return
	}
	var err error

	if err = issue.LoadPullRequest(); err != nil {
		log.Error("LoadPullRequest failed: %v", err)
		return
	}
	issue.PullRequest.Issue = issue
	mode, _ := access_model.AccessLevel(issue.Poster, issue.Repo)
	err = webhook_services.PrepareWebhooks(issue.Repo, webhook.HookEventPullRequest, &api.PullRequestPayload{
		Action: api.HookIssueEdited,
		Index:  issue.Index,
		Changes: &api.ChangesPayload{
			Ref: &api.ChangesFromPayload{
				From: oldBranch,
			},
		},
		PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
		Repository:  convert.ToRepo(issue.Repo, mode),
		Sender:      convert.ToUser(doer, nil),
	})

	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyPullRequestReview(pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifyPullRequestReview Pull[%d] #%d in [%d]", pr.ID, pr.Index, pr.BaseRepoID))
	defer finished()

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

	if err := pr.LoadIssue(); err != nil {
		log.Error("pr.LoadIssue: %v", err)
		return
	}

	mode, err := access_model.AccessLevel(review.Issue.Poster, review.Issue.Repo)
	if err != nil {
		log.Error("models.AccessLevel: %v", err)
		return
	}
	if err := webhook_services.PrepareWebhooks(review.Issue.Repo, reviewHookType, &api.PullRequestPayload{
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

func (m *webhookNotifier) NotifyCreateRef(pusher *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
	apiPusher := convert.ToUser(pusher, nil)
	apiRepo := convert.ToRepo(repo, perm.AccessModeNone)
	refName := git.RefEndName(refFullName)

	if err := webhook_services.PrepareWebhooks(repo, webhook.HookEventCreate, &api.CreatePayload{
		Ref:     refName,
		Sha:     refID,
		RefType: refType,
		Repo:    apiRepo,
		Sender:  apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyPullRequestSynchronized(doer *user_model.User, pr *issues_model.PullRequest) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifyPullRequestSynchronized Pull[%d] #%d in [%d]", pr.ID, pr.Index, pr.BaseRepoID))
	defer finished()

	if err := pr.LoadIssue(); err != nil {
		log.Error("pr.LoadIssue: %v", err)
		return
	}
	if err := pr.Issue.LoadAttributes(db.DefaultContext); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := webhook_services.PrepareWebhooks(pr.Issue.Repo, webhook.HookEventPullRequestSync, &api.PullRequestPayload{
		Action:      api.HookIssueSynchronized,
		Index:       pr.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(ctx, pr, nil),
		Repository:  convert.ToRepo(pr.Issue.Repo, perm.AccessModeNone),
		Sender:      convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [pull_id: %v]: %v", pr.ID, err)
	}
}

func (m *webhookNotifier) NotifyDeleteRef(pusher *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
	apiPusher := convert.ToUser(pusher, nil)
	apiRepo := convert.ToRepo(repo, perm.AccessModeNone)
	refName := git.RefEndName(refFullName)

	if err := webhook_services.PrepareWebhooks(repo, webhook.HookEventDelete, &api.DeletePayload{
		Ref:        refName,
		RefType:    refType,
		PusherType: api.PusherTypeUser,
		Repo:       apiRepo,
		Sender:     apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks.(delete %s): %v", refType, err)
	}
}

func sendReleaseHook(doer *user_model.User, rel *models.Release, action api.HookReleaseAction) {
	if err := rel.LoadAttributes(); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(doer, rel.Repo)
	if err := webhook_services.PrepareWebhooks(rel.Repo, webhook.HookEventRelease, &api.ReleasePayload{
		Action:     action,
		Release:    convert.ToRelease(rel),
		Repository: convert.ToRepo(rel.Repo, mode),
		Sender:     convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyNewRelease(rel *models.Release) {
	sendReleaseHook(rel.Publisher, rel, api.HookReleasePublished)
}

func (m *webhookNotifier) NotifyUpdateRelease(doer *user_model.User, rel *models.Release) {
	sendReleaseHook(doer, rel, api.HookReleaseUpdated)
}

func (m *webhookNotifier) NotifyDeleteRelease(doer *user_model.User, rel *models.Release) {
	sendReleaseHook(doer, rel, api.HookReleaseDeleted)
}

func (m *webhookNotifier) NotifySyncPushCommits(pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifySyncPushCommits User: %s[%d] in %s[%d]", pusher.Name, pusher.ID, repo.FullName(), repo.ID))
	defer finished()

	apiPusher := convert.ToUser(pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(ctx, repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	if err := webhook_services.PrepareWebhooks(repo, webhook.HookEventPush, &api.PushPayload{
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

func (m *webhookNotifier) NotifySyncCreateRef(pusher *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
	m.NotifyCreateRef(pusher, repo, refType, refFullName, refID)
}

func (m *webhookNotifier) NotifySyncDeleteRef(pusher *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
	m.NotifyDeleteRef(pusher, repo, refType, refFullName)
}

func (m *webhookNotifier) NotifyPackageCreate(doer *user_model.User, pd *packages_model.PackageDescriptor) {
	notifyPackage(doer, pd, api.HookPackageCreated)
}

func (m *webhookNotifier) NotifyPackageDelete(doer *user_model.User, pd *packages_model.PackageDescriptor) {
	notifyPackage(doer, pd, api.HookPackageDeleted)
}

func notifyPackage(sender *user_model.User, pd *packages_model.PackageDescriptor, action api.HookPackageAction) {
	if pd.Repository == nil {
		// TODO https://github.com/go-gitea/gitea/pull/17940
		return
	}

	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.notifyPackage Package: %s[%d]", pd.Package.Name, pd.Package.ID))
	defer finished()

	apiPackage, err := convert.ToPackage(ctx, pd, sender)
	if err != nil {
		log.Error("Error converting package: %v", err)
		return
	}

	if err := webhook_services.PrepareWebhooks(pd.Repository, webhook.HookEventPackage, &api.PackagePayload{
		Action:  action,
		Package: apiPackage,
		Sender:  convert.ToUser(sender, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}
