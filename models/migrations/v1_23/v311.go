// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"code.gitea.io/gitea/models/migrations/base"

	"xorm.io/xorm"
)

func RemoveRepoNumWatches(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	return base.DropTableColumns(sess, "repository", "num_watches")
}
