// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"

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
			Where("project_issue.issue_id = ?", issue.ID).
			OrderBy("project.id ASC").
			Find(&issue.Projects)
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

// ProjectColumnMap returns a map of project ID to column ID for this issue.
// This properly handles issues assigned to multiple projects.
func (issue *Issue) ProjectColumnMap(ctx context.Context) (map[int64]int64, error) {
	var pis []project_model.ProjectIssue
	if err := db.GetEngine(ctx).Where("issue_id=?", issue.ID).Find(&pis); err != nil {
		return nil, err
	}

	result := make(map[int64]int64, len(pis))
	for _, pi := range pis {
		result[pi.ProjectID] = pi.ProjectColumnID
	}
	return result, nil
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
	issueList, err := loadIssuesForProjectColumn(ctx, b.ProjectID, b.ID, opts)
	if err != nil {
		return nil, err
	}

	if b.Default {
		issues, err := loadIssuesForProjectColumn(ctx, b.ProjectID, 0, opts)
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

func loadIssuesForProjectColumn(ctx context.Context, projectID, columnID int64, opts *IssuesOptions) (IssueList, error) {
	columnOpts := opts.Copy(func(o *IssuesOptions) {
		o.ProjectIDs = []int64{projectID}
		o.SortType = "project-column-sorting"
	})
	if columnOpts == nil {
		columnOpts = &IssuesOptions{ProjectIDs: []int64{projectID}, SortType: "project-column-sorting"}
	}

	sess := db.GetEngine(ctx).
		Join("INNER", "repository", "`issue`.repo_id = `repository`.id")
	applyLimit(sess, columnOpts)
	applyConditions(sess, columnOpts)
	sess.And("project_issue.project_board_id = ?", columnID)
	applySorts(sess, columnOpts.SortType, columnOpts.PriorityRepoID)

	issueList := IssueList{}
	if err := sess.Find(&issueList); err != nil {
		return nil, err
	}
	if err := issueList.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	return issueList, nil
}

// IssueAssignOrRemoveProject updates the projects associated with an issue.
// It adds projects that are in newProjectIDs but not currently assigned, and removes
// projects that are currently assigned but not in newProjectIDs. If newProjectIDs is
// empty or nil, all projects are removed from the issue.
// When adding an issue to a project, it is placed in the project's default column.
func IssueAssignOrRemoveProject(ctx context.Context, issue *Issue, doer *user_model.User, newProjectIDs []int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		oldProjectIDs, err := issue.projectIDs(ctx)
		if err != nil {
			return err
		}

		if err := issue.LoadRepo(ctx); err != nil {
			return err
		}

		projectsToAdd, projectsToRemove := util.DiffSlice(oldProjectIDs, newProjectIDs)

		if len(projectsToRemove) > 0 {
			if _, err := db.GetEngine(ctx).Where("issue_id=?", issue.ID).In("project_id", projectsToRemove).Delete(&project_model.ProjectIssue{}); err != nil {
				return err
			}
			for _, projectID := range projectsToRemove {
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
			// Reset cached state so subsequent LoadProjects calls fetch fresh data
			issue.isProjectsLoaded = false
			issue.Projects = nil
		}

		if len(projectsToAdd) == 0 {
			return nil
		}

		// Batch load all projects to reduce queries (1 query instead of N)
		projectMap, err := project_model.GetProjectsMapByIDs(ctx, projectsToAdd)
		if err != nil {
			return err
		}

		// Batch load all default columns (1-2 queries instead of N)
		defaultColumns, err := project_model.GetDefaultColumnsByProjectIDs(ctx, projectsToAdd)
		if err != nil {
			return err
		}

		pi := make([]*project_model.ProjectIssue, 0, len(projectsToAdd))

		for _, projectID := range projectsToAdd {
			if projectID == 0 {
				continue
			}

			newProject, ok := projectMap[projectID]
			if !ok {
				return fmt.Errorf("project %d not found", projectID)
			}
			if !newProject.CanBeAccessedByOwnerRepo(issue.Repo.OwnerID, issue.Repo) {
				return util.NewPermissionDeniedErrorf("issue %d can't be accessed by project %d", issue.ID, newProject.ID)
			}

			// Get the default column for this project (from batch-loaded map)
			defaultColumn, ok := defaultColumns[projectID]
			if !ok {
				// Fallback: if batch loading didn't find it, call the individual method
				defaultColumn, err = newProject.MustDefaultColumn(ctx)
				if err != nil {
					return err
				}
			}
			projectColumnID := defaultColumn.ID

			// Calculate sorting position (this query must be per-project for correctness)
			newSorting, err := project_model.GetColumnIssueNextSorting(ctx, projectID, projectColumnID)
			if err != nil {
				return err
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
			if err := db.Insert(ctx, pi); err != nil {
				return err
			}
			// Reset cached state so subsequent LoadProjects calls fetch fresh data
			issue.isProjectsLoaded = false
			issue.Projects = nil
		}

		return nil
	})
}
