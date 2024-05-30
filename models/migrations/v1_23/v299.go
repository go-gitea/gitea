// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"xorm.io/xorm"
)

func AddMilestoneType(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Exec("ALTER TABLE milestone ADD type INT NOT NULL DEFAULT 1"); err != nil {
		return err
	}

	return sess.Commit()
}

func AddNumMilestoneInUser(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Exec("ALTER TABLE user ADD num_milestones INT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := sess.Exec("ALTER TABLE user ADD num_closed_milestones INT NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	return sess.Commit()
}
