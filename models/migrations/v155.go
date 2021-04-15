// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addChangedProtectedFilesPullRequestColumn(x *xorm.Engine) error {
	type PullRequest struct {
		ChangedProtectedFiles []string `xorm:"TEXT JSON"`
	}

	if err := x.Sync2(new(PullRequest)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
