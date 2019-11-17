// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addBranchProtectionAddEnableWhitelist(x *xorm.Engine) error {

	type ProtectedBranch struct {
		CanPush                  bool  `xorm:"NOT NULL DEFAULT false"`
		EnableWhitelist          bool  `xorm:"NOT NULL DEFAULT false"`
		EnableApprovalsWhitelist bool  `xorm:"NOT NULL DEFAULT false"`
		RequiredApprovals        int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := x.Sync2(new(ProtectedBranch)); err != nil {
		return err
	}

	if _, err := x.Exec("UPDATE `protected_branch` SET `can_push` = `enable_whitelist`"); err != nil {
		return err
	}
	_, err := x.Exec("UPDATE `protected_branch` SET `enable_approvals_whitelist` = ? WHERE `required_approvals` > ?", true, 0)
	return err
}
