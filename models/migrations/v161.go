// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/log"

	"xorm.io/xorm"
)

func dropEmailHashTable(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Exec("DROP TABLE email_hash"); err != nil {
		log.Error("Unable to drop table email_hash. Error: %v", err)
		return err
	}

	return sess.Commit()
}
