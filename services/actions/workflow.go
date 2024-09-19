// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"

	"github.com/nektos/act/pkg/jobparser"
	"github.com/nektos/act/pkg/model"
)

func getActionWorkflowEntry(ctx *context.APIContext, entry *git.TreeEntry, commit *git.Commit) (*api.ActionWorkflow, error) {
	cfgUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()

	URL := fmt.Sprintf("%s/actions/workflows/%s", ctx.Repo.Repository.APIURL(), entry.Name())
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

	// TODO: NodeID
	// TODO: CreatedAt
	// TODO: UpdatedAt
	// TODO: HTMLURL
	// TODO: DeletedAt

	return &api.ActionWorkflow{
		ID:       entry.Name(),
		Name:     entry.Name(),
		Path:     entry.Name(),
		State:    state,
		URL:      URL,
		BadgeURL: badgeURL,
	}, nil
}

func disableOrEnableWorkflow(ctx *context.APIContext, workflowID string, isEnable bool) error {
	cfgUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()

	if isEnable {
		cfg.EnableWorkflow(workflowID)
	} else {
		cfg.DisableWorkflow(workflowID)
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

	workflows := make([]*api.ActionWorkflow, len(entries))
	for i, entry := range entries {
		workflows[i], err = getActionWorkflowEntry(ctx, entry, defaultBranchCommit)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "WorkflowGetError", err.Error())
			return nil, err
		}
	}

	return workflows, nil
}

func GetActionWorkflow(ctx *context.APIContext, workflowID string) (*api.ActionWorkflow, error) {
	entries, err := ListActionWorkflows(ctx)
	if err != nil {
		return nil, err
	}

	workflows := make([]*api.ActionWorkflow, len(entries))
	for i, entry := range entries {
		if entry.Name == workflowID {
			workflows[i] = entry
			break
		}
	}

	return workflows[len(workflows)-1], nil
}

func DisableActionWorkflow(ctx *context.APIContext, workflowID string) error {
	return disableOrEnableWorkflow(ctx, workflowID, false)
}

func DispatchActionWorkflow(ctx *context.APIContext, workflowID string, opt *api.CreateActionWorkflowDispatch) {
	// can not run job when workflow is disabled
	cfgUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()
	if cfg.IsWorkflowDisabled(workflowID) {
		ctx.Error(http.StatusInternalServerError, "WorkflowDisabled", ctx.Tr("actions.workflow.disabled"))
		return
	}

	// get target commit of run from specified ref
	refName := git.RefName(opt.Ref)
	var runTargetCommit *git.Commit
	var err error
	if refName.IsTag() {
		runTargetCommit, err = ctx.Repo.GitRepo.GetTagCommit(refName.TagName())
	} else if refName.IsBranch() {
		runTargetCommit, err = ctx.Repo.GitRepo.GetBranchCommit(refName.BranchName())
	} else {
		ctx.Error(http.StatusInternalServerError, "WorkflowRefNameError", ctx.Tr("form.git_ref_name_error", opt.Ref))
		return
	}
	if err != nil {
		ctx.Error(http.StatusNotFound, "WorkflowRefNotFound", ctx.Tr("form.target_ref_not_exist", opt.Ref))
		return
	}

	// get workflow entry from default branch commit
	defaultBranchCommit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "WorkflowDefaultBranchError", err.Error())
		return
	}
	entries, err := actions.ListWorkflows(defaultBranchCommit)
	if err != nil {
		ctx.Error(http.StatusNotFound, "WorkflowListNotFound", err.Error())
	}

	// find workflow from commit
	var workflows []*jobparser.SingleWorkflow
	for _, entry := range entries {
		if entry.Name() == workflowID {
			content, err := actions.GetContentFromEntry(entry)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "WorkflowGetContentError", err.Error())
				return
			}
			workflows, err = jobparser.Parse(content)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "WorkflowParseError", err.Error())
				return
			}
			break
		}
	}

	if len(workflows) == 0 {
		ctx.Error(http.StatusNotFound, "WorkflowNotFound", ctx.Tr("actions.workflow.not_found", workflowID))
		return
	}

	workflow := &model.Workflow{
		RawOn: workflows[0].RawOn,
	}
	inputs := make(map[string]any)
	if workflowDispatch := workflow.WorkflowDispatchConfig(); workflowDispatch != nil {
		for name, config := range workflowDispatch.Inputs {
			value, exists := opt.Inputs[name]
			if !exists {
				continue
			}
			if config.Type == "boolean" {
				inputs[name] = strconv.FormatBool(value == "on")
			} else if value != "" {
				inputs[name] = value
			} else {
				inputs[name] = config.Default
			}
		}
	}

	workflowDispatchPayload := &api.WorkflowDispatchPayload{
		Workflow:   workflowID,
		Ref:        opt.Ref,
		Repository: convert.ToRepo(ctx, ctx.Repo.Repository, access_model.Permission{AccessMode: perm.AccessModeNone}),
		Inputs:     inputs,
		Sender:     convert.ToUserWithAccessMode(ctx, ctx.Doer, perm.AccessModeNone),
	}
	var eventPayload []byte
	if eventPayload, err = workflowDispatchPayload.JSONPayload(); err != nil {
		ctx.Error(http.StatusInternalServerError, "WorkflowDispatchJSONParseError", err.Error())
		return
	}

	run := &actions_model.ActionRun{
		Title:             strings.SplitN(runTargetCommit.CommitMessage, "\n", 2)[0],
		RepoID:            ctx.Repo.Repository.ID,
		OwnerID:           ctx.Repo.Repository.Owner.ID,
		WorkflowID:        workflowID,
		TriggerUserID:     ctx.Doer.ID,
		Ref:               opt.Ref,
		CommitSHA:         runTargetCommit.ID.String(),
		IsForkPullRequest: false,
		Event:             "workflow_dispatch",
		TriggerEvent:      "workflow_dispatch",
		EventPayload:      string(eventPayload),
		Status:            actions_model.StatusWaiting,
	}

	if err := actions_model.InsertRun(ctx, run, workflows); err != nil {
		ctx.Error(http.StatusInternalServerError, "WorkflowInsertRunError", err.Error())
		return
	}

	alljobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{RunID: run.ID})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "WorkflowFindRunJobError", err.Error())
		return
	}
	CreateCommitStatus(ctx, alljobs...)
}

func EnableActionWorkflow(ctx *context.APIContext, workflowID string) error {
	return disableOrEnableWorkflow(ctx, workflowID, true)
}
