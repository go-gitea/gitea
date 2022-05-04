// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	user_model "code.gitea.io/gitea/models/user"
)

// LoadProject load the project the issue was assigned to
func (i *Issue) LoadProject() (err error) {
	return i.loadProject(db.GetEngine(db.DefaultContext))
}

func (i *Issue) loadProject(e db.Engine) (err error) {
	if i.Project == nil {
		var p project_model.Project
		if _, err = e.Table("project").
			Join("INNER", "project_issue", "project.id=project_issue.project_id").
			Where("project_issue.issue_id = ?", i.ID).
			Get(&p); err != nil {
			return err
		}
		i.Project = &p
	}
	return
}

// ProjectID return project id if issue was assigned to one
func (i *Issue) ProjectID() int64 {
	return i.projectID(db.GetEngine(db.DefaultContext))
}

func (i *Issue) projectID(e db.Engine) int64 {
	var ip project_model.ProjectIssue
	has, err := e.Where("issue_id=?", i.ID).Get(&ip)
	if err != nil || !has {
		return 0
	}
	return ip.ProjectID
}

// ProjectBoardID return project board id if issue was assigned to one
func (i *Issue) ProjectBoardID() int64 {
	return i.projectBoardID(db.GetEngine(db.DefaultContext))
}

func (i *Issue) projectBoardID(e db.Engine) int64 {
	var ip project_model.ProjectIssue
	has, err := e.Where("issue_id=?", i.ID).Get(&ip)
	if err != nil || !has {
		return 0
	}
	return ip.ProjectBoardID
}

// LoadIssuesFromBoard load issues assigned to this board
func LoadIssuesFromBoard(b *project_model.Board) (IssueList, error) {
	issueList := make([]*Issue, 0, 10)

	if b.ID != 0 {
		issues, err := Issues(&IssuesOptions{
			ProjectBoardID: b.ID,
			ProjectID:      b.ProjectID,
		})
		if err != nil {
			return nil, err
		}
		issueList = issues
	}

	if b.Default {
		issues, err := Issues(&IssuesOptions{
			ProjectBoardID: -1, // Issues without ProjectBoardID
			ProjectID:      b.ProjectID,
		})
		if err != nil {
			return nil, err
		}
		issueList = append(issueList, issues...)
	}

	if err := IssueList(issueList).LoadComments(); err != nil {
		return nil, err
	}

	return issueList, nil
}

// LoadIssuesFromBoardList load issues assigned to the boards
func LoadIssuesFromBoardList(bs project_model.BoardList) (map[int64]IssueList, error) {
	issuesMap := make(map[int64]IssueList, len(bs))
	for i := range bs {
		il, err := LoadIssuesFromBoard(bs[i])
		if err != nil {
			return nil, err
		}
		issuesMap[bs[i].ID] = il
	}
	return issuesMap, nil
}

// ChangeProjectAssign changes the project associated with an issue
func ChangeProjectAssign(issue *Issue, doer *user_model.User, newProjectID int64) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := addUpdateIssueProject(ctx, issue, doer, newProjectID); err != nil {
		return err
	}

	return committer.Commit()
}

func addUpdateIssueProject(ctx context.Context, issue *Issue, doer *user_model.User, newProjectID int64) error {
	e := db.GetEngine(ctx)
	oldProjectID := issue.projectID(e)

	if _, err := e.Where("project_issue.issue_id=?", issue.ID).Delete(&project_model.ProjectIssue{}); err != nil {
		return err
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}

	if oldProjectID > 0 || newProjectID > 0 {
		if _, err := CreateCommentCtx(ctx, &CreateCommentOptions{
			Type:         CommentTypeProject,
			Doer:         doer,
			Repo:         issue.Repo,
			Issue:        issue,
			OldProjectID: oldProjectID,
			ProjectID:    newProjectID,
		}); err != nil {
			return err
		}
	}

	_, err := e.Insert(&project_model.ProjectIssue{
		IssueID:   issue.ID,
		ProjectID: newProjectID,
	})
	return err
}

// MoveIssueAcrossProjectBoards move a card from one board to another
func MoveIssueAcrossProjectBoards(issue *Issue, board *project_model.Board) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	var pis project_model.ProjectIssue
	has, err := sess.Where("issue_id=?", issue.ID).Get(&pis)
	if err != nil {
		return err
	}

	if !has {
		return fmt.Errorf("issue has to be added to a project first")
	}

	pis.ProjectBoardID = board.ID
	if _, err := sess.ID(pis.ID).Cols("project_board_id").Update(&pis); err != nil {
		return err
	}

	return committer.Commit()
}
