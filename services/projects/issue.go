// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"slices"
	"strings"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"
	"gitea.dev/services/notify"

	"xorm.io/builder"
)

// ErrIssueNotInProject unwraps as ErrUnprocessableContent, not ErrNotExist: ctx.ServerError
// diverts ErrNotExist to a 404, which would hide this from the web caller's logs.
var ErrIssueNotInProject = util.ErrorWrap(util.ErrUnprocessableContent, "all issues have to be added to a project first")

// AddIssueToColumn assigns the issue to the column's project if needed, then places it in
// the column. One transaction, so a failure cannot strand it in the default column.
func AddIssueToColumn(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, column *project_model.Column) error {
	var added []int64
	moves, err := db.WithTx2(ctx, func(ctx context.Context) (moves []columnMove, err error) {
		projectIDs, err := issue.ProjectIDs(ctx)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(projectIDs, column.ProjectID) {
			// lands in the default column, the move below puts it in the requested one
			if added, _, err = issues_model.IssueAssignOrRemoveProject(ctx, issue, doer, append(projectIDs, column.ProjectID)); err != nil {
				return nil, err
			}
		}
		return moveIssueToColumn(ctx, doer, issue, column, optional.None[int64]())
	})
	if err != nil {
		return err
	}
	notify.IssueChangeProjects(ctx, doer, issue, added, nil)
	notifyColumnMoves(ctx, doer, column, moves)
	return nil
}

// MoveIssueToColumn places an issue already in the project into a column, appending it
// when sorting is absent.
func MoveIssueToColumn(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, column *project_model.Column, sorting optional.Option[int64]) error {
	moves, err := db.WithTx2(ctx, func(ctx context.Context) ([]columnMove, error) {
		return moveIssueToColumn(ctx, doer, issue, column, sorting)
	})
	if err != nil {
		return err
	}
	notifyColumnMoves(ctx, doer, column, moves)
	return nil
}

func moveIssueToColumn(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, column *project_model.Column, sorting optional.Option[int64]) ([]columnMove, error) {
	position := sorting.Value()
	if !sorting.Has() {
		next, err := project_model.GetColumnIssueNextSorting(ctx, column)
		if err != nil {
			return nil, err
		}
		position = next
	}
	return moveIssuesOnProjectColumn(ctx, doer, column, map[int64]int64{position: issue.ID})
}

// RemoveIssueFromColumn detaches the issue from the column's project, reporting a
// not-exist error when it is not in that column.
func RemoveIssueFromColumn(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, column *project_model.Column) error {
	removed, err := db.WithTx2(ctx, func(ctx context.Context) ([]int64, error) {
		exists, err := project_model.IsIssueInColumn(ctx, issue.ID, column)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, util.NewNotExistErrorf("issue %d is not in column %d", issue.ID, column.ID)
		}
		projectIDs, err := issue.ProjectIDs(ctx)
		if err != nil {
			return nil, err
		}
		_, removed, err := issues_model.IssueAssignOrRemoveProject(ctx, issue, doer, util.SliceRemoveAll(projectIDs, column.ProjectID))
		return removed, err
	})
	if err != nil {
		return err
	}
	notify.IssueChangeProjects(ctx, doer, issue, nil, removed)
	return nil
}

// MoveIssuesOnProjectColumn moves or keeps issues in a column and sorts them inside that column
func MoveIssuesOnProjectColumn(ctx context.Context, doer *user_model.User, column *project_model.Column, sortedIssueIDs map[int64]int64) error {
	moves, err := db.WithTx2(ctx, func(ctx context.Context) ([]columnMove, error) {
		return moveIssuesOnProjectColumn(ctx, doer, column, sortedIssueIDs)
	})
	if err != nil {
		return err
	}
	notifyColumnMoves(ctx, doer, column, moves)
	return nil
}

// columnMove is an issue that changed column, notified once the transaction has committed
type columnMove struct {
	issue       *issues_model.Issue
	oldColumnID int64
}

func notifyColumnMoves(ctx context.Context, doer *user_model.User, column *project_model.Column, moves []columnMove) {
	for _, move := range moves {
		notify.IssueChangeProjectColumn(ctx, doer, move.issue, move.oldColumnID, column)
	}
}

func moveIssuesOnProjectColumn(ctx context.Context, doer *user_model.User, column *project_model.Column, sortedIssueIDs map[int64]int64) ([]columnMove, error) {
	issueIDs := make([]int64, 0, len(sortedIssueIDs))
	for _, issueID := range sortedIssueIDs {
		issueIDs = append(issueIDs, issueID)
	}
	count, err := db.GetEngine(ctx).
		Where("project_id=?", column.ProjectID).
		In("issue_id", issueIDs).
		Count(new(project_model.ProjectIssue))
	if err != nil {
		return nil, err
	}
	if int(count) != len(sortedIssueIDs) {
		return nil, ErrIssueNotInProject
	}

	issues, err := issues_model.GetIssuesByIDs(ctx, issueIDs)
	if err != nil {
		return nil, err
	}
	if _, err := issues.LoadRepositories(ctx); err != nil {
		return nil, err
	}

	project, err := project_model.GetProjectByID(ctx, column.ProjectID)
	if err != nil {
		return nil, err
	}

	issuesMap := make(map[int64]*issues_model.Issue, len(issues))
	for _, issue := range issues {
		issuesMap[issue.ID] = issue
	}

	var moves []columnMove
	var defaultColumn *project_model.Column
	for sorting, issueID := range sortedIssueIDs {
		curIssue := issuesMap[issueID]
		if curIssue == nil {
			continue
		}

		projectColumnMap, err := curIssue.ProjectColumnMap(ctx)
		if err != nil {
			return nil, err
		}

		projectColumnID := projectColumnMap[column.ProjectID]
		oldColumnID := projectColumnID
		if oldColumnID == 0 { // legacy encoding of the default column
			if defaultColumn == nil {
				if defaultColumn, err = project.MustDefaultColumn(ctx); err != nil {
					return nil, err
				}
			}
			oldColumnID = defaultColumn.ID
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
				return nil, err
			}
		}
		if oldColumnID != column.ID {
			moves = append(moves, columnMove{issue: curIssue, oldColumnID: oldColumnID})
		}

		// Update the column and sorting for this specific issue in this specific project.
		// IMPORTANT: The WHERE clause must include both issue_id AND project_id to ensure
		// that moving an issue's column in one project doesn't affect its column in other
		// projects when the issue is assigned to multiple projects.
		_, err = db.Exec(ctx, "UPDATE `project_issue` SET project_board_id=?, sorting=? WHERE issue_id=? AND project_id=?",
			column.ID, sorting, issueID, column.ProjectID)
		if err != nil {
			return nil, err
		}
	}
	return moves, nil
}

func LoadIssuesAssigneesForProject(ctx context.Context, projectID int64) (users []*user_model.User, _ error) {
	sub := builder.Select("distinct issue_assignees.assignee_id").
		From("project_issue").Join("INNER", "issue_assignees", "project_issue.issue_id=issue_assignees.issue_id").
		Where(builder.Eq{"project_issue.project_id": projectID})
	err := db.GetEngine(ctx).Table("`user`").Where(builder.In("id", sub)).Find(&users)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(users, func(a, b *user_model.User) int { return strings.Compare(a.Name, b.Name) })
	return users, nil
}

// LoadIssuesFromProject load issues assigned to each project column inside the given project
func LoadIssuesFromProject(ctx context.Context, project *project_model.Project, opts *issues_model.IssuesOptions) (results map[int64]issues_model.IssueList, _ error) {
	issueList, err := issues_model.Issues(ctx, opts.Copy(func(o *issues_model.IssuesOptions) {
		o.ProjectIDs = []int64{project.ID}
		o.SortType = "project-column-sorting"
	}))
	if err != nil {
		return nil, err
	}
	if len(issueList) == 0 {
		// if no issue, return directly, then no need to create a default column for an empty project
		return results, nil
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

	results = make(map[int64]issues_model.IssueList)
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
		ProjectIDs: []int64{project.ID},
		Doer:       doer,
		AllPublic:  doer == nil,
		Owner:      project.Owner,
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
