// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"github.com/go-xorm/xorm"
)

// RepoV17 describe the added constraints
type RepoV17 struct {
	NumWatches      int `xorm:"NOT NULL DEFAULT 0"`
	NumStars        int `xorm:"NOT NULL DEFAULT 0"`
	NumForks        int `xorm:"NOT NULL DEFAULT 0"`
	NumIssues       int `xorm:"NOT NULL DEFAULT 0"`
	NumClosedIssues int `xorm:"NOT NULL DEFAULT 0"`
	NumPulls        int `xorm:"NOT NULL DEFAULT 0"`
	NumClosedPulls  int `xorm:"NOT NULL DEFAULT 0"`
}

// TableName will be invoked by XORM
func (*RepoV17) TableName() string {
	return "repository"
}

func addNotNullConstraintsToRepository(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Exec("UPDATE `repository` SET num_watches = 0 WHERE num_watches IS NULL"); err != nil {
		return err
	}

	if _, err := sess.Exec("UPDATE `repository` SET num_stars = 0 WHERE num_stars IS NULL"); err != nil {
		return err
	}

	if _, err := sess.Exec("UPDATE `repository` SET num_forks = 0 WHERE num_forks IS NULL"); err != nil {
		return err
	}

	if _, err := sess.Exec("UPDATE `repository` SET num_issues = 0 WHERE num_issues IS NULL"); err != nil {
		return err
	}

	if _, err := sess.Exec("UPDATE `repository` SET num_closed_issues = 0 WHERE num_closed_issues IS NULL"); err != nil {
		return err
	}

	if _, err := sess.Exec("UPDATE `repository` SET num_pulls = 0 WHERE num_pulls IS NULL"); err != nil {
		return err
	}

	if _, err := sess.Exec("UPDATE `repository` SET num_closed_pulls = 0 WHERE num_closed_pulls IS NULL"); err != nil {
		return err
	}

	if err := sess.Sync2(new(RepoV17)); err != nil {
		return err
	}

	return sess.Commit()
}
