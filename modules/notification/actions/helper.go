// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	packages_model "code.gitea.io/gitea/models/packages"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	actions_module "code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	actions_service "code.gitea.io/gitea/services/actions"

	"github.com/nektos/act/pkg/jobparser"
)

var methodCtxKey struct{}

func withMethod(ctx context.Context, method string) context.Context {
	// don't overwrite
	if v := ctx.Value(methodCtxKey); v != nil {
		if _, ok := v.(string); ok {
			return ctx
		}
	}
	return context.WithValue(ctx, methodCtxKey, method)
}

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
	Event webhook.HookEventType

	// optional
	Ref         string
	Payload     api.Payloader
	PullRequest *issues_model.PullRequest
}

func newNotifyInput(repo *repo_model.Repository, doer *user_model.User, event webhook.HookEventType) *notifyInput {
	return &notifyInput{
		Repo:  repo,
		Ref:   repo.DefaultBranch,
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
	return input
}

func (input *notifyInput) Notify(ctx context.Context) {
	if err := notify(ctx, input); err != nil {
		log.Error("%s: %v", getMethod(ctx), err)
	}
}

func notify(ctx context.Context, input *notifyInput) error {
	if unit.TypeActions.UnitGlobalDisabled() {
		return nil
	}
	if err := input.Repo.LoadUnits(db.DefaultContext); err != nil {
		return fmt.Errorf("repo.LoadUnits: %w", err)
	} else if !input.Repo.UnitEnabled(ctx, unit.TypeActions) {
		return nil
	}

	gitRepo, err := git.OpenRepository(context.Background(), input.Repo.RepoPath())
	if err != nil {
		return fmt.Errorf("git.OpenRepository: %w", err)
	}
	defer gitRepo.Close()

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(input.Ref)
	if err != nil {
		return fmt.Errorf("gitRepo.GetCommit: %v", err)
	}

	workflows, err := actions_module.DetectWorkflows(commit, input.Event)
	if err != nil {
		return fmt.Errorf("DetectWorkflows: %v", err)
	}

	if len(workflows) == 0 {
		log.Trace("repo %s with commit %s couldn't find workflows", input.Repo.RepoPath(), commit.ID)
		return nil
	}

	p, err := json.Marshal(input.Payload)
	if err != nil {
		return fmt.Errorf("json.Marshal: %v", err)
	}

	for id, content := range workflows {
		run := actions_model.ActionRun{
			Title:             truncateContent(strings.SplitN(commit.CommitMessage, "\n", 2)[0], 255),
			RepoID:            input.Repo.ID,
			OwnerID:           input.Repo.OwnerID,
			WorkflowID:        id,
			TriggerUserID:     input.Doer.ID,
			Ref:               input.Ref,
			CommitSHA:         commit.ID.String(),
			IsForkPullRequest: input.PullRequest != nil && input.PullRequest.IsFromFork(),
			Event:             input.Event,
			EventPayload:      string(p),
			Status:            actions_model.StatusWaiting,
		}
		jobs, err := jobparser.Parse(content)
		if err != nil {
			log.Error("jobparser.Parse: %v", err)
			continue
		}
		if err := actions_model.InsertRun(ctx, &run, jobs); err != nil {
			log.Error("InsertRun: %v", err)
			continue
		}
		if jobs, _, err := actions_model.FindRunJobs(ctx, actions_model.FindRunJobOptions{RunID: run.ID}); err != nil {
			log.Error("FindRunJobs: %v", err)
		} else {
			for _, job := range jobs {
				if err := actions_service.CreateCommitStatus(ctx, job); err != nil {
					log.Error("CreateCommitStatus: %v", err)
				}
			}
		}

	}
	return nil
}

func newNotifyInputFromIssue(issue *issues_model.Issue, event webhook.HookEventType) *notifyInput {
	return newNotifyInput(issue.Repo, issue.Poster, event)
}

func notifyRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release, ref string, action api.HookReleaseAction) {
	if err := rel.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(ctx, doer, rel.Repo)

	newNotifyInput(rel.Repo, doer, webhook.HookEventRelease).
		WithRef(ref).
		WithPayload(&api.ReleasePayload{
			Action:     action,
			Release:    convert.ToRelease(rel),
			Repository: convert.ToRepo(ctx, rel.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		}).
		Notify(ctx)
}

func notifyPackage(ctx context.Context, sender *user_model.User, pd *packages_model.PackageDescriptor, action api.HookPackageAction) {
	if pd.Repository == nil {
		// TODO https://github.com/go-gitea/gitea/pull/17940
		return
	}

	apiPackage, err := convert.ToPackage(ctx, pd, sender)
	if err != nil {
		log.Error("Error converting package: %v", err)
		return
	}

	newNotifyInput(pd.Repository, sender, webhook.HookEventPackage).
		WithPayload(&api.PackagePayload{
			Action:  action,
			Package: apiPackage,
			Sender:  convert.ToUser(sender, nil),
		}).
		Notify(ctx)
}

func truncateContent(content string, n int) string {
	truncatedContent, truncatedRight := util.SplitStringAtByteN(content, n)
	if truncatedRight != "" {
		// in case the content is in a Latin family language, we remove the last broken word.
		lastSpaceIdx := strings.LastIndex(truncatedContent, " ")
		if lastSpaceIdx != -1 && (len(truncatedContent)-lastSpaceIdx < 15) {
			truncatedContent = truncatedContent[:lastSpaceIdx] + "…"
		}
	}
	return truncatedContent
}
