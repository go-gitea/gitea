// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"

	"github.com/nektos/act/pkg/jobparser"
	"github.com/nektos/act/pkg/model"
)

type TranslateableError struct {
	Translation string
	Args        []any
	Code        int
}

func (t TranslateableError) Error() string {
	return t.Translation
}

func (t TranslateableError) GetCode() int {
	if t.Code == 0 {
		return http.StatusInternalServerError
	}
	return t.Code
}

func getActionWorkflowPath(commit *git.Commit) string {
	paths := []string{".gitea/workflows", ".github/workflows"}
	for _, path := range paths {
		if _, err := commit.SubTree(path); err == nil {
			return path
		}
	}
	return ""
}

func getActionWorkflowEntry(ctx *context.APIContext, commit *git.Commit, folder string, entry *git.TreeEntry) *api.ActionWorkflow {
	cfgUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()

	defaultBranch, _ := commit.GetBranchName()

	URL := fmt.Sprintf("%s/actions/workflows/%s", ctx.Repo.Repository.APIURL(), entry.Name())
	HTMLURL := fmt.Sprintf("%s/src/branch/%s/%s/%s", ctx.Repo.Repository.HTMLURL(ctx), defaultBranch, folder, entry.Name())
	badgeURL := fmt.Sprintf("%s/actions/workflows/%s/badge.svg?branch=%s", ctx.Repo.Repository.HTMLURL(ctx), entry.Name(), ctx.Repo.Repository.DefaultBranch)

	// See https://docs.github.com/en/rest/actions/workflows?apiVersion=2022-11-28#get-a-workflow
	// State types:
	// - active
	// - deleted
	// - disabled_fork
	// - disabled_inactivity
	// - disabled_manually
	state := "active"
	if cfg.IsWorkflowDisabled(entry.Name()) {
		state = "disabled_manually"
	}

	// The CreatedAt and UpdatedAt fields currently reflect the timestamp of the latest commit, which can later be refined
	// by retrieving the first and last commits for the file history. The first commit would indicate the creation date,
	// while the last commit would represent the modification date. The DeletedAt could be determined by identifying
	// the last commit where the file existed. However, this implementation has not been done here yet, as it would likely
	// cause a significant performance degradation.
	createdAt := commit.Author.When
	updatedAt := commit.Author.When

	return &api.ActionWorkflow{
		ID:        entry.Name(),
		Name:      entry.Name(),
		Path:      path.Join(folder, entry.Name()),
		State:     state,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		URL:       URL,
		HTMLURL:   HTMLURL,
		BadgeURL:  badgeURL,
	}
}

func disableOrEnableWorkflow(ctx *context.APIContext, workflowID string, isEnable bool) error {
	workflow, err := GetActionWorkflow(ctx, workflowID)
	if err != nil {
		return err
	}

	cfgUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()

	if isEnable {
		cfg.EnableWorkflow(workflow.ID)
	} else {
		cfg.DisableWorkflow(workflow.ID)
	}

	return repo_model.UpdateRepoUnit(ctx, cfgUnit)
}

func ListActionWorkflows(ctx *context.APIContext) ([]*api.ActionWorkflow, error) {
	defaultBranchCommit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "WorkflowDefaultBranchError", err.Error())
		return nil, err
	}

	entries, err := actions.ListWorkflows(defaultBranchCommit)
	if err != nil {
		ctx.Error(http.StatusNotFound, "WorkflowListNotFound", err.Error())
		return nil, err
	}

	folder := getActionWorkflowPath(defaultBranchCommit)

	workflows := make([]*api.ActionWorkflow, len(entries))
	for i, entry := range entries {
		workflows[i] = getActionWorkflowEntry(ctx, defaultBranchCommit, folder, entry)
	}

	return workflows, nil
}

func GetActionWorkflow(ctx *context.APIContext, workflowID string) (*api.ActionWorkflow, error) {
	entries, err := ListActionWorkflows(ctx)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.Name == workflowID {
			return entry, nil
		}
	}

	return nil, fmt.Errorf("workflow '%s' not found", workflowID)
}

func DisableActionWorkflow(ctx *context.APIContext, workflowID string) error {
	return disableOrEnableWorkflow(ctx, workflowID, false)
}

func DispatchActionWorkflow(ctx *context.Context, workflowID, ref string, processInputs func(model *model.WorkflowDispatch, inputs *map[string]any) error) error {
	if len(workflowID) == 0 {
		return fmt.Errorf("workflowID is empty")
	}

	if len(ref) == 0 {
		return fmt.Errorf("ref is empty")
	}

	// can not rerun job when workflow is disabled
	cfgUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()
	if cfg.IsWorkflowDisabled(workflowID) {
		return &TranslateableError{
			Translation: "actions.workflow.disabled",
		}
	}

	// get target commit of run from specified ref
	refName := git.RefName(ref)
	var runTargetCommit *git.Commit
	var err error
	if refName.IsTag() {
		runTargetCommit, err = ctx.Repo.GitRepo.GetTagCommit(refName.TagName())
	} else if refName.IsBranch() {
		runTargetCommit, err = ctx.Repo.GitRepo.GetBranchCommit(refName.BranchName())
	} else {
		refName = git.RefNameFromBranch(ref)
		runTargetCommit, err = ctx.Repo.GitRepo.GetBranchCommit(ref)
	}
	if err != nil {
		return &TranslateableError{
			Code:        http.StatusNotFound,
			Translation: "form.target_ref_not_exist",
			Args:        []any{ref},
		}
	}

	// get workflow entry from runTargetCommit
	entries, err := actions.ListWorkflows(runTargetCommit)
	if err != nil {
		return err
	}

	// find workflow from commit
	var workflows []*jobparser.SingleWorkflow
	for _, entry := range entries {
		if entry.Name() != workflowID {
			continue
		}

		content, err := actions.GetContentFromEntry(entry)
		if err != nil {
			return err
		}
		workflows, err = jobparser.Parse(content)
		if err != nil {
			return err
		}
		break
	}

	if len(workflows) == 0 {
		return &TranslateableError{
			Code:        http.StatusNotFound,
			Translation: "actions.workflow.not_found",
			Args:        []any{workflowID},
		}
	}

	// get inputs from post
	workflow := &model.Workflow{
		RawOn: workflows[0].RawOn,
	}
	inputsWithDefaults := make(map[string]any)
	workflowDispatch := workflow.WorkflowDispatchConfig()
	if err := processInputs(workflowDispatch, &inputsWithDefaults); err != nil {
		return err
	}

	// ctx.Req.PostForm -> WorkflowDispatchPayload.Inputs -> ActionRun.EventPayload -> runner: ghc.Event
	// https://docs.github.com/en/actions/learn-github-actions/contexts#github-context
	// https://docs.github.com/en/webhooks/webhook-events-and-payloads#workflow_dispatch
	workflowDispatchPayload := &api.WorkflowDispatchPayload{
		Workflow:   workflowID,
		Ref:        ref,
		Repository: convert.ToRepo(ctx, ctx.Repo.Repository, access_model.Permission{AccessMode: perm.AccessModeNone}),
		Inputs:     inputsWithDefaults,
		Sender:     convert.ToUserWithAccessMode(ctx, ctx.Doer, perm.AccessModeNone),
	}
	var eventPayload []byte
	if eventPayload, err = workflowDispatchPayload.JSONPayload(); err != nil {
		return fmt.Errorf("JSONPayload: %w", err)
	}

	run := &actions_model.ActionRun{
		Title:             strings.SplitN(runTargetCommit.CommitMessage, "\n", 2)[0],
		RepoID:            ctx.Repo.Repository.ID,
		OwnerID:           ctx.Repo.Repository.OwnerID,
		WorkflowID:        workflowID,
		TriggerUserID:     ctx.Doer.ID,
		Ref:               string(refName),
		CommitSHA:         runTargetCommit.ID.String(),
		IsForkPullRequest: false,
		Event:             "workflow_dispatch",
		TriggerEvent:      "workflow_dispatch",
		EventPayload:      string(eventPayload),
		Status:            actions_model.StatusWaiting,
	}

	// cancel running jobs of the same workflow
	if err := actions_model.CancelPreviousJobs(
		ctx,
		run.RepoID,
		run.Ref,
		run.WorkflowID,
		run.Event,
	); err != nil {
		log.Error("CancelRunningJobs: %v", err)
	}

	// Insert the action run and its associated jobs into the database
	if err := actions_model.InsertRun(ctx, run, workflows); err != nil {
		return fmt.Errorf("workflow: %w", err)
	}

	alljobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{RunID: run.ID})
	if err != nil {
		log.Error("FindRunJobs: %v", err)
	}
	CreateCommitStatus(ctx, alljobs...)

	return nil
}

func EnableActionWorkflow(ctx *context.APIContext, workflowID string) error {
	return disableOrEnableWorkflow(ctx, workflowID, true)
}
