// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"bytes"
	stdCtx "context"
	"net/http"
	"slices"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"

	"github.com/nektos/act/pkg/model"
	"gopkg.in/yaml.v3"
)

const (
	tplListActions           templates.TplName = "repo/actions/list"
	tplDispatchInputsActions templates.TplName = "repo/actions/workflow_dispatch_inputs"
	tplViewActions           templates.TplName = "repo/actions/view"
)

type Workflow struct {
	Entry  git.TreeEntry
	ErrMsg string
}

// MustEnableActions check if actions are enabled in settings
func MustEnableActions(ctx *context.Context) {
	if !setting.Actions.Enabled {
		ctx.NotFound("MustEnableActions", nil)
		return
	}

	if unit.TypeActions.UnitGlobalDisabled() {
		ctx.NotFound("MustEnableActions", nil)
		return
	}

	if ctx.Repo.Repository != nil {
		if !ctx.Repo.CanRead(unit.TypeActions) {
			ctx.NotFound("MustEnableActions", nil)
			return
		}
	}
}

func List(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsActions"] = true

	commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		ctx.ServerError("GetBranchCommit", err)
		return
	}

	workflows := prepareWorkflowDispatchTemplate(ctx, commit)
	if ctx.Written() {
		return
	}

	prepareWorkflowList(ctx, workflows)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplListActions)
}

func WorkflowDispatchInputs(ctx *context.Context) {
	ref := ctx.FormString("ref")
	if ref == "" {
		ctx.NotFound("WorkflowDispatchInputs: no ref", nil)
		return
	}
	// get target commit of run from specified ref
	refName := git.RefName(ref)
	var commit *git.Commit
	var err error
	if refName.IsTag() {
		commit, err = ctx.Repo.GitRepo.GetTagCommit(refName.TagName())
	} else if refName.IsBranch() {
		commit, err = ctx.Repo.GitRepo.GetBranchCommit(refName.BranchName())
	} else {
		ctx.ServerError("UnsupportedRefType", nil)
		return
	}
	if err != nil {
		ctx.ServerError("GetTagCommit/GetBranchCommit", err)
		return
	}
	prepareWorkflowDispatchTemplate(ctx, commit)
	if ctx.Written() {
		return
	}
	ctx.HTML(http.StatusOK, tplDispatchInputsActions)
}

func prepareWorkflowDispatchTemplate(ctx *context.Context, commit *git.Commit) (workflows []Workflow) {
	workflowID := ctx.FormString("workflow")
	ctx.Data["CurWorkflow"] = workflowID
	ctx.Data["CurWorkflowExists"] = false

	var curWorkflow *model.Workflow

	entries, err := actions.ListWorkflows(commit)
	if err != nil {
		ctx.ServerError("ListWorkflows", err)
		return nil
	}

	// Get all runner labels
	runners, err := db.Find[actions_model.ActionRunner](ctx, actions_model.FindRunnerOptions{
		RepoID:        ctx.Repo.Repository.ID,
		IsOnline:      optional.Some(true),
		WithAvailable: true,
	})
	if err != nil {
		ctx.ServerError("FindRunners", err)
		return nil
	}
	allRunnerLabels := make(container.Set[string])
	for _, r := range runners {
		allRunnerLabels.AddMultiple(r.AgentLabels...)
	}

	workflows = make([]Workflow, 0, len(entries))
	for _, entry := range entries {
		workflow := Workflow{Entry: *entry}
		content, err := actions.GetContentFromEntry(entry)
		if err != nil {
			ctx.ServerError("GetContentFromEntry", err)
			return nil
		}
		wf, err := model.ReadWorkflow(bytes.NewReader(content))
		if err != nil {
			workflow.ErrMsg = ctx.Locale.TrString("actions.runs.invalid_workflow_helper", err.Error())
			workflows = append(workflows, workflow)
			continue
		}
		// The workflow must contain at least one job without "needs". Otherwise, a deadlock will occur and no jobs will be able to run.
		hasJobWithoutNeeds := false
		// Check whether you have matching runner and a job without "needs"
		emptyJobsNumber := 0
		for _, j := range wf.Jobs {
			if j == nil {
				emptyJobsNumber++
				continue
			}
			if !hasJobWithoutNeeds && len(j.Needs()) == 0 {
				hasJobWithoutNeeds = true
			}
			runsOnList := j.RunsOn()
			for _, ro := range runsOnList {
				if strings.Contains(ro, "${{") {
					// Skip if it contains expressions.
					// The expressions could be very complex and could not be evaluated here,
					// so just skip it, it's OK since it's just a tooltip message.
					continue
				}
				if !allRunnerLabels.Contains(ro) {
					workflow.ErrMsg = ctx.Locale.TrString("actions.runs.no_matching_online_runner_helper", ro)
					break
				}
			}
			if workflow.ErrMsg != "" {
				break
			}
		}
		if !hasJobWithoutNeeds {
			workflow.ErrMsg = ctx.Locale.TrString("actions.runs.no_job_without_needs")
		}
		if emptyJobsNumber == len(wf.Jobs) {
			workflow.ErrMsg = ctx.Locale.TrString("actions.runs.no_job")
		}
		workflows = append(workflows, workflow)

		if workflow.Entry.Name() == workflowID {
			curWorkflow = wf
			ctx.Data["CurWorkflowExists"] = true
		}
	}

	ctx.Data["workflows"] = workflows
	ctx.Data["RepoLink"] = ctx.Repo.Repository.Link()

	actionsConfig := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions).ActionsConfig()
	ctx.Data["ActionsConfig"] = actionsConfig

	if len(workflowID) > 0 && ctx.Repo.CanWrite(unit.TypeActions) {
		ctx.Data["AllowDisableOrEnableWorkflow"] = ctx.Repo.IsAdmin()
		isWorkflowDisabled := actionsConfig.IsWorkflowDisabled(workflowID)
		ctx.Data["CurWorkflowDisabled"] = isWorkflowDisabled

		if !isWorkflowDisabled && curWorkflow != nil {
			workflowDispatchConfig := workflowDispatchConfig(curWorkflow)
			if workflowDispatchConfig != nil {
				ctx.Data["WorkflowDispatchConfig"] = workflowDispatchConfig

				branchOpts := git_model.FindBranchOptions{
					RepoID:          ctx.Repo.Repository.ID,
					IsDeletedBranch: optional.Some(false),
					ListOptions: db.ListOptions{
						ListAll: true,
					},
				}
				branches, err := git_model.FindBranchNames(ctx, branchOpts)
				if err != nil {
					ctx.ServerError("FindBranchNames", err)
					return nil
				}
				// always put default branch on the top if it exists
				if slices.Contains(branches, ctx.Repo.Repository.DefaultBranch) {
					branches = util.SliceRemoveAll(branches, ctx.Repo.Repository.DefaultBranch)
					branches = append([]string{ctx.Repo.Repository.DefaultBranch}, branches...)
				}
				ctx.Data["Branches"] = branches

				tags, err := repo_model.GetTagNamesByRepoID(ctx, ctx.Repo.Repository.ID)
				if err != nil {
					ctx.ServerError("GetTagNamesByRepoID", err)
					return nil
				}
				ctx.Data["Tags"] = tags
			}
		}
	}
	return workflows
}

func prepareWorkflowList(ctx *context.Context, workflows []Workflow) {
	actorID := ctx.FormInt64("actor")
	status := ctx.FormInt("status")
	workflowID := ctx.FormString("workflow")
	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	// if status or actor query param is not given to frontend href, (href="/<repoLink>/actions")
	// they will be 0 by default, which indicates get all status or actors
	ctx.Data["CurActor"] = actorID
	ctx.Data["CurStatus"] = status
	if actorID > 0 || status > int(actions_model.StatusUnknown) {
		ctx.Data["IsFiltered"] = true
	}

	opts := actions_model.FindRunOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
		},
		RepoID:        ctx.Repo.Repository.ID,
		WorkflowID:    workflowID,
		TriggerUserID: actorID,
	}

	// if status is not StatusUnknown, it means user has selected a status filter
	if actions_model.Status(status) != actions_model.StatusUnknown {
		opts.Status = []actions_model.Status{actions_model.Status(status)}
	}

	runs, total, err := db.FindAndCount[actions_model.ActionRun](ctx, opts)
	if err != nil {
		ctx.ServerError("FindAndCount", err)
		return
	}

	for _, run := range runs {
		run.Repo = ctx.Repo.Repository
	}

	if err := actions_model.RunList(runs).LoadTriggerUser(ctx); err != nil {
		ctx.ServerError("LoadTriggerUser", err)
		return
	}

	if err := loadIsRefDeleted(ctx, ctx.Repo.Repository.ID, runs); err != nil {
		log.Error("LoadIsRefDeleted", err)
	}

	ctx.Data["Runs"] = runs

	actors, err := actions_model.GetActors(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetActors", err)
		return
	}
	ctx.Data["Actors"] = shared_user.MakeSelfOnTop(ctx.Doer, actors)

	ctx.Data["StatusInfoList"] = actions_model.GetStatusInfoList(ctx)

	pager := context.NewPagination(int(total), opts.PageSize, opts.Page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager
	ctx.Data["HasWorkflowsOrRuns"] = len(workflows) > 0 || len(runs) > 0
}

// loadIsRefDeleted loads the IsRefDeleted field for each run in the list.
// TODO: move this function to models/actions/run_list.go but now it will result in a circular import.
func loadIsRefDeleted(ctx stdCtx.Context, repoID int64, runs actions_model.RunList) error {
	branches := make(container.Set[string], len(runs))
	for _, run := range runs {
		refName := git.RefName(run.Ref)
		if refName.IsBranch() {
			branches.Add(refName.ShortName())
		}
	}
	if len(branches) == 0 {
		return nil
	}

	branchInfos, err := git_model.GetBranches(ctx, repoID, branches.Values(), false)
	if err != nil {
		return err
	}
	branchSet := git_model.BranchesToNamesSet(branchInfos)
	for _, run := range runs {
		refName := git.RefName(run.Ref)
		if refName.IsBranch() && !branchSet.Contains(refName.ShortName()) {
			run.IsRefDeleted = true
		}
	}
	return nil
}

type WorkflowDispatchInput struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Required    bool     `yaml:"required"`
	Default     string   `yaml:"default"`
	Type        string   `yaml:"type"`
	Options     []string `yaml:"options"`
}

type WorkflowDispatch struct {
	Inputs []WorkflowDispatchInput
}

func workflowDispatchConfig(w *model.Workflow) *WorkflowDispatch {
	switch w.RawOn.Kind {
	case yaml.ScalarNode:
		var val string
		if !decodeNode(w.RawOn, &val) {
			return nil
		}
		if val == "workflow_dispatch" {
			return &WorkflowDispatch{}
		}
	case yaml.SequenceNode:
		var val []string
		if !decodeNode(w.RawOn, &val) {
			return nil
		}
		for _, v := range val {
			if v == "workflow_dispatch" {
				return &WorkflowDispatch{}
			}
		}
	case yaml.MappingNode:
		var val map[string]yaml.Node
		if !decodeNode(w.RawOn, &val) {
			return nil
		}

		workflowDispatchNode, found := val["workflow_dispatch"]
		if !found {
			return nil
		}

		var workflowDispatch WorkflowDispatch
		var workflowDispatchVal map[string]yaml.Node
		if !decodeNode(workflowDispatchNode, &workflowDispatchVal) {
			return &workflowDispatch
		}

		inputsNode, found := workflowDispatchVal["inputs"]
		if !found || inputsNode.Kind != yaml.MappingNode {
			return &workflowDispatch
		}

		i := 0
		for {
			if i+1 >= len(inputsNode.Content) {
				break
			}
			var input WorkflowDispatchInput
			if decodeNode(*inputsNode.Content[i+1], &input) {
				input.Name = inputsNode.Content[i].Value
				workflowDispatch.Inputs = append(workflowDispatch.Inputs, input)
			}
			i += 2
		}
		return &workflowDispatch

	default:
		return nil
	}
	return nil
}

func decodeNode(node yaml.Node, out any) bool {
	if err := node.Decode(out); err != nil {
		log.Warn("Failed to decode node %v into %T: %v", node, out, err)
		return false
	}
	return true
}
