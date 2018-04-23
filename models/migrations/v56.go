// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"github.com/go-xorm/xorm"
)

func removeIsOwnerColumnFromOrgUser(x *xorm.Engine) (err error) {
	sess := x.NewSession()
	defer sess.Close()

	err = sess.Begin()
	if err != nil {
		return err
	}

	return dropTableColumns(sess, "org_user", "is_owner", "num_teams")
}
