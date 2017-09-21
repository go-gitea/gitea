// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models"

	"github.com/go-xorm/xorm"
)

func fixProtectedBranchCanPushValue(x *xorm.Engine) error {
	_, err := x.Cols("can_push").Update(&models.ProtectedBranch{
		CanPush: false,
	})
	return err
}
