// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

type (
	// ProjectBoardType is used to represent a project board type
	ProjectBoardType uint8

	// ProjectBoardList is a list of all project boards in a repository
	ProjectBoardList []*ProjectBoard
)

const (
	// None is a project board type that has no predefined columns
	None ProjectBoardType = iota

	// BasicKanban is a project board type that has basic predefined columns
	BasicKanban

	// BugTriage is a project board type that has predefined columns suited to hunting down bugs
	BugTriage
)

// ProjectBoard is used to represent boards on a project
type ProjectBoard struct {
	ID      int64 `xorm:"pk autoincr"`
	Title   string
	Default bool //if true it collects issues witch are not signed to a specific board jet

	ProjectID int64 `xorm:"INDEX NOT NULL"`
	RepoID    int64 `xorm:"INDEX NOT NULL"`
	CreatorID int64 `xorm:"NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

	Issues []*Issue `xorm:"-"`
}

// IsProjectBoardTypeValid checks if the project board type is valid
func IsProjectBoardTypeValid(p ProjectBoardType) bool {
	switch p {
	case None, BasicKanban, BugTriage:
		return true
	default:
		return false
	}
}

func createBoardsForProjectsType(sess *xorm.Session, project *Project) error {

	var items []string

	switch project.BoardType {

	case BugTriage:
		items = setting.Repository.ProjectBoardBugTriageType

	case BasicKanban:
		items = setting.Repository.ProjectBoardBasicKanbanType

	case None:
		fallthrough
	default:
		return nil
	}

	if len(items) == 0 {
		return nil
	}

	var boards = make([]ProjectBoard, 0, len(items))

	for _, v := range items {
		boards = append(boards, ProjectBoard{
			CreatedUnix: timeutil.TimeStampNow(),
			UpdatedUnix: timeutil.TimeStampNow(),
			CreatorID:   project.CreatorID,
			Title:       v,
			ProjectID:   project.ID,
			RepoID:      project.RepoID,
		})
	}

	_, err := sess.Insert(boards)
	return err
}

// NewProjectBoard adds a new project board to a given project
func NewProjectBoard(board *ProjectBoard) error {
	_, err := x.Insert(board)
	return err
}

// DeleteProjectBoardByID removes all issues references to the project board.
func DeleteProjectBoardByID(repoID, projectID, boardID int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := deleteProjectBoardByID(sess, repoID, projectID, boardID); err != nil {
		return err
	}

	return sess.Commit()
}

func deleteProjectBoardByID(e Engine, repoID, projectID, boardID int64) error {
	board, err := getProjectBoard(e, repoID, projectID, boardID)
	if err != nil {
		if IsErrProjectBoardNotExist(err) {
			return nil
		}

		return err
	}

	if err = board.removeIssues(e); err != nil {
		return err
	}

	if _, err := e.ID(board.ID).Delete(board); err != nil {
		return err
	}
	return nil
}

// GetProjectBoard fetches the current board of a project
func GetProjectBoard(repoID, projectID, boardID int64) (*ProjectBoard, error) {
	return getProjectBoard(x, repoID, projectID, boardID)
}

func getProjectBoard(e Engine, repoID, projectID, boardID int64) (*ProjectBoard, error) {
	board := &ProjectBoard{
		ID:        boardID,
		RepoID:    repoID,
		ProjectID: projectID,
	}

	has, err := e.Get(board)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrProjectBoardNotExist{RepoID: repoID, BoardID: boardID, ProjectID: projectID}
	}

	return board, nil
}

// UpdateProjectBoard updates the title of a project board
func UpdateProjectBoard(board *ProjectBoard) error {
	return updateProjectBoard(x, board)
}

func updateProjectBoard(e Engine, board *ProjectBoard) error {
	board.UpdatedUnix = timeutil.TimeStampNow()
	_, err := e.ID(board.ID).Cols(
		"title",
		"default",
		"updated_unix",
	).Update(board)
	return err
}

// GetProjectBoards fetches all boards related to a project
func GetProjectBoards(repoID, projectID int64) ([]*ProjectBoard, error) {

	var boards = make([]*ProjectBoard, 0)

	sess := x.Where("repo_id=? AND project_id=?", repoID, projectID)
	return boards, sess.Find(&boards)
}

// GetUnCategorizedBoard represents a board for issues not assigned to one
func GetUnCategorizedBoard(repoID, projectID int64) (*ProjectBoard, error) {
	return &ProjectBoard{
		ProjectID: projectID,
		RepoID:    repoID,
		Title:     "UnCategorized",
		Default:   true,
	}, nil
}

// LoadIssues load issues assigned to this board
func (b ProjectBoard) LoadIssues() (IssueList, error) {
	var boardID int64
	if !b.Default {
		boardID = b.ID

	} else {
		// Issues without ProjectBoardID
		boardID = -1
	}
	issues, err := Issues(&IssuesOptions{
		ProjectBoardID: boardID,
		ProjectID:      b.ProjectID,
		RepoIDs:        []int64{b.RepoID},
	})
	b.Issues = issues
	return issues, err
}

// LoadIssues load issues assigned to the boards
func (bs ProjectBoardList) LoadIssues() (IssueList, error) {
	issues := make(IssueList, 0, len(bs)*10)
	for i := range bs {
		il, err := bs[i].LoadIssues()
		if err != nil {
			return nil, err
		}
		bs[i].Issues = il
		issues = append(issues, il...)
	}
	return issues, nil
}
