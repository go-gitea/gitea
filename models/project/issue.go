// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
)

// ProjectIssue saves relation from issue to a project
type ProjectIssue struct { //revive:disable-line:exported
	ID        int64 `xorm:"pk autoincr"`
	IssueID   int64 `xorm:"INDEX"`
	ProjectID int64 `xorm:"INDEX"`

	ProjectBoardID int64  `xorm:"INDEX"`
	ProjectBoard   *Board `xorm:"-"`

	// the sorting order on the board
	Sorting int64 `xorm:"NOT NULL DEFAULT 0"`
}

func init() {
	db.RegisterModel(new(ProjectIssue))
}

func deleteProjectIssuesByProjectID(ctx context.Context, projectID int64) error {
	_, err := db.GetEngine(ctx).Where("project_id=?", projectID).Delete(&ProjectIssue{})
	return err
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
}

// NumClosedIssues return counter of closed issues assigned to a project
func (p *Project) NumClosedIssues(ctx context.Context) int {
	c, err := db.GetEngine(ctx).Table("project_issue").
		Join("INNER", "issue", "project_issue.issue_id=issue.id").
		Where("project_issue.project_id=? AND issue.is_closed=?", p.ID, true).
		Cols("issue_id").
		Count()
	if err != nil {
		log.Error("NumClosedIssues: %v", err)
		return 0
	}
	return int(c)
}

// NumOpenIssues return counter of open issues assigned to a project
func (p *Project) NumOpenIssues(ctx context.Context) int {
	c, err := db.GetEngine(ctx).Table("project_issue").
		Join("INNER", "issue", "project_issue.issue_id=issue.id").
		Where("project_issue.project_id=? AND issue.is_closed=?", p.ID, false).
		Cols("issue_id").
		Count()
	if err != nil {
		log.Error("NumOpenIssues: %v", err)
		return 0
	}
	return int(c)
}

// MoveIssuesOnProjectBoard moves or keeps issues in a column and sorts them inside that column
func MoveIssuesOnProjectBoard(ctx context.Context, board *Board, issueIDs, issueSorts []int64) error {
	sess := db.GetEngine(ctx)

	count, err := sess.Table(new(ProjectIssue)).Where("project_id=?", board.ProjectID).In("issue_id", issueIDs).Count()
	if err != nil {
		return err
	}
	if int(count) != len(issueIDs) {
		return fmt.Errorf("all issues have to be added to a project first")
	}

	for i, issueID := range issueIDs {
		_, err = sess.Exec("UPDATE `project_issue` SET project_board_id=?, sorting=? WHERE issue_id=?", board.ID, issueSorts[i], issueID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Board) removeIssues(ctx context.Context) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `project_issue` SET project_board_id = 0 WHERE project_board_id = ? ", b.ID)
	return err
}

type ErrProjectIssueNotExist struct {
	IssueID int64
}

func (e ErrProjectIssueNotExist) Error() string {
	return fmt.Sprintf("can't find project issue [issue_id: %d]", e.IssueID)
}

func IsErrProjectIssueNotExist(e error) bool {
	_, ok := e.(ErrProjectIssueNotExist)
	return ok
}

func GetProjectIssueByIssueID(ctx context.Context, issueID int64) (*ProjectIssue, error) {
	issue := &ProjectIssue{}

	has, err := db.GetEngine(ctx).Where("issue_id = ?", issueID).Get(issue)
	if err != nil {
		return nil, err
	}

	if !has {
		return nil, ErrProjectIssueNotExist{IssueID: issueID}
	}

	return issue, nil
}

func (issue *ProjectIssue) LoadProjectBoard(ctx context.Context) error {
	if issue.ProjectBoard != nil {
		return nil
	}

	var err error

	issue.ProjectBoard, err = GetBoard(ctx, issue.ProjectBoardID)
	if IsErrProjectBoardNotExist(err) {
		issue.ProjectBoard = &Board{
			ID:    -1,
			Title: "Deleted",
		}

		return nil
	}

	return err
}
