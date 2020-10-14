// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addPrimaryKeyToRepoTopic(x *xorm.Engine) error {
	// Topic represents a topic of repositories
	type Topic struct {
		ID          int64  `xorm:"pk autoincr"`
		Name        string `xorm:"UNIQUE VARCHAR(25)"`
		RepoCount   int
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	// RepoTopic represents associated repositories and topics
	type RepoTopic struct {
		RepoID  int64 `xorm:"pk"`
		TopicID int64 `xorm:"pk"`
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	recreateTable(sess, &Topic{})
	recreateTable(sess, &RepoTopic{})

	return sess.Commit()
}
