// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strconv"

	"gitea.dev/actionslib/pkg/model"
	actions_model "gitea.dev/models/actions"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/util"
	actions_service "gitea.dev/services/actions"
	context_module "gitea.dev/services/context"
)

func DisableWorkflowFile(ctx *context_module.Context) {
	disableOrEnableWorkflowFile(ctx, false)
}

func EnableWorkflowFile(ctx *context_module.Context) {
	disableOrEnableWorkflowFile(ctx, true)
}

func disableOrEnableWorkflowFile(ctx *context_module.Context, isEnable bool) {
	workflow := ctx.FormString("workflow")
	if len(workflow) == 0 {
		ctx.JSONError("workflow is required")
		return
	}

	cfgUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()

	scopedRepoID := ctx.FormInt64("scoped_workflow_source_repo_id")
	if scopedRepoID > 0 {
		if !isEnable {
			// a required scoped workflow can never be opted out
			required, err := actions_model.IsScopedWorkflowRequired(ctx, ctx.Repo.Repository.OwnerID, scopedRepoID, workflow)
			if err != nil {
				ctx.ServerError("IsScopedWorkflowRequired", err)
				return
			}
			if required {
				ctx.JSONError(ctx.Locale.Tr("actions.workflow.scoped_required_cannot_disable"))
				return
			}
			cfg.DisableScopedWorkflow(scopedRepoID, workflow)
		} else {
			cfg.EnableScopedWorkflow(scopedRepoID, workflow)
		}
	} else if isEnable {
		cfg.EnableWorkflow(workflow)
	} else {
		cfg.DisableWorkflow(workflow)
	}

	if err := repo_model.UpdateRepoUnitConfig(ctx, cfgUnit); err != nil {
		ctx.ServerError("UpdateRepoUnit", err)
		return
	}
	actions_service.RecordWorkflowToggle(ctx, ctx.Repo.Repository, workflow, isEnable)

	if isEnable {
		ctx.Flash.Success(ctx.Tr("actions.workflow.enable_success", workflow))
	} else {
		ctx.Flash.Success(ctx.Tr("actions.workflow.disable_success", workflow))
	}

	redirectURL := actionsListRedirectURL(ctx.Repo.RepoLink, workflow, ctx.FormString("scoped_workflow_source_repo_id"),
		ctx.FormString("actor"), ctx.FormString("status"), ctx.FormString("branch"))
	ctx.JSONRedirect(redirectURL)
}

func Run(ctx *context_module.Context) {
	redirectURL := actionsListRedirectURL(ctx.Repo.RepoLink, ctx.FormString("workflow"), ctx.FormString("scoped_workflow_source_repo_id"),
		ctx.FormString("actor"), ctx.FormString("status"), ctx.FormString("branch"))

	workflowID := ctx.FormString("workflow")
	if len(workflowID) == 0 {
		ctx.ServerError("workflow", nil)
		return
	}

	ref := ctx.FormString("ref")
	if len(ref) == 0 {
		ctx.ServerError("ref", nil)
		return
	}
	sourceRepoID := ctx.FormInt64("scoped_workflow_source_repo_id")
	_, err := actions_service.DispatchActionWorkflow(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.GitRepo, workflowID, ref, sourceRepoID, func(workflowDispatch *model.WorkflowDispatch, inputs map[string]any) error {
		for name, config := range workflowDispatch.Inputs {
			value := ctx.Req.PostFormValue(name)
			if config.Type == "boolean" {
				inputs[name] = strconv.FormatBool(ctx.FormBool(name))
			} else if value != "" {
				inputs[name] = value
			} else {
				inputs[name] = config.Default
			}
		}
		return nil
	})
	if err != nil {
		if errTr := util.ErrorAsTranslatable(err); errTr != nil {
			ctx.Flash.Error(errTr.Translate(ctx.Locale))
			ctx.Redirect(redirectURL)
		} else {
			ctx.ServerError("DispatchActionWorkflow", err)
		}
		return
	}

	ctx.Flash.Success(ctx.Tr("actions.workflow.run_success", workflowID))
	ctx.Redirect(redirectURL)
}
