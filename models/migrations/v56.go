// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func removeIsOwnerColumnFromOrgUser(x *xorm.Engine) (err error) {
	sess := x.NewSession()
	defer sess.Close()

	if err = sess.Begin(); err != nil {
		return err
	}

	if err := dropTableColumns(sess, "org_user", "is_owner", "num_teams"); err != nil {
		return err
	}
	return sess.Commit()
}
