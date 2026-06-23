// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/actions"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/git"
	"gitea.dev/modules/reqctx"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"

	"gitea.com/gitea/runner/act/model"
	"go.yaml.in/yaml/v4"
)

func EnableOrDisableWorkflow(ctx *context.APIContext, workflowID string, isEnable bool) error {
	workflow, err := convert.GetActionWorkflow(ctx, ctx.Repo.GitRepo, ctx.Repo.Repository, workflowID)
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

	return repo_model.UpdateRepoUnitConfig(ctx, cfgUnit)
}

// DispatchActionWorkflow manually triggers a workflow_dispatch run.
// scopedWorkflowSourceRepoID selects the workflow source: 0 means a repo-level workflow in this repo; a non-zero value is the source repo of a scoped workflow.
func DispatchActionWorkflow(ctx reqctx.RequestContext, doer *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, workflowID, ref string, scopedWorkflowSourceRepoID int64, processInputs func(model *model.WorkflowDispatch, inputs map[string]any) error) (runID int64, _ error) {
	if workflowID == "" {
		return 0, util.ErrorWrapTranslatable(
			util.NewNotExistErrorf("workflowID is empty"),
			"actions.workflow.not_found", workflowID,
		)
	}

	if ref == "" {
		return 0, util.ErrorWrapTranslatable(
			util.NewNotExistErrorf("ref is empty"),
			"form.target_ref_not_exist", ref,
		)
	}

	isScoped := scopedWorkflowSourceRepoID > 0

	// can not run when the workflow is disabled (opt-out is keyed by source repo for scoped workflows)
	cfgUnit := repo.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()
	workflowDisabled := cfg.IsWorkflowDisabled(workflowID)
	if isScoped {
		workflowDisabled = cfg.IsScopedWorkflowDisabled(scopedWorkflowSourceRepoID, workflowID)
	}
	if workflowDisabled {
		return 0, util.ErrorWrapTranslatable(
			util.NewPermissionDeniedErrorf("workflow is disabled"),
			"actions.workflow.disabled",
		)
	}

	// get target commit of run from specified ref
	refName := git.RefName(ref)
	var runTargetCommit *git.Commit
	var err error
	if refName.IsTag() {
		runTargetCommit, err = gitRepo.GetTagCommit(refName.TagName())
	} else if refName.IsBranch() {
		runTargetCommit, err = gitRepo.GetBranchCommit(refName.BranchName())
	} else {
		refName = git.RefNameFromBranch(ref)
		runTargetCommit, err = gitRepo.GetBranchCommit(ref)
	}
	if err != nil {
		return 0, util.ErrorWrapTranslatable(
			util.NewNotExistErrorf("ref %q doesn't exist", ref),
			"form.target_ref_not_exist", ref,
		)
	}

	run := &actions_model.ActionRun{
		Title:             runTargetCommit.MessageTitle(),
		RepoID:            repo.ID,
		Repo:              repo,
		OwnerID:           repo.OwnerID,
		WorkflowID:        workflowID,
		TriggerUserID:     doer.ID,
		TriggerUser:       doer,
		Ref:               string(refName),
		CommitSHA:         runTargetCommit.ID.String(),
		IsForkPullRequest: false,
		Event:             "workflow_dispatch",
		TriggerEvent:      "workflow_dispatch",
		Status:            actions_model.StatusWaiting,
		// local dispatch: own repo at the target commit; the scoped path overrides these below
		WorkflowRepoID:    repo.ID,
		WorkflowCommitSHA: runTargetCommit.ID.String(),
	}

	// resolve the workflow content and record its source on the run (scoped runs read from the source repo)
	content, err := resolveDispatchWorkflowContent(ctx, repo, runTargetCommit, workflowID, scopedWorkflowSourceRepoID, isScoped, run)
	if err != nil {
		return 0, err
	}

	singleWorkflow := &jobparser.SingleWorkflow{}
	if err := yaml.Unmarshal(content, singleWorkflow); err != nil {
		return 0, fmt.Errorf("failed to unmarshal workflow content: %w", err)
	}
	// get inputs from post
	workflow := &model.Workflow{
		RawOn: singleWorkflow.RawOn,
	}
	workflowDispatch := workflow.WorkflowDispatchConfig()
	if workflowDispatch == nil {
		return 0, util.ErrorWrapTranslatable(
			util.NewInvalidArgumentErrorf("workflow %q has no workflow_dispatch event trigger", workflowID),
			"actions.workflow.has_no_workflow_dispatch", workflowID,
		)
	}

	inputsWithDefaults := make(map[string]any)
	if err = processInputs(workflowDispatch, inputsWithDefaults); err != nil {
		return 0, err
	}

	// ctx.Req.PostForm -> WorkflowDispatchPayload.Inputs -> ActionRun.EventPayload -> runner: ghc.Event
	// https://docs.github.com/en/actions/learn-github-actions/contexts#github-context
	// https://docs.github.com/en/webhooks/webhook-events-and-payloads#workflow_dispatch
	workflowDispatchPayload := &api.WorkflowDispatchPayload{
		Workflow:   workflowID,
		Ref:        ref,
		Repository: convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeNone}),
		Inputs:     inputsWithDefaults,
		Sender:     convert.ToUserWithAccessMode(ctx, doer, perm.AccessModeNone),
	}

	var eventPayload []byte
	if eventPayload, err = workflowDispatchPayload.JSONPayload(); err != nil {
		return 0, fmt.Errorf("JSONPayload: %w", err)
	}
	run.EventPayload = string(eventPayload)

	// Insert the action run and its associated jobs into the database
	if err := PrepareRunAndInsert(ctx, content, run, inputsWithDefaults); err != nil {
		return 0, fmt.Errorf("PrepareRun: %w", err)
	}
	return run.ID, nil
}

// resolveDispatchWorkflowContent returns the YAML for a dispatched workflow and records its source on the run.
//   - Repo-level: from the consumer's runTargetCommit.
//   - Scoped: from the source repo's default branch.
func resolveDispatchWorkflowContent(ctx reqctx.RequestContext, repo *repo_model.Repository, runTargetCommit *git.Commit, workflowID string, sourceRepoID int64, isScoped bool, run *actions_model.ActionRun) ([]byte, error) {
	if isScoped {
		return resolveScopedDispatchContent(ctx, repo, sourceRepoID, workflowID, run)
	}

	_, entries, err := actions.ListWorkflows(runTargetCommit)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Name() == workflowID {
			return actions.GetContentFromEntry(e)
		}
	}
	return nil, util.ErrorWrapTranslatable(
		util.NewNotExistErrorf("workflow %q doesn't exist", workflowID),
		"actions.workflow.not_found", workflowID,
	)
}

func resolveScopedDispatchContent(ctx reqctx.RequestContext, repo *repo_model.Repository, sourceRepoID int64, workflowID string, run *actions_model.ActionRun) ([]byte, error) {
	// the source must be an effective scoped source for this consumer repo
	effective, err := actions_model.IsScopedWorkflowSourceEffective(ctx, repo.OwnerID, sourceRepoID)
	if err != nil {
		return nil, err
	}
	if !effective {
		return nil, util.ErrorWrapTranslatable(
			util.NewNotExistErrorf("scoped workflow source %d is not effective for this repository", sourceRepoID),
			"actions.workflow.not_found", workflowID,
		)
	}

	sourceRepo, err := repo_model.GetRepositoryByID(ctx, sourceRepoID)
	if err != nil {
		return nil, err
	}

	sha, parsed, err := LoadParsedScopedWorkflows(ctx, sourceRepo)
	if err != nil {
		return nil, err
	}
	for _, p := range parsed {
		if p.EntryName == workflowID {
			run.WorkflowRepoID = sourceRepo.ID
			run.WorkflowCommitSHA = sha
			run.IsScopedRun = true
			return p.Content, nil
		}
	}
	return nil, util.ErrorWrapTranslatable(
		util.NewNotExistErrorf("scoped workflow %q doesn't exist", workflowID),
		"actions.workflow.not_found", workflowID,
	)
}
