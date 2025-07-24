// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"strconv"

	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
)

var (
	tmplRepoWorkflows = templates.TplName("repo/projects/workflows")
	tmplOrgWorkflows  = templates.TplName("org/projects/workflows")
)

func Workflows(ctx *context.Context) {
	ctx.Data["WorkflowEvents"] = project_model.GetWorkflowEvents()

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
	if p.Type == project_model.TypeRepository && p.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound(nil)
		return
	}
	if (p.Type == project_model.TypeOrganization || p.Type == project_model.TypeIndividual) && p.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound(nil)
		return
	}

	ctx.Data["Title"] = ctx.Tr("projects.workflows")
	ctx.Data["PageIsWorkflows"] = true
	ctx.Data["PageIsProjects"] = true
	ctx.Data["PageIsProjectsWorkflows"] = true
	ctx.Data["Project"] = p

	workflows, err := project_model.FindWorkflowEvents(ctx, projectID)
	if err != nil {
		ctx.ServerError("GetWorkflows", err)
		return
	}
	for _, wf := range workflows {
		wf.Project = p
	}
	ctx.Data["Workflows"] = workflows

	workflowIDStr := ctx.PathParam("workflow_id")
	ctx.Data["workflowIDStr"] = workflowIDStr
	var curWorkflow *project_model.Workflow
	if workflowIDStr == "" { // get first value workflow or the first workflow
		for _, wf := range workflows {
			if wf.ID > 0 {
				curWorkflow = wf
				break
			}
		}
	} else {
		workflowID, _ := strconv.ParseInt(workflowIDStr, 10, 64)
		if workflowID > 0 {
			for _, wf := range workflows {
				if wf.ID == workflowID {
					curWorkflow = wf
					break
				}
			}
		}
	}
	ctx.Data["CurWorkflow"] = curWorkflow

	if p.Type == project_model.TypeRepository {
		ctx.HTML(200, tmplRepoWorkflows)
	} else {
		ctx.HTML(200, tmplOrgWorkflows)
	}
}
