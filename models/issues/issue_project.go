// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
)

// LoadProject load the project the issue was assigned to
func (issue *Issue) LoadProject(ctx context.Context) (err error) {
	if issue.Project == nil {
		var p project_model.Project
		has, err := db.GetEngine(ctx).Table("project").
			Join("INNER", "project_issue", "project.id=project_issue.project_id").
			Where("project_issue.issue_id = ?", issue.ID).Get(&p)
		if err != nil {
			return err
		} else if has {
			issue.Project = &p
		}
	}
	return err
}

func (issue *Issue) projectID(ctx context.Context) int64 {
	var ip project_model.ProjectIssue
	has, err := db.GetEngine(ctx).Where("issue_id=?", issue.ID).Get(&ip)
	if err != nil || !has {
		return 0
	}
	return ip.ProjectID
}

// ProjectColumnID return project column id if issue was assigned to one
func (issue *Issue) ProjectColumnID(ctx context.Context) (int64, error) {
	var ip project_model.ProjectIssue
	has, err := db.GetEngine(ctx).Where("issue_id=?", issue.ID).Get(&ip)
	if err != nil {
		return 0, err
	} else if !has {
		return 0, nil
	}
	return ip.ProjectColumnID, nil
}

func LoadProjectIssueColumnMap(ctx context.Context, projectID, defaultColumnID int64) (map[int64]int64, error) {
	issues := make([]project_model.ProjectIssue, 0)
	if err := db.GetEngine(ctx).Where("project_id=?", projectID).Find(&issues); err != nil {
		return nil, err
	}
	result := make(map[int64]int64, len(issues))
	for _, issue := range issues {
		if issue.ProjectColumnID == 0 {
			issue.ProjectColumnID = defaultColumnID
		}
		result[issue.IssueID] = issue.ProjectColumnID
	}
	return result, nil
}

// LoadIssuesFromColumn load issues assigned to this column
func LoadIssuesFromColumn(ctx context.Context, b *project_model.Column, opts *IssuesOptions) (IssueList, error) {
	issueList, err := Issues(ctx, opts.Copy(func(o *IssuesOptions) {
		o.ProjectColumnID = b.ID
		o.ProjectID = b.ProjectID
		o.SortType = "project-column-sorting"
	}))
	if err != nil {
		return nil, err
	}

	if b.Default {
		issues, err := Issues(ctx, opts.Copy(func(o *IssuesOptions) {
			o.ProjectColumnID = db.NoConditionID
			o.ProjectID = b.ProjectID
			o.SortType = "project-column-sorting"
		}))
		if err != nil {
			return nil, err
		}
		issueList = append(issueList, issues...)
	}

	if err := issueList.LoadComments(ctx); err != nil {
		return nil, err
	}

	return issueList, nil
}

// IssueAssignOrRemoveProject changes the project associated with an issue
// If newProjectID is 0, the issue is removed from the project
func IssueAssignOrRemoveProject(ctx context.Context, issue *Issue, doer *user_model.User, newProjectID, newColumnID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		oldProjectID := issue.projectID(ctx)

		if err := issue.LoadRepo(ctx); err != nil {
			return err
		}

		// Only check if we add a new project and not remove it.
		if newProjectID > 0 {
			newProject, err := project_model.GetProjectByID(ctx, newProjectID)
			if err != nil {
				return err
			}
			if !newProject.CanBeAccessedByOwnerRepo(issue.Repo.OwnerID, issue.Repo) {
				return util.NewPermissionDeniedErrorf("issue %d can't be accessed by project %d", issue.ID, newProject.ID)
			}
			if newColumnID == 0 {
				newDefaultColumn, err := newProject.MustDefaultColumn(ctx)
				if err != nil {
					return err
				}
				newColumnID = newDefaultColumn.ID
			}
		}

		if _, err := db.GetEngine(ctx).Where("project_issue.issue_id=?", issue.ID).Delete(&project_model.ProjectIssue{}); err != nil {
			return err
		}

		if oldProjectID > 0 || newProjectID > 0 {
			if _, err := CreateComment(ctx, &CreateCommentOptions{
				Type:         CommentTypeProject,
				Doer:         doer,
				Repo:         issue.Repo,
				Issue:        issue,
				OldProjectID: oldProjectID,
				ProjectID:    newProjectID,
			}); err != nil {
				return err
			}
		}
		if newProjectID == 0 {
			return nil
		}
		if newColumnID == 0 {
			panic("newColumnID must not be zero") // shouldn't happen
		}

		res := struct {
			MaxSorting int64
			IssueCount int64
		}{}
		if _, err := db.GetEngine(ctx).Select("max(sorting) as max_sorting, count(*) as issue_count").Table("project_issue").
			Where("project_id=?", newProjectID).
			And("project_board_id=?", newColumnID).
			Get(&res); err != nil {
			return err
		}
		newSorting := util.Iif(res.IssueCount > 0, res.MaxSorting+1, 0)
		return db.Insert(ctx, &project_model.ProjectIssue{
			IssueID:         issue.ID,
			ProjectID:       newProjectID,
			ProjectColumnID: newColumnID,
			Sorting:         newSorting,
		})
	})
}
