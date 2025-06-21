// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"strconv"

	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
)

var tmplWorkflows = templates.TplName("projects/workflows")

func Workflows(ctx *context.Context) {
	projectID := ctx.PathParamInt64("id")
	p, err := project_model.GetProjectByID(ctx, projectID)
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetProjectByID", err)
		}
		return
	}
	if p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(nil)
		return
	}

	ctx.Data["Title"] = ctx.Tr("projects.workflows")
	ctx.Data["PageIsWorkflows"] = true
	ctx.Data["PageIsProjects"] = true
	ctx.Data["PageIsProjectsWorkflows"] = true

	workflows, err := project_model.GetWorkflows(ctx, projectID)
	if err != nil {
		ctx.ServerError("GetWorkflows", err)
		return
	}
	ctx.Data["Workflows"] = workflows

	workflowIDStr := ctx.PathParam("workflow_id")
	var workflow *project_model.ProjectWorkflow
	if workflowIDStr == "" { // get first value workflow or the first workflow
		for _, wf := range workflows {
			if wf.ID > 0 {
				workflow = wf
				break
			}
		}
		if workflow.ID == 0 {
			workflow = workflows[0]
		}
	} else {
		workflowID, _ := strconv.ParseInt(workflowIDStr, 10, 64)
		if workflowID > 0 {
			var err error
			workflow, err = project_model.GetWorkflowByID(ctx, workflowID)
			if err != nil {
				ctx.ServerError("GetWorkflowByID", err)
				return
			}
			ctx.Data["CurWorkflow"] = workflow
		} else {
			workflow = project_model.GetWorkflowDefaultValue(workflowIDStr)
		}
	}
	ctx.Data["CurWorkflow"] = workflow

	ctx.HTML(200, tmplWorkflows)
}
