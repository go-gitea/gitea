// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"

	board_model "code.gitea.io/gitea/models/board"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
)

// LoadProject load the project the issue was assigned to
func (issue *Issue) LoadProject() (err error) {
	return issue.loadProject(db.DefaultContext)
}

func (issue *Issue) loadProject(ctx context.Context) (err error) {
	if issue.Board == nil {
		var p board_model.Board
		if _, err = db.GetEngine(ctx).Table("project").
			Join("INNER", "project_issue", "project.id=project_issue.project_id").
			Where("project_issue.issue_id = ?", issue.ID).
			Get(&p); err != nil {
			return err
		}
		issue.Board = &p
	}
	return err
}

// ProjectID return project id if issue was assigned to one
func (issue *Issue) BoardID() int64 {
	return issue.boardID(db.DefaultContext)
}

func (issue *Issue) boardID(ctx context.Context) int64 {
	var ip board_model.BoardIssue
	has, err := db.GetEngine(ctx).Where("issue_id=?", issue.ID).Get(&ip)
	if err != nil || !has {
		return 0
	}
	return ip.BoardID
}

// ProjectBoardID return project board id if issue was assigned to one
func (issue *Issue) ProjectBoardID() int64 {
	return issue.projectBoardID(db.DefaultContext)
}

func (issue *Issue) projectBoardID(ctx context.Context) int64 {
	var ip board_model.BoardIssue
	has, err := db.GetEngine(ctx).Where("issue_id=?", issue.ID).Get(&ip)
	if err != nil || !has {
		return 0
	}
	return ip.BoardColumnID
}

// LoadIssuesFromBoardColumn load issues assigned to this column
func LoadIssuesFromBoardColumn(ctx context.Context, b *board_model.Column) (IssueList, error) {
	issueList := make([]*Issue, 0, 10)

	if b.ID != 0 {
		issues, err := Issues(ctx, &IssuesOptions{
			ProjectBoardID: b.ID,
			ProjectID:      b.BoardID,
			SortType:       "project-column-sorting",
		})
		if err != nil {
			return nil, err
		}
		issueList = issues
	}

	if b.Default {
		issues, err := Issues(ctx, &IssuesOptions{
			ProjectBoardID: -1, // Issues without ProjectBoardID
			ProjectID:      b.BoardID,
			SortType:       "project-column-sorting",
		})
		if err != nil {
			return nil, err
		}
		issueList = append(issueList, issues...)
	}

	if err := IssueList(issueList).LoadComments(ctx); err != nil {
		return nil, err
	}

	return issueList, nil
}

// LoadIssuesFromBoardList load issues assigned to the boards
func LoadIssuesFromBoardList(ctx context.Context, bs board_model.ColumnList) (map[int64]IssueList, error) {
	issuesMap := make(map[int64]IssueList, len(bs))
	for i := range bs {
		il, err := LoadIssuesFromBoardColumn(ctx, bs[i])
		if err != nil {
			return nil, err
		}
		issuesMap[bs[i].ID] = il
	}
	return issuesMap, nil
}

// ChangeProjectAssign changes the project associated with an issue
func ChangeProjectAssign(issue *Issue, doer *user_model.User, newProjectID int64) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := addUpdateIssueBoard(ctx, issue, doer, newProjectID); err != nil {
		return err
	}

	return committer.Commit()
}

func addUpdateIssueBoard(ctx context.Context, issue *Issue, doer *user_model.User, newBoardID int64) error {
	oldBoardID := issue.boardID(ctx)

	// Only check if we add a new project and not remove it.
	if newBoardID > 0 {
		newProject, err := board_model.GetBoardByID(ctx, newBoardID)
		if err != nil {
			return err
		}
		if newProject.RepoID != issue.RepoID {
			return fmt.Errorf("issue's repository is not the same as project's repository")
		}
	}

	if _, err := db.GetEngine(ctx).Where("project_issue.issue_id=?", issue.ID).Delete(&board_model.BoardIssue{}); err != nil {
		return err
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}

	if oldBoardID > 0 || newBoardID > 0 {
		if _, err := CreateComment(ctx, &CreateCommentOptions{
			Type:         CommentTypeProject,
			Doer:         doer,
			Repo:         issue.Repo,
			Issue:        issue,
			OldProjectID: oldBoardID,
			ProjectID:    newBoardID,
		}); err != nil {
			return err
		}
	}

	return db.Insert(ctx, &board_model.BoardIssue{
		IssueID: issue.ID,
		BoardID: newBoardID,
	})
}

// MoveIssueAcrossProjectBoards move a card from one board to another
func MoveIssueAcrossProjectBoards(issue *Issue, board *board_model.Board) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	var pis board_model.BoardIssue
	has, err := sess.Where("issue_id=?", issue.ID).Get(&pis)
	if err != nil {
		return err
	}

	if !has {
		return fmt.Errorf("issue has to be added to a project first")
	}

	pis.BoardColumnID = board.ID
	if _, err := sess.ID(pis.ID).Cols("project_board_id").Update(&pis); err != nil {
		return err
	}

	return committer.Commit()
}
