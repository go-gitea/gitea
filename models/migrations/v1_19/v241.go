// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19

import "gitea.dev/models/db"

// AddCardTypeToProjectTable: add CardType column, setting existing rows to CardTypeTextOnly
func AddCardTypeToProjectTable(x db.EngineMigration) error {
	type Project struct {
		CardType int `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync(new(Project))
}
