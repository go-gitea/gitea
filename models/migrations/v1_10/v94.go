// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10 //nolint

import "xorm.io/xorm"

func AddStatusCheckColumnsForProtectedBranches(x *xorm.Engine) error {
	type ProtectedBranch struct {
		EnableStatusCheck   bool     `xorm:"NOT NULL DEFAULT false"`
		StatusCheckContexts []string `xorm:"JSON TEXT"`
	}

	if err := x.Sync(new(ProtectedBranch)); err != nil {
		return err
	}

	_, err := x.Cols("enable_status_check", "status_check_contexts").Update(&ProtectedBranch{
		EnableStatusCheck:   false,
		StatusCheckContexts: []string{},
	})
	return err
}
