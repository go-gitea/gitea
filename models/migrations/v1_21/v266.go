// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import "gitea.dev/models/db"

func ReduceCommitStatus(x db.EngineMigration) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Exec(`UPDATE commit_status SET state='pending' WHERE state='running'`); err != nil {
		return err
	}

	return sess.Commit()
}
