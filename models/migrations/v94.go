// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "xorm.io/xorm"

func addStatusCheckColumnsForProtectedBranches(x *xorm.Engine) error {
	type ProtectedBranch struct {
		EnableStatusCheck   bool     `xorm:"NOT NULL DEFAULT false"`
		StatusCheckContexts []string `xorm:"JSON TEXT"`
	}

	if err := x.Sync2(new(ProtectedBranch)); err != nil {
		return err
	}

	_, err := x.Cols("enable_status_check", "status_check_contexts").Update(&ProtectedBranch{
		EnableStatusCheck:   false,
		StatusCheckContexts: []string{},
	})
	return err
}
