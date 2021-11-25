// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addPrivateIssues(x *xorm.Engine) error {
	type Repository struct {
		NumPrivateIssues       int
		NumClosedPrivateIssues int
	}

	if err := x.Sync2(new(Repository)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	type Issue struct {
		IsPrivate bool `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync2(new(Issue)); err != nil {
		return err
	}

	type Team struct {
		ID                  int64 `xorm:"pk autoincr"`
		CanSeePrivateIssues bool  `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync2(new(Team)); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE `team` SET `can_see_private_issues` = ? WHERE `name`=?",
		true, "Owners")
	return err
}
