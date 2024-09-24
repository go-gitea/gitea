// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"net/http"
	"os"
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

func getActionWorkflowPath(commit *git.Commit) string {
	paths := []string{".gitea/workflows", ".github/workflows"}
	for _, path := range paths {
		if _, err := commit.SubTree(path); err == nil {
			return path
		}
	}
	return ""
}

func getActionWorkflowEntry(ctx *context.APIContext, commit *git.Commit, entry *git.TreeEntry) *api.ActionWorkflow {
	cfgUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()

	defaultBranch, _ := commit.GetBranchName()

	URL := fmt.Sprintf("%s/actions/workflows/%s", ctx.Repo.Repository.APIURL(), entry.Name())
	HTMLURL := fmt.Sprintf("%s/src/branch/%s/%s/%s", ctx.Repo.Repository.HTMLURL(ctx), defaultBranch, getActionWorkflowPath(commit), entry.Name())
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

	// Currently, the NodeID returns the hostname of the server since, as far as I know, Gitea does not have a parameter
	// similar to an instance ID.
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
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
		NodeID:    hostname,
		Name:      entry.Name(),
		Path:      entry.Name(),
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

	workflows := make([]*api.ActionWorkflow, len(entries))
	for i, entry := range entries {
		workflows[i] = getActionWorkflowEntry(ctx, defaultBranchCommit, entry)
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

	return nil, fmt.Errorf("workflow not found")
}

func DisableActionWorkflow(ctx *context.APIContext, workflowID string) error {
	return disableOrEnableWorkflow(ctx, workflowID, false)
}

func DispatchActionWorkflow(ctx *context.APIContext, workflowID string, opt *api.CreateActionWorkflowDispatch) {
	cfgUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()

	if cfg.IsWorkflowDisabled(workflowID) {
		ctx.Error(http.StatusInternalServerError, "WorkflowDisabled", ctx.Tr("actions.workflow.disabled"))
		return
	}

	refName := git.RefName(opt.Ref)
	var runTargetCommit *git.Commit
	var err error

	switch {
	case refName.IsTag():
		runTargetCommit, err = ctx.Repo.GitRepo.GetTagCommit(refName.TagName())
	case refName.IsBranch():
		runTargetCommit, err = ctx.Repo.GitRepo.GetBranchCommit(refName.BranchName())
	default:
		ctx.Error(http.StatusInternalServerError, "WorkflowRefNameError", ctx.Tr("form.git_ref_name_error", opt.Ref))
		return
	}

	if err != nil {
		ctx.Error(http.StatusNotFound, "WorkflowRefNotFound", ctx.Tr("form.target_ref_not_exist", opt.Ref))
		return
	}

	defaultBranchCommit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "WorkflowDefaultBranchError", err.Error())
		return
	}

	entries, err := actions.ListWorkflows(defaultBranchCommit)
	if err != nil {
		ctx.Error(http.StatusNotFound, "WorkflowListNotFound", err.Error())
		return
	}

	var workflow *jobparser.SingleWorkflow
	for _, entry := range entries {
		if entry.Name() == workflowID {
			content, err := actions.GetContentFromEntry(entry)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "WorkflowGetContentError", err.Error())
				return
			}
			workflows, err := jobparser.Parse(content)
			if err != nil || len(workflows) == 0 {
				ctx.Error(http.StatusInternalServerError, "WorkflowParseError", err.Error())
				return
			}
			workflow = workflows[0]
			break
		}
	}

	if workflow == nil {
		ctx.Error(http.StatusNotFound, "WorkflowNotFound", ctx.Tr("actions.workflow.not_found", workflowID))
		return
	}

	// Process workflow inputs
	inputs := processWorkflowInputs(opt, &model.Workflow{
		RawOn: workflow.RawOn,
	})

	workflowDispatchPayload := &api.WorkflowDispatchPayload{
		Workflow:   workflowID,
		Ref:        opt.Ref,
		Repository: convert.ToRepo(ctx, ctx.Repo.Repository, access_model.Permission{AccessMode: perm.AccessModeNone}),
		Inputs:     inputs,
		Sender:     convert.ToUserWithAccessMode(ctx, ctx.Doer, perm.AccessModeNone),
	}

	eventPayload, err := workflowDispatchPayload.JSONPayload()
	if err != nil {
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

	if err := actions_model.InsertRun(ctx, run, []*jobparser.SingleWorkflow{workflow}); err != nil {
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

func processWorkflowInputs(opt *api.CreateActionWorkflowDispatch, workflow *model.Workflow) map[string]any {
	inputs := make(map[string]any)
	if workflowDispatch := workflow.WorkflowDispatchConfig(); workflowDispatch != nil {
		for name, config := range workflowDispatch.Inputs {
			value, exists := opt.Inputs[name]
			if !exists {
				continue
			}
			if value == "" {
				value = config.Default
			}
			switch config.Type {
			case "boolean":
				inputs[name] = strconv.FormatBool(value == "on")
			default:
				inputs[name] = value
			}
		}
	}
	return inputs
}

func EnableActionWorkflow(ctx *context.APIContext, workflowID string) error {
	return disableOrEnableWorkflow(ctx, workflowID, true)
}
