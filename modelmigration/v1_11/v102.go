// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_11

import (
	"gitea.dev/modelmigration/base"
)

func DropColumnHeadUserNameOnPullRequest(x base.EngineMigration) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := base.DropTableColumns(sess, "pull_request", "head_user_name"); err != nil {
		return err
	}
	return sess.Commit()
}
