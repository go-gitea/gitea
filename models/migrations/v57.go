// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addIssueClosedTime(x *xorm.Engine) error {
	// Issue see models/issue.go
	type Issue struct {
		ClosedUnix timeutil.TimeStamp `xorm:"INDEX"`
	}

	if err := x.Sync2(new(Issue)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	if _, err := x.Exec("UPDATE `issue` SET `closed_unix` = `updated_unix` WHERE `is_closed` = ?", true); err != nil {
		return err
	}

	return nil
}
