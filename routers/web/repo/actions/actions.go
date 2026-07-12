// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"bytes"
	stdCtx "context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"slices"
	"strings"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/actions"
	"gitea.dev/modules/base"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	shared_user "gitea.dev/routers/web/shared/user"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"

	act_model "gitea.com/gitea/runner/act/model"
	"go.yaml.in/yaml/v4"
)

const (
	tplListActions           templates.TplName = "repo/actions/list"
	tplDispatchInputsActions templates.TplName = "repo/actions/workflow_dispatch_inputs"
	tplViewActions           templates.TplName = "repo/actions/view"
)

type WorkflowInfo struct {
	EntryName string
	ErrMsg    string
	Workflow  *act_model.Workflow
}

type workflowBadge struct {
	DisplayName string
	BadgeURL    string
	WorkflowURL string
}

// DisplayName returns the workflow name from the YAML file if present, otherwise the filename.
func (w WorkflowInfo) DisplayName() string {
	if w.Workflow != nil && w.Workflow.Name != "" {
		return w.Workflow.Name
	}
	return w.EntryName
}

// MustEnableActions check if actions are enabled in settings
func MustEnableActions(ctx *context.Context) {
	if !setting.Actions.Enabled {
		ctx.NotFound(nil)
		return
	}

	if unit.TypeActions.UnitGlobalDisabled() {
		ctx.NotFound(nil)
		return
	}

	if ctx.Repo.Repository != nil {
		if !ctx.Repo.Permission.CanRead(unit.TypeActions) {
			ctx.NotFound(nil)
			return
		}
	}
}

func List(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsActions"] = true

	commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
	if errors.Is(err, util.ErrNotExist) {
		ctx.Data["NotFoundPrompt"] = ctx.Tr("repo.branch.default_branch_not_exist", ctx.Repo.Repository.DefaultBranch)
		ctx.NotFound(nil)
		return
	} else if err != nil {
		ctx.ServerError("GetBranchCommit", err)
		return
	}

	workflows, curWorkflowID := prepareWorkflowTemplate(ctx, commit)
	if ctx.Written() {
		return
	}

	data := prepareWorkflowList(ctx, curWorkflowID)
	if data.partialRefresh {
		// TODO: at the moment, the "partial refresh" depends on some heavy git&db operations (especially finding & parsing the workflow files)
		// If it becomes a real problem in the future, maybe some "caches" can be used to optimize.
		if !data.preparePartialRefreshRuns(ctx) {
			return
		}
		if !data.prepareCommon(ctx, workflows) {
			return
		}
		ctx.HTML(http.StatusOK, "repo/actions/runs_list")
		return
	}

	scopedNames := prepareScopedWorkflows(ctx, curWorkflowID, data.scopedWorkflowSourceRepoID)
	if ctx.Written() {
		return
	}
	otherWorkflows := prepareOtherWorkflows(ctx, workflows, scopedNames, curWorkflowID)
	if ctx.Written() {
		return
	}
	prepareWorkflowDispatchTemplate(ctx, workflows, curWorkflowID, data.scopedWorkflowSourceRepoID)
	if ctx.Written() {
		return
	}

	if !data.prepareFullPageRuns(ctx, otherWorkflows) {
		return
	}
	if !data.prepareCommon(ctx, workflows) {
		return
	}
	if !data.prepareFullPageTemplate(ctx) {
		return
	}

	ctx.Data["HasWorkflowsOrRuns"] = len(workflows) > 0 || len(otherWorkflows) > 0 || len(data.ActionRuns) > 0 || len(scopedNames) > 0
	ctx.HTML(http.StatusOK, tplListActions)
}

// prepareOtherWorkflows surfaces historical runs whose workflow file no longer
// exists on the default branch (renamed, removed, or only on other branches).
func prepareOtherWorkflows(ctx *context.Context, workflows []WorkflowInfo, scopedNames container.Set[string], curWorkflowID string) []string {
	listed := make(container.Set[string], len(workflows))
	for _, w := range workflows {
		listed.Add(w.EntryName)
	}

	var other []string
	if ctx.Repo.Repository.NumActionRuns > 0 {
		// "Other workflows" lists repo-level orphans only: GetRepoRunWorkflowIDs excludes scoped runs.
		ids, err := actions_model.GetRepoRunWorkflowIDs(ctx, ctx.Repo.Repository.ID)
		if err != nil {
			ctx.ServerError("GetRepoRunWorkflowIDs", err)
			return nil
		}
		other = container.FilterSlice(ids, func(id string) (string, bool) {
			return id, id != "" && !listed.Contains(id)
		})
	}

	ctx.Data["OtherWorkflows"] = other
	// A selected workflow counts as "listed" if it is a repo-level file or an active scoped workflow.
	ctx.Data["CurWorkflowIsListed"] = curWorkflowID == "" || listed.Contains(curWorkflowID) || scopedNames.Contains(curWorkflowID)
	return other
}

func WorkflowDispatchInputs(ctx *context.Context) {
	ref := ctx.FormString("ref")
	if ref == "" {
		ctx.NotFound(nil)
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
	workflows, curWorkflowID := prepareWorkflowTemplate(ctx, commit)
	if ctx.Written() {
		return
	}
	prepareWorkflowDispatchTemplate(ctx, workflows, curWorkflowID, ctx.FormInt64("scoped_workflow_source_repo_id"))
	if ctx.Written() {
		return
	}
	ctx.HTML(http.StatusOK, tplDispatchInputsActions)
}

func prepareWorkflowTemplate(ctx *context.Context, commit *git.Commit) (workflows []WorkflowInfo, curWorkflowID string) {
	curWorkflowID = ctx.FormString("workflow")

	_, entries, err := actions.ListWorkflows(commit)
	if err != nil {
		ctx.ServerError("ListWorkflows", err)
		return nil, ""
	}

	workflows = make([]WorkflowInfo, 0, len(entries))
	for _, entry := range entries {
		workflow := WorkflowInfo{EntryName: entry.Name()}
		content, err := actions.GetContentFromEntry(entry)
		if err != nil {
			ctx.ServerError("GetContentFromEntry", err)
			return nil, ""
		}
		wf, err := act_model.ReadWorkflow(bytes.NewReader(content))
		if err != nil {
			workflow.ErrMsg = ctx.Locale.TrString("actions.runs.invalid_workflow_helper", err.Error())
			workflows = append(workflows, workflow)
			continue
		}
		if err := actions.ValidateWorkflowContent(content); err != nil {
			workflow.ErrMsg = ctx.Locale.TrString("actions.runs.invalid_workflow_helper", err.Error())
			workflows = append(workflows, workflow)
			continue
		}
		workflow.Workflow = wf
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
			if j.Uses != "" {
				if _, err := actions_service.ResolveUses(ctx, j.Uses); err != nil {
					workflow.ErrMsg = ctx.Locale.TrString("actions.runs.invalid_reusable_workflow_uses", err.Error())
					break
				}
			}
		}
		if workflow.ErrMsg == "" {
			if !hasJobWithoutNeeds {
				workflow.ErrMsg = ctx.Locale.TrString("actions.runs.no_job_without_needs")
			}
			if emptyJobsNumber == len(wf.Jobs) {
				workflow.ErrMsg = ctx.Locale.TrString("actions.runs.no_job")
			}
		}
		workflows = append(workflows, workflow)
	}

	ctx.Data["workflows"] = workflows
	ctx.Data["RepoLink"] = ctx.Repo.Repository.Link()
	ctx.Data["RepoID"] = ctx.Repo.Repository.ID
	ctx.Data["AllowDisableOrEnableWorkflow"] = ctx.Repo.Permission.IsAdmin()
	actionsConfig := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions).ActionsConfig()
	ctx.Data["ActionsConfig"] = actionsConfig
	ctx.Data["CurWorkflow"] = curWorkflowID
	ctx.Data["CurWorkflowDisabled"] = actionsConfig.IsWorkflowDisabled(curWorkflowID)

	return workflows, curWorkflowID
}

// ScopedWorkflowInfo describes a scoped workflow effective for the current repo, listed under its source group.
type ScopedWorkflowInfo struct {
	SourceRepoID int64
	EntryName    string
	DisplayName  string
	Required     bool
	Disabled     bool
}

// ScopedWorkflowSourceGroup groups the scoped workflows contributed by one source repo for the All-Workflows sidebar.
type ScopedWorkflowSourceGroup struct {
	SourceRepoID        int64
	SourceRepoName      string // owner/name of the source repo; shown for instance-level sources and used as the tooltip
	SourceRepoShortName string // name only; shown for owner-level sources, where the owner is always the current owner
	FromInstance        bool   // registered at instance level (owner_id == 0) rather than by the owner
	IsActive            bool   // the currently-selected workflow belongs to this source; render the group expanded
	Workflows           []ScopedWorkflowInfo
}

// prepareScopedWorkflows lists the scoped workflows effective for the repo's owner (and instance) for the All-Workflows sidebar.
func prepareScopedWorkflows(ctx *context.Context, curWorkflowID string, curWorkflowRepoID int64) container.Set[string] {
	scopedNames := make(container.Set[string])

	repo := ctx.Repo.Repository
	sources, err := actions_model.GetEffectiveScopedWorkflowSources(ctx, repo.OwnerID)
	if err != nil {
		ctx.ServerError("GetEffectiveScopedWorkflowSources", err)
		return scopedNames
	}
	if len(sources) == 0 {
		return scopedNames
	}

	actionsConfig := repo.MustGetUnit(ctx, unit.TypeActions).ActionsConfig()

	groups := make([]ScopedWorkflowSourceGroup, 0, len(sources))
	seen := make(map[int64]bool, len(sources))
	for _, source := range sources {
		if seen[source.SourceRepoID] {
			continue
		}
		seen[source.SourceRepoID] = true

		sourceRepo, err := repo_model.GetRepositoryByID(ctx, source.SourceRepoID)
		if err != nil {
			log.Error("scoped workflows list: load source repo %d: %v", source.SourceRepoID, err)
			continue
		}
		if sourceRepo.IsEmpty {
			continue
		}

		_, entries, err := actions_service.LoadParsedScopedWorkflows(ctx, sourceRepo)
		if err != nil {
			log.Error("scoped workflows list: parse %s: %v", sourceRepo.FullName(), err)
			continue
		}
		if len(entries) == 0 {
			continue
		}

		group := ScopedWorkflowSourceGroup{
			SourceRepoID:        sourceRepo.ID,
			SourceRepoName:      sourceRepo.FullName(),
			SourceRepoShortName: sourceRepo.Name,
			FromInstance:        source.OwnerID == 0,
		}
		for _, e := range entries {
			scopedNames.Add(e.EntryName)
			required := actions_model.IsWorkflowRequiredInSources(sources, sourceRepo.ID, e.EntryName)
			disabled := actionsConfig.IsScopedWorkflowDisabled(sourceRepo.ID, e.EntryName)
			group.Workflows = append(group.Workflows, ScopedWorkflowInfo{
				SourceRepoID: sourceRepo.ID,
				EntryName:    e.EntryName,
				DisplayName:  e.DisplayName,
				Required:     required,
				Disabled:     disabled,
			})

			if curWorkflowID == e.EntryName && curWorkflowRepoID == sourceRepo.ID {
				ctx.Data["CurWorkflowDisabled"] = disabled
				ctx.Data["CurWorkflowScopedRepoID"] = sourceRepo.ID
				ctx.Data["CurWorkflowRequired"] = required
				group.IsActive = true // keep this group expanded so the selected workflow stays visible
			}
		}
		groups = append(groups, group)
	}

	ctx.Data["ScopedWorkflowGroups"] = groups
	return scopedNames
}

// loadScopedWorkflowModel reads and parses a scoped workflow's content from its source repo's default branch.
func loadScopedWorkflowModel(ctx *context.Context, repo *repo_model.Repository, sourceRepoID int64, workflowID string) *act_model.Workflow {
	effective, err := actions_model.IsScopedWorkflowSourceEffective(ctx, repo.OwnerID, sourceRepoID)
	if err != nil {
		log.Error("scoped dispatch: IsScopedWorkflowSourceEffective: %v", err)
		return nil
	}
	if !effective {
		return nil
	}

	sourceRepo, err := repo_model.GetRepositoryByID(ctx, sourceRepoID)
	if err != nil || sourceRepo.IsEmpty {
		return nil
	}
	content, err := actions_service.ScopedWorkflowContent(ctx, sourceRepo, workflowID)
	if err != nil {
		log.Error("scoped dispatch: content of %s in %s: %v", workflowID, sourceRepo.RelativePath(), err)
		return nil
	}
	if content == nil {
		return nil // the workflow does not exist on the source's default branch
	}
	wf, err := act_model.ReadWorkflow(bytes.NewReader(content))
	if err != nil {
		return nil
	}
	return wf
}

func prepareWorkflowDispatchTemplate(ctx *context.Context, workflowInfos []WorkflowInfo, curWorkflowID string, curWorkflowRepoID int64) {
	repo := ctx.Repo.Repository
	if curWorkflowID == "" || !ctx.Repo.Permission.CanWrite(unit.TypeActions) {
		return
	}
	actionsConfig := repo.MustGetUnit(ctx, unit.TypeActions).ActionsConfig()

	isScoped := curWorkflowRepoID > 0
	if isScoped {
		// a required scoped workflow can never be opted out, so a stale disabled flag must not hide its dispatch form
		optedOut, err := actions_model.IsScopedWorkflowOptedOut(ctx, actionsConfig, repo.OwnerID, curWorkflowRepoID, curWorkflowID)
		if err != nil {
			log.Error("IsScopedWorkflowOptedOut: %v", err)
			return
		}
		if optedOut {
			return
		}
	} else if actionsConfig.IsWorkflowDisabled(curWorkflowID) {
		return
	}

	var curWorkflow *act_model.Workflow
	if isScoped {
		// a scoped workflow's content lives in its source repo, not in workflowInfos (the consumer's own files)
		curWorkflow = loadScopedWorkflowModel(ctx, repo, curWorkflowRepoID, curWorkflowID)
	} else {
		for _, workflowInfo := range workflowInfos {
			if workflowInfo.EntryName == curWorkflowID {
				if workflowInfo.Workflow == nil {
					log.Debug("CurWorkflowID %s is found but its workflowInfo.Workflow is nil", curWorkflowID)
					return
				}
				curWorkflow = workflowInfo.Workflow
				break
			}
		}
	}

	if curWorkflow == nil {
		return
	}

	ctx.Data["CurWorkflowExists"] = true
	curWfDispatchCfg := workflowDispatchConfig(curWorkflow)
	if curWfDispatchCfg == nil {
		return
	}

	ctx.Data["WorkflowDispatchConfig"] = curWfDispatchCfg

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
		return
	}
	// always put default branch on the top
	branches = util.SliceRemoveAll(branches, ctx.Repo.Repository.DefaultBranch)
	branches = append([]string{ctx.Repo.Repository.DefaultBranch}, branches...)
	ctx.Data["Branches"] = branches

	tags, err := repo_model.GetTagNamesByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetTagNamesByRepoID", err)
		return
	}
	ctx.Data["Tags"] = tags
}

func (data *actionRunListData) prepareFullPageRuns(ctx *context.Context, otherWorkflows []string) bool {
	opts := actions_model.FindRunOptions{
		ListOptions: db.ListOptions{
			Page:     max(ctx.FormInt("page"), 1),
			PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
		},
		RepoID:         ctx.Repo.Repository.ID,
		WorkflowID:     data.workflowID,
		WorkflowRepoID: data.scopedWorkflowSourceRepoID,
		TriggerUserID:  data.actorID,
	}

	// Constrain scoped vs repo-level only for a listed workflow, whose link carries scoped_workflow_source_repo_id.
	if data.workflowID != "" && !slices.Contains(otherWorkflows, data.workflowID) {
		opts.IsScopedRun = optional.Some(data.scopedWorkflowSourceRepoID > 0)
	}

	// if status is not StatusUnknown, it means user has selected a status filter
	if data.status != actions_model.StatusUnknown {
		opts.Status = []actions_model.Status{data.status}
	}
	if data.branch != "" {
		opts.Ref = string(git.RefNameFromBranch(data.branch))
	}

	runs, total, err := db.FindAndCount[actions_model.ActionRun](ctx, opts)
	if err != nil {
		ctx.ServerError("FindAndCount", err)
		return false
	}
	data.pager = context.NewPagination(total, opts.PageSize, opts.Page, 5)
	data.pager.AddParamFromRequest(ctx.Req)
	data.ActionRuns = runs
	return true
}

type actionRunListData struct {
	actorID                    int64
	status                     actions_model.Status
	workflowID                 string
	workflowDisplayName        string
	scopedWorkflowSourceRepoID int64
	branch                     string
	refreshRunIDs              []int64
	partialRefresh             bool
	pager                      *context.Pagination

	// the following fields are used by the template
	RefreshLink             template.URL
	RefreshIntervalMs       int64
	ActionRuns              []*actions_model.ActionRun
	IsFiltered              bool
	WorkflowNames           map[string]string
	RunErrors               map[int64]string
	CurWorkflow             string
	CanWriteRepoUnitActions bool
}

func (data *actionRunListData) processActionRuns(ctx *context.Context) bool {
	// Check for each run if there is at least one online runner that can run its jobs
	runners, err := db.Find[actions_model.ActionRunner](ctx, actions_model.FindRunnerOptions{
		RepoID:        ctx.Repo.Repository.ID,
		IsOnline:      optional.Some(true),
		WithAvailable: true,
	})
	if err != nil {
		ctx.ServerError("FindRunners", err)
		return false
	}

	data.RunErrors = make(map[int64]string)
	for _, run := range data.ActionRuns {
		if !run.Status.In(actions_model.StatusWaiting, actions_model.StatusRunning, actions_model.StatusBlocked) {
			continue
		}
		jobs, err := actions_model.GetLatestAttemptJobsByRepoAndRunID(ctx, run.RepoID, run.ID)
		if err != nil {
			ctx.ServerError("GetRunJobsByRunID", err)
			return false
		}
		for _, job := range jobs {
			if !job.Status.In(actions_model.StatusWaiting, actions_model.StatusBlocked) {
				continue
			}
			if err := actions.ValidateWorkflowContent(job.WorkflowPayload); err != nil {
				data.RunErrors[run.ID] = ctx.Locale.TrString("actions.runs.invalid_workflow_helper", err.Error())
				break
			}
			if job.CallUses != "" {
				if _, err := actions_service.ResolveUses(ctx, job.CallUses); err != nil {
					data.RunErrors[run.ID] = ctx.Locale.TrString("actions.runs.invalid_reusable_workflow_uses", err.Error())
					break
				}
			}
			if job.Status.IsWaiting() {
				hasOnlineRunner := false
				for _, runner := range runners {
					if !runner.IsDisabled && runner.CanMatchLabels(job.RunsOn) {
						hasOnlineRunner = true
						break
					}
				}
				if !hasOnlineRunner {
					data.RunErrors[run.ID] = ctx.Locale.TrString("actions.runs.no_matching_online_runner_helper", strings.Join(job.RunsOn, ","))
					break
				}
			}
		}
	}
	return true
}

func (data *actionRunListData) fillRefreshMeta(ctx *context.Context) bool {
	hasActiveRuns := false
	var actionRunIDs []int64
	for _, run := range data.ActionRuns {
		if !hasActiveRuns && run.Status.In(actions_model.StatusWaiting, actions_model.StatusRunning, actions_model.StatusBlocked) {
			hasActiveRuns = true
		}
		actionRunIDs = append(actionRunIDs, run.ID)
	}
	data.RefreshIntervalMs = util.Iif[int64](hasActiveRuns, 3*1000, 12*1000)
	if !setting.IsProd {
		data.RefreshIntervalMs = util.Iif[int64](hasActiveRuns, 1000, 2*1000) // faster in dev mode to make debug easier
	}
	if len(data.ActionRuns) == 0 {
		data.RefreshIntervalMs = 0
	}
	// Only refresh the items on the current page, do not keep loading "new" items.
	// Otherwise, when end users are browsing the list page, they will just keep seeing "new" items coming in
	// and "old" items disappearing, they won't be able to focus on an item that they are interested in, which is a bad user experience.
	data.RefreshLink = templates.QueryBuild(setting.AppSubURL+ctx.Req.RequestURI, "run_ids", strings.Join(base.Int64sToStrings(actionRunIDs), ","))
	return true
}

func (data *actionRunListData) fillWorkflowNames(workflows []WorkflowInfo) bool {
	data.WorkflowNames = make(map[string]string, len(workflows))
	data.workflowDisplayName = data.workflowID
	for _, wf := range workflows {
		displayName := wf.DisplayName()
		data.WorkflowNames[wf.EntryName] = displayName
		if wf.EntryName == data.workflowID {
			data.workflowDisplayName = displayName
		}
	}
	return true
}

func prepareWorkflowList(ctx *context.Context, curWorkflowID string) *actionRunListData {
	data := &actionRunListData{}
	data.actorID = ctx.FormInt64("actor")
	data.status = actions_model.Status(ctx.FormInt("status"))
	data.workflowID = ctx.FormString("workflow")
	data.scopedWorkflowSourceRepoID = ctx.FormInt64("scoped_workflow_source_repo_id")
	data.branch = ctx.FormString("branch")
	data.refreshRunIDs = ctx.FormStringInt64s("run_ids")
	data.partialRefresh = ctx.Req.Form.Has("run_ids")

	data.IsFiltered = data.actorID > 0 || data.status != actions_model.StatusUnknown || data.branch != ""
	data.CurWorkflow = curWorkflowID
	data.CanWriteRepoUnitActions = ctx.Repo.Permission.CanWrite(unit.TypeActions)
	return data
}

func (data *actionRunListData) preparePartialRefreshRuns(ctx *context.Context) bool {
	runs, err := actions_model.GetRunsByRepoAndID(ctx, ctx.Repo.Repository.ID, data.refreshRunIDs)
	if err != nil {
		ctx.ServerError("GetRunsByRepoAndID", err)
		return false
	}
	data.ActionRuns = runs
	return true
}

func (data *actionRunListData) prepareCommon(ctx *context.Context, workflows []WorkflowInfo) bool {
	for _, run := range data.ActionRuns {
		run.Repo = ctx.Repo.Repository
	}

	if err := actions_model.RunList(data.ActionRuns).LoadTriggerUser(ctx); err != nil {
		ctx.ServerError("LoadTriggerUser", err)
		return false
	}
	if err := loadIsRefDeleted(ctx, ctx.Repo.Repository.ID, data.ActionRuns); err != nil {
		log.Error("LoadIsRefDeleted", err)
	}

	if !data.processActionRuns(ctx) {
		return false
	}
	if !data.fillRefreshMeta(ctx) {
		return false
	}
	if !data.fillWorkflowNames(workflows) {
		return false
	}
	ctx.Data["ActionRunListData"] = data
	return true
}

func (data *actionRunListData) prepareFullPageTemplate(ctx *context.Context) bool {
	// A scoped workflow has no repo-level badge on this repo (the badge endpoint reads is_scoped_run=false runs),
	// so don't offer the "create status badge" entry for it.
	if data.scopedWorkflowSourceRepoID == 0 {
		prepareWorkflowBadgeTemplate(ctx, data.workflowID, data.workflowDisplayName)
	}

	// fill template data
	actors, err := actions_model.GetActors(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetActors", err)
		return false
	}
	ctx.Data["Actors"] = shared_user.MakeSelfOnTop(ctx.Doer, actors)

	runBranches, err := actions_model.GetRunBranches(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetRunBranches", err)
		return false
	}
	ctx.Data["RunBranches"] = runBranches

	ctx.Data["StatusInfoList"] = actions_model.GetStatusInfoList(ctx, ctx.Locale)
	ctx.Data["CurActor"] = data.actorID
	ctx.Data["CurStatus"] = int(data.status) // don't make template use its "String" method, otherwise the query parameter would be wrong
	ctx.Data["CurBranch"] = data.branch
	ctx.Data["CurWorkflowRepoID"] = data.scopedWorkflowSourceRepoID
	ctx.Data["Page"] = data.pager
	return true
}

func prepareWorkflowBadgeTemplate(ctx *context.Context, workflowID, displayName string) {
	if workflowID == "" {
		return
	}
	displayName = util.IfZero(displayName, workflowID)

	repoURL := ctx.Repo.Repository.HTMLURL(ctx)
	badgeURL := fmt.Sprintf("%s/actions/workflows/%s/badge.svg?branch=%s", repoURL, util.PathEscapeSegments(workflowID), url.QueryEscape(ctx.Repo.Repository.DefaultBranch))
	workflowURL := fmt.Sprintf("%s/actions?workflow=%s", repoURL, url.QueryEscape(workflowID))

	ctx.Data["WorkflowBadge"] = workflowBadge{
		BadgeURL:    badgeURL,
		WorkflowURL: workflowURL,
		DisplayName: displayName,
	}
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

func workflowDispatchConfig(w *act_model.Workflow) *WorkflowDispatch {
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
		if slices.Contains(val, "workflow_dispatch") {
			return &WorkflowDispatch{}
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

func actionsListRedirectURL(repoLink, workflow, scopedWorkflowSourceRepoID, actor, status, branch string) string {
	return fmt.Sprintf("%s/actions?workflow=%s&scoped_workflow_source_repo_id=%s&actor=%s&status=%s&branch=%s",
		repoLink,
		url.QueryEscape(workflow),
		url.QueryEscape(scopedWorkflowSourceRepoID),
		url.QueryEscape(actor),
		url.QueryEscape(status),
		url.QueryEscape(branch),
	)
}
