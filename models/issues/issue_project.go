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
	if len(issue.Projects) == 0 {
		err = db.GetEngine(ctx).Table("project").
			Join("INNER", "project_issue", "project.id=project_issue.project_id").
			Where("project_issue.issue_id = ?", issue.ID).Find(&issue.Projects)
	}
	return err
}

func (issue *Issue) projectIDs(ctx context.Context) []int64 {
	var ids []int64
	if err := db.GetEngine(ctx).Table("project_issue").Where("issue_id=?", issue.ID).Cols("project_id").Find(&ids); err != nil {
		return nil
	}

	return ids
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
func IssueAssignOrRemoveProject(ctx context.Context, issue *Issue, doer *user_model.User, newProjectIDs []int64, newColumnID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		oldProjectIDs := issue.projectIDs(ctx)

		if err := issue.LoadRepo(ctx); err != nil {
			return err
		}

		projectDB := db.GetEngine(ctx).Where("project_issue.issue_id=?", issue.ID)
		newProjectIDs, oldProjectIDs := util.DiffSlice(oldProjectIDs, newProjectIDs)

		if len(oldProjectIDs) > 0 {
			if _, err := projectDB.Where("issue_id=?", issue.ID).In("project_id", oldProjectIDs).Delete(&project_model.ProjectIssue{}); err != nil {
				return err
			}
			for _, pID := range oldProjectIDs {
				if _, err := CreateComment(ctx, &CreateCommentOptions{
					Type:         CommentTypeProject,
					Doer:         doer,
					Repo:         issue.Repo,
					Issue:        issue,
					OldProjectID: pID,
					ProjectID:    0,
				}); err != nil {
					return err
				}
			}
			return nil
		}

		res := struct {
			MaxSorting int64
			IssueCount int64
		}{}
		if _, err := projectDB.Select("max(sorting) as max_sorting, count(*) as issue_count").Table("project_issue").
			In("project_id", newProjectIDs).
			And("project_board_id=?", newColumnID).
			Get(&res); err != nil {
			return err
		}
		newSorting := util.Iif(res.IssueCount > 0, res.MaxSorting+1, 0)

		pi := make([]*project_model.ProjectIssue, 0, len(newProjectIDs))

		for _, pID := range newProjectIDs {
			newProject, err := project_model.GetProjectByID(ctx, pID)
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
				if newColumnID == 0 {
					panic("newColumnID must not be zero") // shouldn't happen
				}
			}

			pi = append(pi, &project_model.ProjectIssue{
				IssueID:         issue.ID,
				ProjectID:       pID,
				ProjectColumnID: newColumnID,
				Sorting:         newSorting,
			})

			if _, err := CreateComment(ctx, &CreateCommentOptions{
				Type:         CommentTypeProject,
				Doer:         doer,
				Repo:         issue.Repo,
				Issue:        issue,
				OldProjectID: 0,
				ProjectID:    pID,
			}); err != nil {
				return err
			}
		}

		if len(pi) > 0 {
			return db.Insert(ctx, pi)
		}

		return nil
	})
}
