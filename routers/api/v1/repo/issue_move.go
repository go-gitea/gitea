// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"
	"strings"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	perm_model "gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	project_model "gitea.dev/models/project"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

// MoveIssueAPI moves an issue to another repository
func MoveIssueAPI(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/move issue issueMoveIssue
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
	//   description: index of the issue
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
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.MoveIssueOption)

	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if issue.IsPull {
		ctx.APIError(http.StatusUnprocessableEntity, "Cannot move a pull request to another repository.")
		return
	}

	newRepo, err := parseTargetRepo(ctx, form.NewRepo)
	if err != nil {
		return // error already written
	}

	if newRepo.ID == ctx.Repo.Repository.ID {
		ctx.APIError(http.StatusUnprocessableEntity, "Issue is already in this repository.")
		return
	}

	canWrite, err := access_model.HasAccessUnit(ctx, ctx.Doer, newRepo, unit.TypeIssues, perm_model.AccessModeWrite)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if !canWrite {
		ctx.APIError(http.StatusForbidden, "You don't have permission to create issues in the target repository.")
		return
	}

	// Collect original labels, milestone, project before moving
	var origLabels []*issues_model.Label
	var origMilestoneName string
	var origProject *project_model.Project

	origLabels, err = issues_model.GetLabelsByIssueID(ctx, issue.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if issue.MilestoneID > 0 {
		origMilestone, err := issues_model.GetMilestoneByRepoID(ctx, issue.RepoID, issue.MilestoneID)
		if err == nil {
			origMilestoneName = origMilestone.Name
		}
	}

	if err := issue.LoadProject(ctx); err == nil && issue.Project != nil {
		origProject = issue.Project
	}

	oldRepoID := issue.RepoID
	dbCtx, committer, err := db.TxContext(ctx)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	defer committer.Close()

	newIndex, err := db.GetNextResourceIndex(dbCtx, "issue_index", newRepo.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	issue.RepoID = newRepo.ID
	issue.Index = newIndex
	if _, err := db.GetEngine(dbCtx).ID(issue.ID).Cols("repo_id", "`index`").Update(issue); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if _, err := db.GetEngine(dbCtx).Where("issue_id = ?", issue.ID).Delete(&issues_model.IssueLabel{}); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if issue.MilestoneID > 0 {
		if _, err := db.GetEngine(dbCtx).ID(issue.ID).Cols("milestone_id").Update(&issues_model.Issue{MilestoneID: 0}); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	if _, err := db.GetEngine(dbCtx).Where("issue_id = ?", issue.ID).Delete(&project_model.ProjectIssue{}); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if err := issues_model.DecrRepoIssueNumbers(dbCtx, oldRepoID, issue.IsPull, true, issue.IsClosed); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if err := issues_model.IncrRepoIssueNumbers(dbCtx, newRepo.ID, issue.IsPull, true); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if issue.IsClosed {
		if err := issues_model.IncrRepoIssueNumbers(dbCtx, newRepo.ID, issue.IsPull, false); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	if err := committer.Commit(); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	// Best-effort copy after commit
	if form.CopyLabels && len(origLabels) > 0 {
		apiCopyLabelsToIssue(ctx, issue, origLabels, newRepo)
	}

	if form.CopyMilestone && origMilestoneName != "" {
		apiCopyMilestoneToIssue(ctx, issue, origMilestoneName, newRepo)
	}

	if form.CopyProject && origProject != nil {
		apiCopyProjectToIssue(ctx, issue, origProject, newRepo)
	}

	log.Info("Issue [%d] moved from repo [%d] to repo [%d] by user [%d]", issue.ID, oldRepoID, newRepo.ID, ctx.Doer.ID)

	ctx.JSON(http.StatusOK, convert.ToAPIIssue(ctx, ctx.Doer, issue))
}

// CopyIssueAPI copies an issue to another repository
func CopyIssueAPI(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/copy issue issueCopyIssue
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
	//   description: index of the issue
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
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CopyIssueOption)

	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if issue.IsPull {
		ctx.APIError(http.StatusUnprocessableEntity, "Cannot copy a pull request to another repository.")
		return
	}

	newRepo, err := parseTargetRepo(ctx, form.NewRepo)
	if err != nil {
		return
	}

	canWrite, err := access_model.HasAccessUnit(ctx, ctx.Doer, newRepo, unit.TypeIssues, perm_model.AccessModeWrite)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if !canWrite {
		ctx.APIError(http.StatusForbidden, "You don't have permission to create issues in the target repository.")
		return
	}

	dbCtx, committer, err := db.TxContext(ctx)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	defer committer.Close()

	newIndex, err := db.GetNextResourceIndex(dbCtx, "issue_index", newRepo.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	newIssue := &issues_model.Issue{
		RepoID:   newRepo.ID,
		Index:    newIndex,
		PosterID: issue.PosterID,
		Title:    issue.Title,
		Content:  issue.Content,
	}
	if _, err := db.GetEngine(dbCtx).Insert(newIssue); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if err := issues_model.IncrRepoIssueNumbers(dbCtx, newRepo.ID, false, true); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if err := committer.Commit(); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	// Best-effort: set dependency
	if form.DepType == "original_blocks_copy" {
		_ = issues_model.CreateIssueDependency(ctx, ctx.Doer, newIssue, issue)
	} else if form.DepType == "copy_blocks_original" {
		_ = issues_model.CreateIssueDependency(ctx, ctx.Doer, issue, newIssue)
	}

	// Best-effort: copy labels
	if form.CopyLabels {
		origLabels, err := issues_model.GetLabelsByIssueID(ctx, issue.ID)
		if err == nil && len(origLabels) > 0 {
			apiCopyLabelsToIssue(ctx, newIssue, origLabels, newRepo)
		}
	}

	// Best-effort: copy milestone
	if form.CopyMilestone && issue.MilestoneID > 0 {
		origMilestone, err := issues_model.GetMilestoneByRepoID(ctx, issue.RepoID, issue.MilestoneID)
		if err == nil {
			apiCopyMilestoneToIssue(ctx, newIssue, origMilestone.Name, newRepo)
		}
	}

	// Best-effort: copy project
	if form.CopyProject {
		if err := issue.LoadProject(ctx); err == nil && issue.Project != nil {
			apiCopyProjectToIssue(ctx, newIssue, issue.Project, newRepo)
		}
	}

	log.Info("Issue [%d] copied to repo [%d] as issue [%d] by user [%d]", issue.ID, newRepo.ID, newIssue.ID, ctx.Doer.ID)

	ctx.JSON(http.StatusCreated, convert.ToAPIIssue(ctx, ctx.Doer, newIssue))
}

// parseTargetRepo parses "owner/repo" and returns the repository, writing errors to ctx
func parseTargetRepo(ctx *context.APIContext, fullName string) (*repo_model.Repository, error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		ctx.APIError(http.StatusUnprocessableEntity, "Target repository must be in 'owner/repo' format.")
		return nil, fmt.Errorf("invalid format")
	}

	newRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, parts[0], parts[1])
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil, err
	}
	return newRepo, nil
}

// apiCopyLabelsToIssue copies labels from origLabels to issue in targetRepo
func apiCopyLabelsToIssue(ctx *context.APIContext, issue *issues_model.Issue, origLabels []*issues_model.Label, targetRepo *repo_model.Repository) {
	for _, origLabel := range origLabels {
		var labelID int64
		if origLabel.BelongsToOrg() && origLabel.OrgID == targetRepo.OwnerID {
			labelID = origLabel.ID
		} else {
			found, err := issues_model.GetLabelInRepoByName(ctx, targetRepo.ID, origLabel.Name)
			if err == nil {
				labelID = found.ID
			} else {
				newLabel := &issues_model.Label{
					RepoID:      targetRepo.ID,
					Name:        origLabel.Name,
					Exclusive:   origLabel.Exclusive,
					Description: origLabel.Description,
					Color:       origLabel.Color,
				}
				if err := issues_model.NewLabel(ctx, newLabel); err != nil {
					continue
				}
				labelID = newLabel.ID
			}
		}
		_, _ = db.GetEngine(ctx).Insert(&issues_model.IssueLabel{IssueID: issue.ID, LabelID: labelID})
	}
}

// apiCopyMilestoneToIssue copies a milestone by name to issue in targetRepo
func apiCopyMilestoneToIssue(ctx *context.APIContext, issue *issues_model.Issue, milestoneName string, targetRepo *repo_model.Repository) {
	newMilestone, err := issues_model.GetMilestoneByRepoIDANDName(ctx, targetRepo.ID, milestoneName)
	if err != nil {
		newMilestone = &issues_model.Milestone{
			RepoID: targetRepo.ID,
			Name:   milestoneName,
		}
		if err := issues_model.NewMilestone(ctx, newMilestone); err != nil {
			return
		}
	}
	_, _ = db.GetEngine(ctx).ID(issue.ID).Cols("milestone_id").Update(&issues_model.Issue{MilestoneID: newMilestone.ID})
}

// apiCopyProjectToIssue copies a project to issue in targetRepo
func apiCopyProjectToIssue(ctx *context.APIContext, issue *issues_model.Issue, origProject *project_model.Project, targetRepo *repo_model.Repository) {
	var targetProjectID int64
	if origProject.Type == project_model.TypeOrganization && origProject.OwnerID == targetRepo.OwnerID {
		targetProjectID = origProject.ID
	} else {
		projects, err := db.Find[project_model.Project](ctx, project_model.SearchOptions{
			RepoID:   targetRepo.ID,
			IsClosed: optional.Some(false),
			Type:     project_model.TypeRepository,
		})
		if err == nil {
			for _, p := range projects {
				if p.Title == origProject.Title {
					targetProjectID = p.ID
					break
				}
			}
		}
		if targetProjectID == 0 {
			newProject := &project_model.Project{
				Title:     origProject.Title,
				RepoID:    targetRepo.ID,
				CreatorID: ctx.Doer.ID,
				Type:      project_model.TypeRepository,
			}
			if err := project_model.NewProject(ctx, newProject); err == nil {
				targetProjectID = newProject.ID
			}
		}
	}
	if targetProjectID > 0 {
		targetProject, err := project_model.GetProjectByID(ctx, targetProjectID)
		if err == nil {
			defaultColumn, err := targetProject.MustDefaultColumn(ctx)
			if err == nil {
				_, _ = db.GetEngine(ctx).Insert(&project_model.ProjectIssue{
					IssueID:         issue.ID,
					ProjectID:       targetProjectID,
					ProjectColumnID: defaultColumn.ID,
				})
			}
		}
	}
}
