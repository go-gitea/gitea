// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

const (
	tplLabels base.TplName = "repo/issue/labels"
)

// Labels render issue's labels page
func Labels(ctx *context.Context) {
	MustEnableIssues(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Title"] = ctx.Tr("repo.labels")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["PageIsLabels"] = true
	ctx.Data["RequireMinicolors"] = true
	ctx.Data["LabelTemplates"] = models.LabelTemplates
	ctx.HTML(200, tplLabels)
}

// InitializeLabels init labels for a repository
func InitializeLabels(ctx *context.Context, form auth.InitializeLabelsForm) {
	if ctx.HasError() {
		ctx.Redirect(ctx.Repo.RepoLink + "/labels")
		return
	}
	list, err := models.GetLabelTemplateFile(form.TemplateName)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("repo.issues.label_templates.fail_to_load_file", form.TemplateName, err))
		ctx.Redirect(ctx.Repo.RepoLink + "/labels")
		return
	}

	labels := make([]*models.Label, len(list))
	for i := 0; i < len(list); i++ {
		labels[i] = &models.Label{
			RepoID: ctx.Repo.Repository.ID,
			Name:   list[i][0],
			Color:  list[i][1],
		}
	}
	if err := models.NewLabels(labels...); err != nil {
		ctx.Handle(500, "NewLabels", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/labels")
}

// RetrieveLabels find all the labels of a repository
func RetrieveLabels(ctx *context.Context) {
	labels, err := models.GetLabelsByRepoID(ctx.Repo.Repository.ID, ctx.Query("sort"))
	if err != nil {
		ctx.Handle(500, "RetrieveLabels.GetLabels", err)
		return
	}
	for _, l := range labels {
		l.CalOpenIssues()
	}
	ctx.Data["Labels"] = labels
	ctx.Data["NumLabels"] = len(labels)
	ctx.Data["SortType"] = ctx.Query("sort")
}

// NewLabel create new label for repository
func NewLabel(ctx *context.Context, form auth.CreateLabelForm) {
	ctx.Data["Title"] = ctx.Tr("repo.labels")
	ctx.Data["PageIsLabels"] = true

	if ctx.HasError() {
		ctx.Flash.Error(ctx.Data["ErrorMsg"].(string))
		ctx.Redirect(ctx.Repo.RepoLink + "/labels")
		return
	}

	l := &models.Label{
		RepoID: ctx.Repo.Repository.ID,
		Name:   form.Title,
		Color:  form.Color,
	}
	if err := models.NewLabel(l); err != nil {
		ctx.Handle(500, "NewLabel", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/labels")
}

// UpdateLabel update a label's name and color
func UpdateLabel(ctx *context.Context, form auth.CreateLabelForm) {
	l, err := models.GetLabelByID(form.ID)
	if err != nil {
		switch {
		case models.IsErrLabelNotExist(err):
			ctx.Error(404)
		default:
			ctx.Handle(500, "UpdateLabel", err)
		}
		return
	}

	l.Name = form.Title
	l.Color = form.Color
	if err := models.UpdateLabel(l); err != nil {
		ctx.Handle(500, "UpdateLabel", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/labels")
}

// DeleteLabel delete a label
func DeleteLabel(ctx *context.Context) {
	if err := models.DeleteLabel(ctx.Repo.Repository.ID, ctx.QueryInt64("id")); err != nil {
		ctx.Flash.Error("DeleteLabel: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.issues.label_deletion_success"))
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/labels",
	})
	return
}

// UpdateIssueLabel change issue's labels
func UpdateIssueLabel(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	switch action := ctx.Query("action"); action {
	case "clear":
		for _, issue := range issues {
			if err := issue.ClearLabels(ctx.User); err != nil {
				ctx.Handle(500, "ClearLabels", err)
				return
			}
		}
	case "attach", "detach", "toggle":
		label, err := models.GetLabelByID(ctx.QueryInt64("id"))
		if err != nil {
			if models.IsErrLabelNotExist(err) {
				ctx.Error(404, "GetLabelByID")
			} else {
				ctx.Handle(500, "GetLabelByID", err)
			}
			return
		}

		if action == "toggle" {
			anyHaveLabel := false
			for _, issue := range issues {
				if issue.HasLabel(label.ID) {
					anyHaveLabel = true
					break
				}
			}
			if anyHaveLabel {
				action = "detach"
			} else {
				action = "attach"
			}
		}

		if action == "attach" {
			for _, issue := range issues {
				if err = issue.AddLabel(ctx.User, label); err != nil {
					ctx.Handle(500, "AddLabel", err)
					return
				}
			}
		} else {
			for _, issue := range issues {
				if err = issue.RemoveLabel(ctx.User, label); err != nil {
					ctx.Handle(500, "RemoveLabel", err)
					return
				}
			}
		}
	default:
		log.Warn("Unrecognized action: %s", action)
		ctx.Error(500)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}
