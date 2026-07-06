// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13

import "gitea.dev/models/db"

func PurgeInvalidDependenciesComments(x db.EngineMigration) error {
	_, err := x.Exec("DELETE FROM comment WHERE dependent_issue_id != 0 AND dependent_issue_id NOT IN (SELECT id FROM issue)")
	return err
}
