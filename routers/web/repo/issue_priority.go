// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	issue_service "code.gitea.io/gitea/services/issue"
)

// InitializeLabels init labels for a repository
func InitializePriorities(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.InitializeLabelsForm)
	if ctx.HasError() {
		ctx.Redirect(ctx.Repo.RepoLink + "/labels")
		return
	}

	if err := repo_module.InitializeLabels(ctx, ctx.Repo.Repository.ID, form.TemplateName, false); err != nil {
		if repo_module.IsErrIssueLabelTemplateLoad(err) {
			originalErr := err.(repo_module.ErrIssueLabelTemplateLoad).OriginalError
			ctx.Flash.Error(ctx.Tr("repo.issues.label_templates.fail_to_load_file", form.TemplateName, originalErr))
			ctx.Redirect(ctx.Repo.RepoLink + "/labels")
			return
		}
		ctx.ServerError("InitializeLabels", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/labels")
}

// NewLabel create new label for repository
func NewPriority(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateLabelForm)
	ctx.Data["Title"] = ctx.Tr("repo.labels")
	ctx.Data["PageIsLabels"] = true

	if ctx.HasError() {
		ctx.Flash.Error(ctx.Data["ErrorMsg"].(string))
		ctx.Redirect(ctx.Repo.RepoLink + "/labels")
		return
	}

	p := &issues_model.Priority{
		RepoID:      ctx.Repo.Repository.ID,
		Name:        form.Title,
		Description: form.Description,
		Color:       form.Color,
		Weight:      form.Weight,
	}
	if err := issues_model.NewPriority(ctx, p); err != nil {
		ctx.ServerError("NewPriority", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/labels")
}

// UpdateLabel update a label's name and color
func UpdatePriority(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateLabelForm)
	p, err := issues_model.GetPriorityInRepoByID(ctx, ctx.Repo.Repository.ID, form.ID)
	if err != nil {
		switch {
		case issues_model.IsErrRepoPriorityNotExist(err):
			ctx.Error(http.StatusNotFound)
		default:
			ctx.ServerError("UpdatePriority", err)
		}
		return
	}

	p.Name = form.Title
	p.Description = form.Description
	p.Weight = form.Weight
	p.Color = form.Color
	if err := issues_model.UpdatePriority(p); err != nil {
		ctx.ServerError("UpdatePriority", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/labels")
}

// DeleteLabel delete a label
func DeletePriority(ctx *context.Context) {
	if err := issues_model.DeletePriority(ctx.Repo.Repository.ID, ctx.FormInt64("id")); err != nil {
		ctx.Flash.Error("DeletePriority: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.issues.priority_deletion_success"))
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/labels",
	})
}

// UpdateIssueLabel change issue's labels
func UpdateIssuePriority(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	switch action := ctx.FormString("action"); action {
	case "clear":
		for _, issue := range issues {
			if err := issue_service.ClearLabels(issue, ctx.Doer); err != nil {
				ctx.ServerError("ClearLabels", err)
				return
			}
		}
	case "attach", "detach", "toggle":
		label, err := issues_model.GetLabelByID(ctx, ctx.FormInt64("id"))
		if err != nil {
			if issues_model.IsErrRepoLabelNotExist(err) {
				ctx.Error(http.StatusNotFound, "GetLabelByID")
			} else {
				ctx.ServerError("GetLabelByID", err)
			}
			return
		}

		if action == "toggle" {
			// detach if any issues already have label, otherwise attach
			action = "attach"
			for _, issue := range issues {
				if issues_model.HasIssueLabel(ctx, issue.ID, label.ID) {
					action = "detach"
					break
				}
			}
		}

		if action == "attach" {
			for _, issue := range issues {
				if err = issue_service.AddLabel(issue, ctx.Doer, label); err != nil {
					ctx.ServerError("AddLabel", err)
					return
				}
			}
		} else {
			for _, issue := range issues {
				if err = issue_service.RemoveLabel(issue, ctx.Doer, label); err != nil {
					ctx.ServerError("RemoveLabel", err)
					return
				}
			}
		}
	default:
		log.Warn("Unrecognized action: %s", action)
		ctx.Error(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}
