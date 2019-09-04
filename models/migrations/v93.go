// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

<<<<<<< HEAD
import (
	"code.gitea.io/gitea/modules/timeutil"
	"github.com/go-xorm/xorm"
)

func addProjectsTable(x *xorm.Engine) error {

	type ProjectType uint8

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	type Project struct {
		ID              int64  `xorm:"pk autoincr"`
		Title           string `xorm:"INDEX NOT NULL"`
		Description     string `xorm:"TEXT"`
		RepoID          int64  `xorm:"NOT NULL"`
		CreatorID       int64  `xorm:"NOT NULL"`
		IsClosed        bool   `xorm:"INDEX"`
		NumIssues       int
		NumClosedIssues int

		Type ProjectType

		ClosedDateUnix timeutil.TimeStamp
		CreatedUnix    timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	type Issue struct {
		ProjectID int64 `xorm:"INDEX"`
	}

	type ProjectBoard struct {
		ID        int64 `xorm:"pk autoincr"`
		ProjectID int64 `xorm:"INDEX NOT NULL"`
		Title     string

		// Not really needed but helpful
		CreatorID int64 `xorm:"NOT NULL"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	if err := x.Sync(new(Project)); err != nil {
		return err
	}

	if err := x.Sync(new(ProjectBoard)); err != nil {
		return err
	}

	if err := x.Sync2(new(Issue)); err != nil {
		return err
	}

	return sess.Commit()
=======
import "github.com/go-xorm/xorm"

func addEmailNotificationEnabledToUser(x *xorm.Engine) error {
	// User see models/user.go
	type User struct {
		EmailNotificationsPreference string `xorm:"VARCHAR(20) NOT NULL DEFAULT 'enabled'"`
	}

	return x.Sync2(new(User))
>>>>>>> origin
}
