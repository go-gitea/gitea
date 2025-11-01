// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"context"
	"errors"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/services/notify"
)

// MoveIssuesOnProjectColumn moves or keeps issues in a column and sorts them inside that column
func MoveIssuesOnProjectColumn(ctx context.Context, doer *user_model.User, column *project_model.Column, sortedIssueIDs map[int64]int64) error {
	issueIDs := make([]int64, 0, len(sortedIssueIDs))
	for _, issueID := range sortedIssueIDs {
		issueIDs = append(issueIDs, issueID)
	}
	count, err := db.GetEngine(ctx).
		Where("project_id=?", column.ProjectID).
		In("issue_id", issueIDs).
		Count(new(project_model.ProjectIssue))
	if err != nil {
		return err
	}
	if int(count) != len(sortedIssueIDs) {
		return errors.New("all issues have to be added to a project first")
	}

	issues, err := issues_model.GetIssuesByIDs(ctx, issueIDs)
	if err != nil {
		return err
	}
	oldColumnIDsMap := make(map[int64]int64, len(issues))

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := issues.LoadRepositories(ctx); err != nil {
			return err
		}

		project, err := project_model.GetProjectByID(ctx, column.ProjectID)
		if err != nil {
			return err
		}

		issuesMap := make(map[int64]*issues_model.Issue, len(issues))
		for _, issue := range issues {
			issuesMap[issue.ID] = issue
		}

		for sorting, issueID := range sortedIssueIDs {
			curIssue := issuesMap[issueID]
			if curIssue == nil {
				continue
			}

			projectColumnID, err := curIssue.ProjectColumnID(ctx)
			if err != nil {
				return err
			}
			oldColumnIDsMap[issueID] = projectColumnID

			if projectColumnID != column.ID {
				// add timeline to issue
				if _, err := issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
					Type:               issues_model.CommentTypeProjectColumn,
					Doer:               doer,
					Repo:               curIssue.Repo,
					Issue:              curIssue,
					ProjectID:          column.ProjectID,
					ProjectTitle:       project.Title,
					ProjectColumnID:    column.ID,
					ProjectColumnTitle: column.Title,
				}); err != nil {
					return err
				}
			}

			_, err = db.Exec(ctx, "UPDATE `project_issue` SET project_board_id=?, sorting=? WHERE issue_id=?", column.ID, sorting, issueID)
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	for _, issue := range issues {
		notify.IssueChangeProjectColumn(ctx, doer, issue, oldColumnIDsMap[issue.ID], column.ID)
	}

	return nil
}

// LoadIssuesFromProject load issues assigned to each project column inside the given project
func LoadIssuesFromProject(ctx context.Context, project *project_model.Project, opts *issues_model.IssuesOptions) (map[int64]issues_model.IssueList, error) {
	issueList, err := issues_model.Issues(ctx, opts.Copy(func(o *issues_model.IssuesOptions) {
		o.ProjectID = project.ID
		o.SortType = "project-column-sorting"
	}))
	if err != nil {
		return nil, err
	}

	if err := issueList.LoadComments(ctx); err != nil {
		return nil, err
	}

	defaultColumn, err := project.MustDefaultColumn(ctx)
	if err != nil {
		return nil, err
	}

	issueColumnMap, err := issues_model.LoadProjectIssueColumnMap(ctx, project.ID, defaultColumn.ID)
	if err != nil {
		return nil, err
	}

	results := make(map[int64]issues_model.IssueList)
	for _, issue := range issueList {
		projectColumnID, ok := issueColumnMap[issue.ID]
		if !ok {
			continue
		}
		if _, ok := results[projectColumnID]; !ok {
			results[projectColumnID] = make(issues_model.IssueList, 0)
		}
		results[projectColumnID] = append(results[projectColumnID], issue)
	}
	return results, nil
}

// NumClosedIssues return counter of closed issues assigned to a project
func loadNumClosedIssues(ctx context.Context, p *project_model.Project) error {
	cnt, err := db.GetEngine(ctx).Table("project_issue").
		Join("INNER", "issue", "project_issue.issue_id=issue.id").
		Where("project_issue.project_id=? AND issue.is_closed=?", p.ID, true).
		Cols("issue_id").
		Count()
	if err != nil {
		return err
	}
	p.NumClosedIssues = cnt
	return nil
}

// NumOpenIssues return counter of open issues assigned to a project
func loadNumOpenIssues(ctx context.Context, p *project_model.Project) error {
	cnt, err := db.GetEngine(ctx).Table("project_issue").
		Join("INNER", "issue", "project_issue.issue_id=issue.id").
		Where("project_issue.project_id=? AND issue.is_closed=?", p.ID, false).
		Cols("issue_id").
		Count()
	if err != nil {
		return err
	}
	p.NumOpenIssues = cnt
	return nil
}

func LoadIssueNumbersForProjects(ctx context.Context, projects []*project_model.Project, doer *user_model.User) error {
	for _, project := range projects {
		if err := LoadIssueNumbersForProject(ctx, project, doer); err != nil {
			return err
		}
	}
	return nil
}

func LoadIssueNumbersForProject(ctx context.Context, project *project_model.Project, doer *user_model.User) error {
	// for repository project, just get the numbers
	if project.OwnerID == 0 {
		if err := loadNumClosedIssues(ctx, project); err != nil {
			return err
		}
		if err := loadNumOpenIssues(ctx, project); err != nil {
			return err
		}
		project.NumIssues = project.NumClosedIssues + project.NumOpenIssues
		return nil
	}

	if err := project.LoadOwner(ctx); err != nil {
		return err
	}

	// for user or org projects, we need to check access permissions
	opts := issues_model.IssuesOptions{
		ProjectID: project.ID,
		Doer:      doer,
		AllPublic: doer == nil,
		Owner:     project.Owner,
	}

	var err error
	project.NumOpenIssues, err = issues_model.CountIssues(ctx, opts.Copy(func(o *issues_model.IssuesOptions) {
		o.IsClosed = optional.Some(false)
	}))
	if err != nil {
		return err
	}

	project.NumClosedIssues, err = issues_model.CountIssues(ctx, opts.Copy(func(o *issues_model.IssuesOptions) {
		o.IsClosed = optional.Some(true)
	}))
	if err != nil {
		return err
	}

	project.NumIssues = project.NumClosedIssues + project.NumOpenIssues

	return nil
}

func MoveIssueToAnotherColumn(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, newColumn *project_model.Column) error {
	oldColumnID, err := issue.ProjectColumnID(ctx)
	if err != nil {
		return err
	}
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Exec("UPDATE `project_issue` SET project_board_id=? WHERE issue_id=?", newColumn.ID, issue.ID); err != nil {
			return err
		}

		if err := newColumn.LoadProject(ctx); err != nil {
			return err
		}

		// add timeline to issue
		if _, err := issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
			Type:               issues_model.CommentTypeProjectColumn,
			Doer:               doer,
			Repo:               issue.Repo,
			Issue:              issue,
			ProjectID:          newColumn.ProjectID,
			ProjectTitle:       newColumn.Project.Title,
			ProjectColumnID:    newColumn.ID,
			ProjectColumnTitle: newColumn.Title,
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	notify.IssueChangeProjectColumn(ctx, doer, issue, oldColumnID, newColumn.ID)
	return nil
}
