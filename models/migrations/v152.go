// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addSubTeamSupport(x *xorm.Engine) error {
	type Team struct {
		FullName     string `xorm:"NOT NULL DEFAULT ''"`
		ParentTeamID int64  `xorm:"NOT NULL DEFAULT -1"`
		NumSubTeams  int    `xorm:"NOT NULL DEFAULT 0"`
	}

	if err := x.Sync2(new(Team)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	if _, err := x.Exec("UPDATE team SET full_name = name"); err != nil {
		return fmt.Errorf("Copy name to full_name : %v", err)
	}

	type TeamRepo struct {
		Inherited bool `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync2(new(Team)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	return nil
}
