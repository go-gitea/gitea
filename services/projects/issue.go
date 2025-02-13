// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	user_model "code.gitea.io/gitea/models/user"
)

// MoveIssuesOnProjectColumn moves or keeps issues in a column and sorts them inside that column
func MoveIssuesOnProjectColumn(ctx context.Context, doer *user_model.User, column *project_model.Column, sortedIssueIDs map[int64]int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
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
			return fmt.Errorf("all issues have to be added to a project first")
		}

		issues, err := issues_model.GetIssuesByIDs(ctx, issueIDs)
		if err != nil {
			return err
		}
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
	})
}

func LoadIssuesFromColumnList(ctx context.Context, project *project_model.Project, bs project_model.ColumnList, opts *issues_model.IssuesOptions) (map[int64]issues_model.IssueList, error) {
	if project.RepoID > 0 { // repo level project, just load all issues
		return issues_model.LoadIssuesFromColumnList(ctx, bs, opts)
	}

	// for org level project, we need to filter issues according to the user's access
	if opts.User == nil {
		opts.AllPublic = true
	}
	return issues_model.LoadIssuesFromColumnList(ctx, bs, opts)
}

/*
// NumIssues return counter of all issues assigned to the column
func (c *Column) NumIssues(ctx context.Context) int {
	total, err := db.GetEngine(ctx).Table("project_issue").
		Where("project_id=?", c.ProjectID).
		And("project_board_id=?", c.ID).
		GroupBy("issue_id").
		Cols("issue_id").
		Count()
	if err != nil {
		return 0
	}
	return int(total)
}

// NumIssues return counter of all issues assigned to a project
func (p *Project) NumIssues(ctx context.Context) int {
	c, err := db.GetEngine(ctx).Table("project_issue").
		Where("project_id=?", p.ID).
		GroupBy("issue_id").
		Cols("issue_id").
		Count()
	if err != nil {
		log.Error("NumIssues: %v", err)
		return 0
	}
	return int(c)
}*/

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
	bs, err := project.GetColumns(ctx)
	if err != nil {
		return err
	}
	im, err := LoadIssuesFromColumnList(ctx, project, bs, &issues_model.IssuesOptions{
		User: doer,
		Org:  org.From(project.Owner),
	})
	if err != nil {
		return err
	}
	for _, il := range im {
		project.NumIssues += int64(len(il))
		for _, issue := range il {
			if issue.IsClosed {
				project.NumClosedIssues++
			} else {
				project.NumOpenIssues++
			}
		}
	}
	return nil
}
