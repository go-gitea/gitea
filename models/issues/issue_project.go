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

// projectColumn holds where an issue sits within one of its projects.
// Cached on the Issue by LoadProjects and read via ProjectColumn so the
// API converter can build api.ProjectMeta without re-querying.
type projectColumn struct {
	ID    int64
	Title string
}

// ProjectColumn returns the issue's column placement in the given project
// (column ID and title). Returns zero values if no placement is cached or
// the issue is not in the project. Requires LoadProjects to have run.
func (issue *Issue) ProjectColumn(projectID int64) (id int64, title string) {
	if c, ok := issue.projectColumns[projectID]; ok {
		return c.ID, c.Title
	}
	return 0, ""
}

// LoadProjects loads all projects the issue is assigned to into
// Issue.Projects, along with column placement cached for later
// retrieval via ProjectColumn.
func (issue *Issue) LoadProjects(ctx context.Context) (err error) {
	if issue.isProjectsLoaded {
		return nil
	}

	type projectWithColumn struct {
		*project_model.Project `xorm:"extends"`
		ProjectColumnID        int64  `xorm:"'project_board_id'"`
		ColumnTitle            string `xorm:"'column_title'"`
	}

	var rows []*projectWithColumn
	err = db.GetEngine(ctx).
		Table("project").
		Select("project.*, project_issue.project_board_id, project_board.title AS column_title").
		Join("INNER", "project_issue", "project.id = project_issue.project_id").
		Join("LEFT", "project_board", "project_issue.project_board_id = project_board.id").
		Where("project_issue.issue_id = ?", issue.ID).
		OrderBy("project.id ASC").
		Find(&rows)
	if err != nil {
		return err
	}

	issue.Projects = make([]*project_model.Project, 0, len(rows))
	issue.projectColumns = make(map[int64]projectColumn, len(rows))
	for _, r := range rows {
		issue.Projects = append(issue.Projects, r.Project)
		issue.projectColumns[r.Project.ID] = projectColumn{
			ID:    r.ProjectColumnID,
			Title: r.ColumnTitle,
		}
	}
	issue.isProjectsLoaded = true
	return nil
}

func (issue *Issue) projectIDs(ctx context.Context) (projectIDs []int64, _ error) {
	err := db.GetEngine(ctx).Table("project_issue").Where("issue_id = ?", issue.ID).Cols("project_id").Find(&projectIDs)
	return projectIDs, err
}

// ProjectColumnMap returns a map of project ID to column ID for this issue.
func (issue *Issue) ProjectColumnMap(ctx context.Context) (map[int64]int64, error) {
	var projIssues []project_model.ProjectIssue
	if err := db.GetEngine(ctx).Where("issue_id=?", issue.ID).Find(&projIssues); err != nil {
		return nil, err
	}

	result := make(map[int64]int64, len(projIssues))
	for _, projIssue := range projIssues {
		result[projIssue.ProjectID] = projIssue.ProjectColumnID
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

// IssueAssignOrRemoveProject updates the projects associated with an issue.
// It adds projects that are in newProjectIDs but not currently assigned,
// and removes projects that are currently assigned but not in newProjectIDs.
// If newProjectIDs is empty, all projects are removed from the issue.
// When adding an issue to a project, it is placed in the project's default column.
func IssueAssignOrRemoveProject(ctx context.Context, issue *Issue, doer *user_model.User, newProjectIDs []int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := issue.LoadRepo(ctx); err != nil {
			return err
		}

		oldProjectIDs, err := issue.projectIDs(ctx)
		if err != nil {
			return err
		}

		projectsToAdd, projectsToRemove := util.DiffSlice(oldProjectIDs, newProjectIDs)
		issue.isProjectsLoaded = false
		issue.Projects = nil
		issue.projectColumns = nil

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
		}

		if len(projectsToAdd) > 0 {
			projectMap, err := project_model.GetProjectsMapByIDs(ctx, projectsToAdd)
			if err != nil {
				return err
			}

			for _, projectID := range projectsToAdd {
				newProject, ok := projectMap[projectID]
				if !ok {
					return util.NewNotExistErrorf("project %d not found", projectID)
				}
				if !newProject.CanBeAccessedByOwnerRepo(issue.Repo.OwnerID, issue.Repo) {
					return util.NewPermissionDeniedErrorf("issue %d can't be accessed by project %d", issue.ID, newProject.ID)
				}

				defaultColumn, err := newProject.MustDefaultColumn(ctx)
				if err != nil {
					return err
				}

				newSorting, err := project_model.GetColumnIssueNextSorting(ctx, projectID, defaultColumn.ID)
				if err != nil {
					return err
				}

				err = db.Insert(ctx, &project_model.ProjectIssue{
					IssueID:         issue.ID,
					ProjectID:       projectID,
					ProjectColumnID: defaultColumn.ID,
					Sorting:         newSorting,
				})
				if err != nil {
					return err
				}

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
		}
		return nil
	})
}
