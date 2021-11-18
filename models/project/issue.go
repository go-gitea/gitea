// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package project

import (
	"code.gitea.io/gitea/models/db"
)

// ProjectIssue saves relation from issue to a project
type ProjectIssue struct { //revive:disable-line:exported
	ID        int64 `xorm:"pk autoincr"`
	IssueID   int64 `xorm:"INDEX"`
	ProjectID int64 `xorm:"INDEX"`

	// If 0, then it has not been added to a specific board in the project
	ProjectBoardID int64 `xorm:"INDEX"`
}

func init() {
	db.RegisterModel(new(ProjectIssue))
}

func deleteProjectIssuesByProjectID(e db.Engine, projectID int64) error {
	_, err := e.Where("project_id=?", projectID).Delete(&ProjectIssue{})
	return err
}

//  ____            _           _
// |  _ \ _ __ ___ (_) ___  ___| |_
// | |_) | '__/ _ \| |/ _ \/ __| __|
// |  __/| | | (_) | |  __/ (__| |_
// |_|   |_|  \___// |\___|\___|\__|
//               |__/

// NumIssues return counter of all issues assigned to a project
func (p *Project) NumIssues() int {
	c, err := db.GetEngine(db.DefaultContext).Table("project_issue").
		Where("project_id=?", p.ID).
		GroupBy("issue_id").
		Cols("issue_id").
		Count()
	if err != nil {
		return 0
	}
	return int(c)
}

// NumClosedIssues return counter of closed issues assigned to a project
func (p *Project) NumClosedIssues() int {
	c, err := db.GetEngine(db.DefaultContext).Table("project_issue").
		Join("INNER", "issue", "project_issue.issue_id=issue.id").
		Where("project_issue.project_id=? AND issue.is_closed=?", p.ID, true).
		Cols("issue_id").
		Count()
	if err != nil {
		return 0
	}
	return int(c)
}

// NumOpenIssues return counter of open issues assigned to a project
func (p *Project) NumOpenIssues() int {
	c, err := db.GetEngine(db.DefaultContext).Table("project_issue").
		Join("INNER", "issue", "project_issue.issue_id=issue.id").
		Where("project_issue.project_id=? AND issue.is_closed=?", p.ID, false).Count("issue.id")
	if err != nil {
		return 0
	}
	return int(c)
}

//  ____            _           _   ____                      _
// |  _ \ _ __ ___ (_) ___  ___| |_| __ )  ___   __ _ _ __ __| |
// | |_) | '__/ _ \| |/ _ \/ __| __|  _ \ / _ \ / _` | '__/ _` |
// |  __/| | | (_) | |  __/ (__| |_| |_) | (_) | (_| | | | (_| |
// |_|   |_|  \___// |\___|\___|\__|____/ \___/ \__,_|_|  \__,_|
//               |__/

func (pb *Board) removeIssues(e db.Engine) error {
	_, err := e.Exec("UPDATE `project_issue` SET project_board_id = 0 WHERE project_board_id = ? ", pb.ID)
	return err
}
