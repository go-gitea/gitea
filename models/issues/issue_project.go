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

// LoadProjects loads all projects the issue is assigned to
func (issue *Issue) LoadProjects(ctx context.Context) (err error) {
	if !issue.isProjectsLoaded {
		err = db.GetEngine(ctx).Table("project").
			Join("INNER", "project_issue", "project.id=project_issue.project_id").
			Where("project_issue.issue_id = ?", issue.ID).Find(&issue.Projects)
		if err == nil {
			issue.isProjectsLoaded = true
		}
	}
	return err
}

func (issue *Issue) projectIDs(ctx context.Context) ([]int64, error) {
	var pis []project_model.ProjectIssue
	if err := db.GetEngine(ctx).Table("project_issue").Where("issue_id = ?", issue.ID).Cols("project_id").Find(&pis); err != nil {
		return nil, err
	}

	if len(pis) == 0 {
		return []int64{}, nil
	}

	ids := make([]int64, 0, len(pis))
	for _, pi := range pis {
		ids = append(ids, pi.ProjectID)
	}
	return ids, nil
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
		o.ProjectIDs = []int64{b.ProjectID}
		o.SortType = "project-column-sorting"
	}))
	if err != nil {
		return nil, err
	}

	if b.Default {
		issues, err := Issues(ctx, opts.Copy(func(o *IssuesOptions) {
			o.ProjectColumnID = db.NoConditionID
			o.ProjectIDs = []int64{b.ProjectID}
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

// IssueAssignOrRemoveProject updates the projects associated with an issue.
// It adds projects that are in newProjectIDs but not currently assigned, and removes
// projects that are currently assigned but not in newProjectIDs. If newProjectIDs is
// empty or nil, all projects are removed from the issue.
func IssueAssignOrRemoveProject(ctx context.Context, issue *Issue, doer *user_model.User, newProjectIDs []int64, newColumnID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		oldProjectIDs, err := issue.projectIDs(ctx)
		if err != nil {
			return err
		}

		if err := issue.LoadRepo(ctx); err != nil {
			return err
		}

		newProjectIDs, oldProjectIDs := util.DiffSlice(oldProjectIDs, newProjectIDs)

		if len(oldProjectIDs) > 0 {
			if _, err := db.GetEngine(ctx).Where("issue_id=?", issue.ID).In("project_id", oldProjectIDs).Delete(&project_model.ProjectIssue{}); err != nil {
				return err
			}
			for _, projectID := range oldProjectIDs {
				if _, err := CreateComment(ctx, &CreateCommentOptions{
					Type:         CommentTypeProject,
					Doer:         doer,
					Repo:         issue.Repo,
					Issue:        issue,
					OldProjectID: projectID,
					ProjectID:    0,
				}); err != nil {
					return err
				}
			}
		}

		if len(newProjectIDs) == 0 {
			return nil
		}

		res := struct {
			MaxSorting int64
			IssueCount int64
		}{}
		if _, err := db.GetEngine(ctx).Select("max(sorting) as max_sorting, count(*) as issue_count").Table("project_issue").
			In("project_id", newProjectIDs).
			And("project_board_id=?", newColumnID).
			Get(&res); err != nil {
			return err
		}
		newSorting := util.Iif(res.IssueCount > 0, res.MaxSorting+1, 0)

		pi := make([]*project_model.ProjectIssue, 0, len(newProjectIDs))

		for _, projectID := range newProjectIDs {
			if projectID == 0 {
				continue
			}
			newProject, err := project_model.GetProjectByID(ctx, projectID)
			if err != nil {
				return err
			}
			if !newProject.CanBeAccessedByOwnerRepo(issue.Repo.OwnerID, issue.Repo) {
				return util.NewPermissionDeniedErrorf("issue %d can't be accessed by project %d", issue.ID, newProject.ID)
			}

			projectColumnID := newColumnID
			if projectColumnID == 0 {
				defaultColumn, err := newProject.MustDefaultColumn(ctx)
				if err != nil {
					return err
				}
				projectColumnID = defaultColumn.ID
			}

			pi = append(pi, &project_model.ProjectIssue{
				IssueID:         issue.ID,
				ProjectID:       projectID,
				ProjectColumnID: projectColumnID,
				Sorting:         newSorting,
			})

			if _, err := CreateComment(ctx, &CreateCommentOptions{
				Type:         CommentTypeProject,
				Doer:         doer,
				Repo:         issue.Repo,
				Issue:        issue,
				OldProjectID: 0,
				ProjectID:    projectID,
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
