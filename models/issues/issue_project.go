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

// LoadBoard load the board the issue was assigned to
func (issue *Issue) LoadBoard() (err error) {
	return issue.loadBoard(db.DefaultContext)
}

func (issue *Issue) loadBoard(ctx context.Context) (err error) {
	if issue.Board == nil {
		var p board_model.Board
		if _, err = db.GetEngine(ctx).Table("board").
			Join("INNER", "board_issue", "board.id=board_issue.board_id").
			Where("board_issue.issue_id = ?", issue.ID).
			Get(&p); err != nil {
			return err
		}
		issue.Board = &p
	}
	return err
}

// BoardID return board id if issue was assigned to one
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

// BoardColumnID return board column id if issue was assigned to one
func (issue *Issue) BoardColumnID() int64 {
	return issue.boardColumnID(db.DefaultContext)
}

func (issue *Issue) boardColumnID(ctx context.Context) int64 {
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
			BoardColumnID: b.ID,
			BoardID:       b.BoardID,
			SortType:      "board-column-sorting",
		})
		if err != nil {
			return nil, err
		}
		issueList = issues
	}

	if b.Default {
		issues, err := Issues(ctx, &IssuesOptions{
			BoardColumnID: -1, // Issues without BoardColumnID
			BoardID:       b.BoardID,
			SortType:      "board-column-sorting",
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

// ChangeBoardAssign changes the board associated with an issue
func ChangeBoardAssign(issue *Issue, doer *user_model.User, newBoardID int64) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := addUpdateIssueBoard(ctx, issue, doer, newBoardID); err != nil {
		return err
	}

	return committer.Commit()
}

func addUpdateIssueBoard(ctx context.Context, issue *Issue, doer *user_model.User, newBoardID int64) error {
	oldBoardID := issue.boardID(ctx)

	// Only check if we add a new board and not remove it.
	if newBoardID > 0 {
		newBoard, err := board_model.GetBoardByID(ctx, newBoardID)
		if err != nil {
			return err
		}
		if newBoard.RepoID != issue.RepoID {
			return fmt.Errorf("issue's repository is not the same as board's repository")
		}
	}

	if _, err := db.GetEngine(ctx).Where("board_issue.issue_id=?", issue.ID).Delete(&board_model.BoardIssue{}); err != nil {
		return err
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}

	if oldBoardID > 0 || newBoardID > 0 {
		if _, err := CreateComment(ctx, &CreateCommentOptions{
			Type:       CommentTypeBoard,
			Doer:       doer,
			Repo:       issue.Repo,
			Issue:      issue,
			OldBoardID: oldBoardID,
			BoardID:    newBoardID,
		}); err != nil {
			return err
		}
	}

	return db.Insert(ctx, &board_model.BoardIssue{
		IssueID: issue.ID,
		BoardID: newBoardID,
	})
}

// MoveIssueAcrossBoardColumns move a card from one column to another
func MoveIssueAcrossBoardColumns(issue *Issue, board *board_model.Board) error {
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
		return fmt.Errorf("issue has to be added to a board first")
	}

	pis.BoardColumnID = board.ID
	if _, err := sess.ID(pis.ID).Cols("board_column_id").Update(&pis); err != nil {
		return err
	}

	return committer.Commit()
}
