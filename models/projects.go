// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import "code.gitea.io/gitea/modules/util"

// Project is a kanban board
type Project struct {
	ID          int64  `xorm:"pk autoincr"`
	Title       string `xorm:"INDEX NOT NULL"`
	Description string `xorm:"NOT NULL"`
	RepoID      string `xorm:"NOT NULL"`
	CreatorID   int64  `xorm:"NOT NULL"`

	CreatedUnix util.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix util.TimeStamp `xorm:"INDEX updated"`
}

// CreateProject adds a new project entry to the database
func CreateProject(p *Project, creator *User) error {

	return nil
}

// ProjectExists checks if a given project exists
func ProjectExists(p *Project) bool {
	exists, _ := x.Exist(p)
	return exists
}

func GetProjects(repoID int64) ([]Project, error) {

	var projects []Project

	err := x.Where("repo_id", repoID).Find(&projects)
	return projects, err
}
