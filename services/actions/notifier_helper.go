// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	issues_model "code.gitea.io/gitea/models/issues"
	packages_model "code.gitea.io/gitea/models/packages"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	actions_module "code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/services/convert"

	"github.com/nektos/act/pkg/jobparser"
)

var methodCtxKey struct{}

// withMethod sets the notification method that this context currently executes.
// Used for debugging/ troubleshooting purposes.
func withMethod(ctx context.Context, method string) context.Context {
	// don't overwrite
	if v := ctx.Value(methodCtxKey); v != nil {
		if _, ok := v.(string); ok {
			return ctx
		}
	}
	return context.WithValue(ctx, methodCtxKey, method)
}

// getMethod gets the notification method that this context currently executes.
// Default: "notify"
// Used for debugging/ troubleshooting purposes.
func getMethod(ctx context.Context) string {
	if v := ctx.Value(methodCtxKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return "notify"
}

type notifyInput struct {
	// required
	Repo  *repo_model.Repository
	Doer  *user_model.User
	Event webhook_module.HookEventType

	// optional
	Ref         string
	Payload     api.Payloader
	PullRequest *issues_model.PullRequest
}

func newNotifyInput(repo *repo_model.Repository, doer *user_model.User, event webhook_module.HookEventType) *notifyInput {
	return &notifyInput{
		Repo:  repo,
		Doer:  doer,
		Event: event,
	}
}

func (input *notifyInput) WithDoer(doer *user_model.User) *notifyInput {
	input.Doer = doer
	return input
}

func (input *notifyInput) WithRef(ref string) *notifyInput {
	input.Ref = ref
	return input
}

func (input *notifyInput) WithPayload(payload api.Payloader) *notifyInput {
	input.Payload = payload
	return input
}

func (input *notifyInput) WithPullRequest(pr *issues_model.PullRequest) *notifyInput {
	input.PullRequest = pr
	if input.Ref == "" {
		input.Ref = pr.GetGitRefName()
	}
	return input
}

func (input *notifyInput) Notify(ctx context.Context) {
	log.Trace("execute %v for event %v whose doer is %v", getMethod(ctx), input.Event, input.Doer.Name)

	if err := notify(ctx, input); err != nil {
		log.Error("an error occurred while executing the %s actions method: %v", getMethod(ctx), err)
	}
}

func notify(ctx context.Context, input *notifyInput) error {
	if input.Doer.IsActions() {
		// avoiding triggering cyclically, for example:
		// a comment of an issue will trigger the runner to add a new comment as reply,
		// and the new comment will trigger the runner again.
		log.Debug("ignore executing %v for event %v whose doer is %v", getMethod(ctx), input.Event, input.Doer.Name)
		return nil
	}
	if unit_model.TypeActions.UnitGlobalDisabled() {
		return nil
	}
	if err := input.Repo.LoadUnits(ctx); err != nil {
		return fmt.Errorf("repo.LoadUnits: %w", err)
	} else if !input.Repo.UnitEnabled(ctx, unit_model.TypeActions) {
		return nil
	}

	gitRepo, err := git.OpenRepository(context.Background(), input.Repo.RepoPath())
	if err != nil {
		return fmt.Errorf("git.OpenRepository: %w", err)
	}
	defer gitRepo.Close()

	ref := input.Ref
	if input.Event == webhook_module.HookEventDelete {
		// The event is deleting a reference, so it will fail to get the commit for a deleted reference.
		// Set ref to empty string to fall back to the default branch.
		ref = ""
	}
	if ref == "" {
		ref = input.Repo.DefaultBranch
	}

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(ref)
	if err != nil {
		return fmt.Errorf("gitRepo.GetCommit: %w", err)
	}

	workflows, err := actions_module.DetectWorkflows(commit, input.Event, input.Payload)
	if err != nil {
		return fmt.Errorf("DetectWorkflows: %w", err)
	}

	if len(workflows) == 0 {
		log.Trace("repo %s with commit %s couldn't find workflows", input.Repo.RepoPath(), commit.ID)
		return nil
	}

	p, err := json.Marshal(input.Payload)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	isForkPullRequest := false
	if pr := input.PullRequest; pr != nil {
		switch pr.Flow {
		case issues_model.PullRequestFlowGithub:
			isForkPullRequest = pr.IsFromFork()
		case issues_model.PullRequestFlowAGit:
			// There is no fork concept in agit flow, anyone with read permission can push refs/for/<target-branch>/<topic-branch> to the repo.
			// So we can treat it as a fork pull request because it may be from an untrusted user
			isForkPullRequest = true
		default:
			// unknown flow, assume it's a fork pull request to be safe
			isForkPullRequest = true
		}
	}

	for id, content := range workflows {
		run := &actions_model.ActionRun{
			Title:             strings.SplitN(commit.CommitMessage, "\n", 2)[0],
			RepoID:            input.Repo.ID,
			OwnerID:           input.Repo.OwnerID,
			WorkflowID:        id,
			TriggerUserID:     input.Doer.ID,
			Ref:               ref,
			CommitSHA:         commit.ID.String(),
			IsForkPullRequest: isForkPullRequest,
			Event:             input.Event,
			EventPayload:      string(p),
			Status:            actions_model.StatusWaiting,
		}
		if need, err := ifNeedApproval(ctx, run, input.Repo, input.Doer); err != nil {
			log.Error("check if need approval for repo %d with user %d: %v", input.Repo.ID, input.Doer.ID, err)
			continue
		} else {
			run.NeedApproval = need
		}

		jobs, err := jobparser.Parse(content)
		if err != nil {
			log.Error("jobparser.Parse: %v", err)
			continue
		}
		if err := actions_model.InsertRun(ctx, run, jobs); err != nil {
			log.Error("InsertRun: %v", err)
			continue
		}
		if jobs, _, err := actions_model.FindRunJobs(ctx, actions_model.FindRunJobOptions{RunID: run.ID}); err != nil {
			log.Error("FindRunJobs: %v", err)
		} else {
			CreateCommitStatus(ctx, jobs...)
		}

	}
	return nil
}

func newNotifyInputFromIssue(issue *issues_model.Issue, event webhook_module.HookEventType) *notifyInput {
	return newNotifyInput(issue.Repo, issue.Poster, event)
}

func notifyRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release, action api.HookReleaseAction) {
	if err := rel.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	permission, _ := access_model.GetUserRepoPermission(ctx, rel.Repo, doer)

	newNotifyInput(rel.Repo, doer, webhook_module.HookEventRelease).
		WithRef(git.RefNameFromTag(rel.TagName).String()).
		WithPayload(&api.ReleasePayload{
			Action:     action,
			Release:    convert.ToAPIRelease(ctx, rel.Repo, rel),
			Repository: convert.ToRepo(ctx, rel.Repo, permission),
			Sender:     convert.ToUser(ctx, doer, nil),
		}).
		Notify(ctx)
}

func notifyPackage(ctx context.Context, sender *user_model.User, pd *packages_model.PackageDescriptor, action api.HookPackageAction) {
	if pd.Repository == nil {
		// When a package is uploaded to an organization, it could trigger an event to notify.
		// So the repository could be nil, however, actions can't support that yet.
		// See https://github.com/go-gitea/gitea/pull/17940
		return
	}

	apiPackage, err := convert.ToPackage(ctx, pd, sender)
	if err != nil {
		log.Error("Error converting package: %v", err)
		return
	}

	newNotifyInput(pd.Repository, sender, webhook_module.HookEventPackage).
		WithPayload(&api.PackagePayload{
			Action:  action,
			Package: apiPackage,
			Sender:  convert.ToUser(ctx, sender, nil),
		}).
		Notify(ctx)
}

func ifNeedApproval(ctx context.Context, run *actions_model.ActionRun, repo *repo_model.Repository, user *user_model.User) (bool, error) {
	// don't need approval if it's not a fork PR
	if !run.IsForkPullRequest {
		return false, nil
	}

	// always need approval if the user is restricted
	if user.IsRestricted {
		log.Trace("need approval because user %d is restricted", user.ID)
		return true, nil
	}

	// don't need approval if the user can write
	if perm, err := access_model.GetUserRepoPermission(ctx, repo, user); err != nil {
		return false, fmt.Errorf("GetUserRepoPermission: %w", err)
	} else if perm.CanWrite(unit_model.TypeActions) {
		log.Trace("do not need approval because user %d can write", user.ID)
		return false, nil
	}

	// don't need approval if the user has been approved before
	if count, err := actions_model.CountRuns(ctx, actions_model.FindRunOptions{
		RepoID:        repo.ID,
		TriggerUserID: user.ID,
		Approved:      true,
	}); err != nil {
		return false, fmt.Errorf("CountRuns: %w", err)
	} else if count > 0 {
		log.Trace("do not need approval because user %d has been approved before", user.ID)
		return false, nil
	}

	// otherwise, need approval
	log.Trace("need approval because it's the first time user %d triggered actions", user.ID)
	return true, nil
}
