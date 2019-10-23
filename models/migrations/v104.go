// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func removeLabelUneededCols(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := dropTableColumns(sess, "label", "query_string"); err != nil {
		return err
	}
	if err := dropTableColumns(sess, "label", "is_selected"); err != nil {
		return err
	}
	return sess.Commit()
}
