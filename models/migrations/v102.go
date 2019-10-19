// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func dropColumnHeadUserNameOnPullRequest(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	return dropTableColumns(sess, "pull_request", "head_user_name")
}
