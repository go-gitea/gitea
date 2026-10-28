// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strings"

	repo_model "gitea.dev/models/repo"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	issue_service "gitea.dev/services/issue"
)

func getMoveIssueTargetRepo(ctx *context.APIContext, fullName string) *repo_model.Repository {
	ownerName, repoName, ok := strings.Cut(fullName, "/")
	if !ok || ownerName == "" || repoName == "" {
		ctx.APIError(http.StatusUnprocessableEntity, "target_repo must be in \"owner/repo\" format")
		return nil
	}
	targetRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, ownerName, repoName)
	if repo_model.IsErrRepoNotExist(err) {
		ctx.APIError(http.StatusUnprocessableEntity, "target repository does not exist")
		return nil
	} else if err != nil {
		ctx.APIErrorInternal(err)
		return nil
	}
	return targetRepo
}

// MoveIssue moves an issue to another repository
func MoveIssue(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/move issue issueMove
	// ---
	// summary: Move an issue to another repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the issue to move
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/MoveIssueOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Issue"
	//   default:
	//     "$ref": "#/responses/error"

	form := web.GetForm[*api.MoveIssueOption](ctx)
	issue := getRepoIssueByIndexForDoer(ctx, ctx.PathParamInt64("index"))
	if issue == nil {
		return
	}
	targetRepo := getMoveIssueTargetRepo(ctx, form.TargetRepo)
	if targetRepo == nil {
		return
	}
	if err := issue_service.MoveIssue(ctx, ctx.Doer, issue, targetRepo, issue_service.MoveIssueOptions{
		KeepLabels:    form.KeepLabels,
		KeepMilestone: form.KeepMilestone,
		KeepProjects:  form.KeepProjects,
	}); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToAPIIssue(ctx, ctx.Doer, issue))
}

// CopyIssue copies an issue to another repository
func CopyIssue(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/copy issue issueCopy
	// ---
	// summary: Copy an issue to another repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the issue to copy
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CopyIssueOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Issue"
	//   default:
	//     "$ref": "#/responses/error"

	form := web.GetForm[*api.CopyIssueOption](ctx)
	issue := getRepoIssueByIndexForDoer(ctx, ctx.PathParamInt64("index"))
	if issue == nil {
		return
	}
	targetRepo := getMoveIssueTargetRepo(ctx, form.TargetRepo)
	if targetRepo == nil {
		return
	}
	newIssue, err := issue_service.CopyIssue(ctx, ctx.Doer, issue, targetRepo, issue_service.CopyIssueOptions{
		MoveIssueOptions: issue_service.MoveIssueOptions{
			KeepLabels:    form.KeepLabels,
			KeepMilestone: form.KeepMilestone,
			KeepProjects:  form.KeepProjects,
		},
		CopyAttachments: form.CopyAttachments,
		CopyAssignees:   form.CopyAssignees,
		Dependency:      issue_service.CopyIssueDependency(form.Dependency),
	})
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToAPIIssue(ctx, ctx.Doer, newIssue))
}
