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
func (issue *Issue) LoadProjects(ctx context.Context) (err error) {
	if issue.Projects == nil {
		err = db.GetEngine(ctx).Table("project").
			Join("INNER", "project_issue", "project.id=project_issue.project_id").
			Where("project_issue.issue_id = ?", issue.ID).Find(&issue.Projects)
	}
	return err
}

func (issue *Issue) projectIDs(ctx context.Context) []int64 {
	var ips []int64
	if err := db.GetEngine(ctx).Table("project_issue").Select("project_id").Where("issue_id=?", issue.ID).Find(&ips); err != nil {
		return nil
	}
	return ips
}

// ProjectColumnID return project column id if issue was assigned to one
func (issue *Issue) ProjectColumnID(ctx context.Context) int64 {
	var ip project_model.ProjectIssue
	has, err := db.GetEngine(ctx).Where("issue_id=?", issue.ID).Get(&ip)
	if err != nil || !has {
		return 0
	}
	return ip.ProjectColumnID
}

// LoadIssuesFromColumn load issues assigned to this column
func LoadIssuesFromColumn(ctx context.Context, b *project_model.Column, opts *IssuesOptions) (IssueList, error) {
	issueList, err := Issues(ctx, opts.Copy(func(o *IssuesOptions) {
		o.ProjectColumnID = b.ID
		o.ProjectIDs = append(o.ProjectIDs, b.ProjectID)
		o.SortType = "project-column-sorting"
	}))
	if err != nil {
		return nil, err
	}

	if b.Default {
		issues, err := Issues(ctx, &IssuesOptions{
			ProjectColumnID: db.NoConditionID,
			ProjectIDs:      []int64{b.ProjectID},
			SortType:        "project-column-sorting",
		})
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

// LoadIssuesFromColumnList load issues assigned to the columns
func LoadIssuesFromColumnList(ctx context.Context, bs project_model.ColumnList, opts *IssuesOptions) (map[int64]IssueList, error) {
	issuesMap := make(map[int64]IssueList, len(bs))
	for i := range bs {
		il, err := LoadIssuesFromColumn(ctx, bs[i], opts)
		if err != nil {
			return nil, err
		}
		issuesMap[bs[i].ID] = il
	}
	return issuesMap, nil
}

// IssueAssignOrRemoveProject changes the project associated with an issue
// If newProjectID is 0, the issue is removed from the project
func IssueAssignOrRemoveProject(ctx context.Context, issue *Issue, doer *user_model.User, newProjectID, newColumnID int64, action string) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		var oldProjectIDs []int64
		var err error
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
				newDefaultColumn, err := newProject.GetDefaultColumn(ctx)
				if err != nil {
					return err
				}
				newColumnID = newDefaultColumn.ID
			}
		}

		if action == "attach" || (action == "null" && newProjectID > 0) {
			if newProjectID == 0 {
				return nil
			}
			if newColumnID == 0 {
				panic("newColumnID must not be zero") // shouldn't happen
			}
			res := struct {
				IssueCount int64
			}{}
			if _, err := db.GetEngine(ctx).Select("count(*) as issue_count").Table("project_issue").
				Where("project_id=?", newProjectID).
				And("issue_id=?", issue.ID).
				Get(&res); err != nil {
				return err
			}
			if res.IssueCount == 0 {
				err = db.Insert(ctx, &project_model.ProjectIssue{
					IssueID:         issue.ID,
					ProjectID:       newProjectID,
					ProjectColumnID: newColumnID,
				})
				oldProjectIDs = []int64{0}
			} else {
				_, err = db.GetEngine(ctx).Where("issue_id=? AND project_id=?", issue.ID, newProjectID).Delete(&project_model.ProjectIssue{})
				oldProjectIDs = []int64{newProjectID}
				newProjectID = 0
			}
		} else if action == "detach" {
			_, err = db.GetEngine(ctx).Where("issue_id=? AND project_id=?", issue.ID, newProjectID).Delete(&project_model.ProjectIssue{})
			oldProjectIDs = append(oldProjectIDs, newProjectID)
			newProjectID = 0
		} else if action == "clear" || (action == "null" && newProjectID == 0) {
			if err = db.GetEngine(ctx).Table("project_issue").Select("project_id").Where("issue_id=?", issue.ID).Find(&oldProjectIDs); err != nil {
				return err
			}
			_, err = db.GetEngine(ctx).Where("issue_id=?", issue.ID).Delete(&project_model.ProjectIssue{})
			newProjectID = 0
		}

		for _, oldProjectID := range oldProjectIDs {
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
		return err
	})
}
