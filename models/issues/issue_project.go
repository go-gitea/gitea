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

// LoadedProject pairs a project the issue belongs to with the issue's
// column placement within that project. Populated by LoadProjects;
// consumed by the API converter to build api.ProjectMeta.
type LoadedProject struct {
	Project     *project_model.Project
	ColumnID    int64
	ColumnTitle string
}

// LoadProjects loads all projects the issue is assigned to,
// along with the issue's column placement in each (Issue.LoadedProjects).
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
	issue.LoadedProjects = make([]*LoadedProject, 0, len(rows))
	for _, r := range rows {
		issue.Projects = append(issue.Projects, r.Project)
		issue.LoadedProjects = append(issue.LoadedProjects, &LoadedProject{
			Project:     r.Project,
			ColumnID:    r.ProjectColumnID,
			ColumnTitle: r.ColumnTitle,
		})
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
