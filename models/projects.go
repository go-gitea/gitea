// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// Project is a kanban board
type Project struct {
	ID          int64  `xorm:"pk autoincr"`
	Title       string `xorm:"INDEX NOT NULL"`
	Description string `xorm:"TEXT"`
	RepoID      string `xorm:"NOT NULL"`
	CreatorID   int64  `xorm:"NOT NULL"`
	IsClosed    bool   `xorm:"INDEX"`

	RenderedContent string `xorm:"-"`

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

// GetProjects returns a list of all projects that have been created in the
// repository
func GetProjects(repoID int64, page int, isClosed bool, sortType string) ([]*Project, error) {

	projects := make([]*Project, 0, setting.UI.IssuePagingNum)
	sess := x.Where("repo_id = ? AND is_closed = ?", repoID, isClosed)
	if page > 0 {
		sess = sess.Limit(setting.UI.IssuePagingNum, (page-1)*setting.UI.IssuePagingNum)
	}

	switch sortType {
	case "oldest":
		sess.Desc("created_unix")
	case "recentupdate":
		sess.Desc("updated_unix")
	case "leastupdate":
		sess.Asc("updated_unix")
	default:
		sess.Asc("created_unix")
	}

	return projects, sess.Find(&projects)
}
