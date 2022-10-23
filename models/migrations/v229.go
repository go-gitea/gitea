// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func renameWebhookOrgToOwner(x *xorm.Engine) error {
	type Webhook struct {
		ID      int64 `xorm:"pk autoincr"`
		OrgID   int64
		OwnerID int64 `xorm:"INDEX"`
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync2(new(Webhook)); err != nil {
		return err
	}
	if _, err := sess.Exec("UPDATE webhook SET owner_id = org_id"); err != nil {
		return err
	}
	if err := dropTableColumns(sess, "webhook", "org_id"); err != nil {
		return err
	}

	return sess.Commit()
}
