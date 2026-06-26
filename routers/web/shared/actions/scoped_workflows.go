// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	actions_model "gitea.dev/models/actions"
	repo_model "gitea.dev/models/repo"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/container"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	shared_user "gitea.dev/routers/web/shared/user"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/context"
)

const (
	tplOrgScopedWorkflows   templates.TplName = "org/settings/actions"
	tplUserScopedWorkflows  templates.TplName = "user/settings/actions"
	tplAdminScopedWorkflows templates.TplName = "admin/actions"
)

type scopedWorkflowsCtx struct {
	OwnerID      int64 // 0 = instance-level
	IsOrg        bool
	IsUser       bool
	IsGlobal     bool
	Template     templates.TplName
	RedirectLink string
	// SearchUID is the uid passed to the repo-search box. For org/user it scopes the search to that owner;
	// for admin (0) it searches all repos and therefore requires admin access on the route.
	SearchUID int64
}

func getScopedWorkflowsCtx(ctx *context.Context) (*scopedWorkflowsCtx, error) {
	if ctx.Data["PageIsOrgSettings"] == true {
		if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
			ctx.ServerError("RenderUserOrgHeader", err)
			return nil, nil //nolint:nilnil // error is already handled by ctx.ServerError
		}
		return &scopedWorkflowsCtx{
			OwnerID:      ctx.Org.Organization.ID,
			IsOrg:        true,
			Template:     tplOrgScopedWorkflows,
			RedirectLink: ctx.Org.OrgLink + "/settings/actions/scoped-workflows",
			SearchUID:    ctx.Org.Organization.ID,
		}, nil
	}

	if ctx.Data["PageIsUserSettings"] == true {
		return &scopedWorkflowsCtx{
			OwnerID:      ctx.Doer.ID,
			IsUser:       true,
			Template:     tplUserScopedWorkflows,
			RedirectLink: setting.AppSubURL + "/user/settings/actions/scoped-workflows",
			SearchUID:    ctx.Doer.ID,
		}, nil
	}

	if ctx.Data["PageIsAdmin"] == true {
		return &scopedWorkflowsCtx{
			OwnerID:      0,
			IsGlobal:     true,
			Template:     tplAdminScopedWorkflows,
			RedirectLink: setting.AppSubURL + "/-/admin/actions/scoped-workflows",
			SearchUID:    0,
		}, nil
	}

	return nil, errors.New("unable to set scoped workflows context")
}

// scopedWorkflowInfo is one scoped workflow shown on the settings page, merged with its stored merge-gate config.
type scopedWorkflowInfo struct {
	EntryName   string
	DisplayName string
	Required    bool
	Patterns    string   // newline-joined stored status-check patterns (kept even when not required, as history)
	Contexts    []string // the commit-status contexts this workflow is expected to post, to preview which patterns match
	Missing     bool     // the workflow file no longer exists on the source default branch, but a stored config lingers and must stay clearable
}

// scopedWorkflowSourceView is the per-source data shown on the settings page.
type scopedWorkflowSourceView struct {
	Repo                *repo_model.Repository
	ScopedWorkflowInfos []scopedWorkflowInfo
}

func ScopedWorkflows(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.scoped_workflows")
	ctx.Data["PageType"] = "scoped-workflows"
	ctx.Data["PageIsSharedSettingsScopedWorkflows"] = true

	swCtx, err := getScopedWorkflowsCtx(ctx)
	if err != nil {
		ctx.ServerError("getScopedWorkflowsCtx", err)
		return
	}
	if ctx.Written() {
		return
	}

	switch {
	case swCtx.IsOrg:
		ctx.Data["ScopedWorkflowsDesc"] = ctx.Tr("actions.scoped_workflows.desc_org")
	case swCtx.IsUser:
		ctx.Data["ScopedWorkflowsDesc"] = ctx.Tr("actions.scoped_workflows.desc_user")
	default: // instance-level
		ctx.Data["ScopedWorkflowsDesc"] = ctx.Tr("actions.scoped_workflows.desc_global")
	}

	sources, err := actions_model.GetScopedWorkflowSourcesByOwner(ctx, swCtx.OwnerID)
	if err != nil {
		ctx.ServerError("GetScopedWorkflowSourcesByOwner", err)
		return
	}

	views := make([]*scopedWorkflowSourceView, 0, len(sources))
	for _, src := range sources {
		repo, err := repo_model.GetRepositoryByID(ctx, src.SourceRepoID)
		if err != nil {
			log.Error("scoped workflows settings: load source repo %d: %v", src.SourceRepoID, err)
			continue
		}
		views = append(views, &scopedWorkflowSourceView{
			Repo:                repo,
			ScopedWorkflowInfos: listSourceScopedWorkflowFiles(ctx, repo, src.WorkflowConfigs),
		})
	}

	ctx.Data["ScopedWorkflowSources"] = views
	ctx.Data["RepoSearchUID"] = swCtx.SearchUID
	// owner/user scopes the repo search to the owner (exclusive);
	// instance-level (admin) searches all repos and so must submit owner/name to disambiguate the selection across owners.
	ctx.Data["ScopedWorkflowsSearchExclusive"] = !swCtx.IsGlobal
	ctx.Data["ScopedWorkflowsSearchFullName"] = swCtx.IsGlobal
	ctx.Data["RedirectLink"] = swCtx.RedirectLink
	ctx.Data["ScopedWorkflowDirs"] = strings.Join(setting.Actions.ScopedWorkflowDirs, ", ")
	ctx.HTML(http.StatusOK, swCtx.Template)
}

// parsePatternLines splits a textarea value into trimmed, non-empty status-check patterns (one per line).
func parsePatternLines(raw string) []string {
	var patterns []string
	for line := range strings.SplitSeq(raw, "\n") {
		if p := strings.TrimSpace(line); p != "" {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

// deriveScopedStatusContexts returns the commit-status contexts a scoped workflow is expected to post on a consumer:
// "<source FullName>: <display> / <job> (<event>)" for each parsed job (matrix-expanded, matching run creation) and triggering event.
// Job names that depend on run-context expressions cannot resolve here (no run context) and appear as authored; a glob pattern still matches them.
func deriveScopedStatusContexts(prefix, displayName string, content []byte, events []*jobparser.Event) []string {
	parsed, err := jobparser.Parse(content)
	if err != nil {
		return nil
	}
	eventNames := make([]string, 0, len(events))
	for _, e := range events {
		// only events whose runs post a commit status can be a required check; workflow_dispatch, schedule, etc. post none.
		if actions_module.ShouldEventCreateCommitStatus(e.Name) {
			eventNames = append(eventNames, e.Name)
		}
	}
	seen := make(container.Set[string])
	contexts := make([]string, 0, len(parsed)*len(eventNames))
	for _, sw := range parsed {
		_, job := sw.Job()
		if job == nil {
			continue
		}
		jobName := util.EllipsisDisplayString(job.Name, 255) // run creation truncates job names the same way
		for _, ev := range eventNames {
			ctxName := actions_module.ScopedWorkflowStatusContextName(prefix, displayName, jobName, ev)
			if seen.Contains(ctxName) {
				continue
			}
			seen.Add(ctxName)
			contexts = append(contexts, ctxName)
		}
	}
	return contexts
}

func listSourceScopedWorkflowFiles(ctx *context.Context, repo *repo_model.Repository, configs map[string]*actions_model.ScopedWorkflowConfig) []scopedWorkflowInfo {
	rendered := make(container.Set[string], len(configs))
	files := make([]scopedWorkflowInfo, 0, len(configs))

	// An empty source repo (or one that fails to parse) has no live workflow files, but a previously-saved config may still linger;
	// fall through to surface those as orphan rows below so they remain clearable.
	if !repo.IsEmpty {
		_, parsed, err := actions_service.LoadParsedScopedWorkflows(ctx, repo)
		if err != nil {
			log.Error("scoped workflows settings: parse %s: %v", repo.RelativePath(), err)
		} else {
			for _, p := range parsed {
				info := scopedWorkflowInfo{
					EntryName:   p.EntryName,
					DisplayName: p.DisplayName,
					Contexts:    deriveScopedStatusContexts(repo.FullName(), p.DisplayName, p.Content, p.Events),
				}
				if cfg := configs[p.EntryName]; cfg != nil {
					info.Required = cfg.Required
					info.Patterns = strings.Join(cfg.Patterns, "\n")
				}
				rendered.Add(p.EntryName)
				files = append(files, info)
			}
		}
	}

	// Surface configs whose workflow file no longer exists on the source default branch as orphan rows.
	// A required orphan still gates merges (must-present), so the owner/admin must be able to see and clear it;
	// otherwise the only escape would be removing the whole source registration.
	orphans := make([]scopedWorkflowInfo, 0, len(configs))
	for name, cfg := range configs {
		if cfg == nil || rendered.Contains(name) {
			continue
		}
		orphans = append(orphans, scopedWorkflowInfo{
			EntryName:   name,
			DisplayName: name,
			Required:    cfg.Required,
			Patterns:    strings.Join(cfg.Patterns, "\n"),
			Missing:     true,
		})
	}
	// map iteration order is random; sort orphans for a stable settings page
	slices.SortFunc(orphans, func(a, b scopedWorkflowInfo) int { return strings.Compare(a.EntryName, b.EntryName) })
	return append(files, orphans...)
}

func ScopedWorkflowAdd(ctx *context.Context) {
	swCtx, err := getScopedWorkflowsCtx(ctx)
	if err != nil {
		ctx.ServerError("getScopedWorkflowsCtx", err)
		return
	}
	if ctx.Written() {
		return
	}

	repoName := ctx.FormString("repo_name")
	var repo *repo_model.Repository
	if swCtx.IsGlobal {
		// instance-level: the source may be any repo on the instance, identified by owner/name
		ownerName, name, ok := strings.Cut(repoName, "/")
		if !ok {
			ctx.JSONError(ctx.Tr("actions.scoped_workflows.source.not_found"))
			return
		}
		repo, err = repo_model.GetRepositoryByOwnerAndName(ctx, ownerName, name)
	} else {
		// owner-level: resolve within the owner, which also enforces that the source is one of the owner's own repositories
		repo, err = repo_model.GetRepositoryByName(ctx, swCtx.OwnerID, repoName)
	}
	if err != nil {
		ctx.JSONError(ctx.Tr("actions.scoped_workflows.source.not_found"))
		return
	}

	if err := actions_model.AddScopedWorkflowSource(ctx, swCtx.OwnerID, repo.ID); err != nil {
		ctx.ServerError("AddScopedWorkflowSource", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.scoped_workflows.source.add_success"))
	ctx.JSONRedirect(swCtx.RedirectLink)
}

func ScopedWorkflowSetRequired(ctx *context.Context) {
	swCtx, err := getScopedWorkflowsCtx(ctx)
	if err != nil {
		ctx.ServerError("getScopedWorkflowsCtx", err)
		return
	}
	if ctx.Written() {
		return
	}

	repoID := ctx.FormInt64("repo_id")

	// the source must be registered for this owner
	if _, err := actions_model.GetScopedWorkflowSource(ctx, swCtx.OwnerID, repoID); err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.JSONError(ctx.Tr("actions.scoped_workflows.source.not_found"))
		} else {
			ctx.ServerError("GetScopedWorkflowSource", err)
		}
		return
	}

	// Live workflow entry names on the source default branch, used to distinguish orphan configs (whose workflow file no longer exists) from live ones.
	sourceRepo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		ctx.ServerError("GetRepositoryByID", err)
		return
	}
	liveSet := make(container.Set[string])
	if !sourceRepo.IsEmpty { // an empty source has no live workflows
		_, parsed, err := actions_service.LoadParsedScopedWorkflows(ctx, sourceRepo)
		if err != nil {
			ctx.ServerError("LoadParsedScopedWorkflows", err)
			return
		}
		for _, p := range parsed {
			liveSet.Add(p.EntryName)
		}
	}

	// Every workflow row submits its ID in workflow_ids and its patterns (one per line) in required_patterns[<id>];
	// checked rows additionally submit their ID in required_workflow_ids.
	// A required workflow must have at least one pattern.
	requiredSet := make(container.Set[string])
	for _, workflowID := range ctx.FormStrings("required_workflow_ids") {
		requiredSet.Add(workflowID)
	}
	configs := make(map[string]*actions_model.ScopedWorkflowConfig)
	for _, workflowID := range ctx.FormStrings("workflow_ids") {
		patterns := parsePatternLines(ctx.FormString("required_patterns[" + workflowID + "]"))
		required := requiredSet.Contains(workflowID)
		if required && len(patterns) == 0 {
			ctx.JSONError(ctx.Tr("actions.scoped_workflows.required.patterns_empty"))
			return
		}
		// Keep a config only if it is required, or it is a still-existing.
		// An orphan (file no longer in the source) that is not required is dropped.
		if required || (liveSet.Contains(workflowID) && len(patterns) > 0) {
			configs[workflowID] = &actions_model.ScopedWorkflowConfig{Required: required, Patterns: patterns}
		}
	}
	if err := actions_model.SetScopedWorkflowSourceConfigs(ctx, swCtx.OwnerID, repoID, configs); err != nil {
		ctx.ServerError("SetScopedWorkflowSourceConfigs", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.scoped_workflows.required.update_success"))
	ctx.JSONRedirect(swCtx.RedirectLink)
}

func ScopedWorkflowRemove(ctx *context.Context) {
	swCtx, err := getScopedWorkflowsCtx(ctx)
	if err != nil {
		ctx.ServerError("getScopedWorkflowsCtx", err)
		return
	}
	if ctx.Written() {
		return
	}

	repoID := ctx.FormInt64("repo_id")
	if err := actions_model.RemoveScopedWorkflowSource(ctx, swCtx.OwnerID, repoID); err != nil {
		ctx.ServerError("RemoveScopedWorkflowSource", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("actions.scoped_workflows.source.remove_success"))
	ctx.JSONRedirect(swCtx.RedirectLink)
}
