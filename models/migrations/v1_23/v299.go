// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import "xorm.io/xorm"

func AddVersionToIssue(x *xorm.Engine) error {
	type Issue struct {
		Version int `xorm:"version"`
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync(new(Issue)); err != nil {
		return err
	}

	return sess.Commit()
}
