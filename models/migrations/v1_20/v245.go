// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func RenameWebhookOrgToOwner(x *xorm.Engine) error {
	type Webhook struct {
		OrgID int64 `xorm:"INDEX"`
	}

	// This migration maybe rerun so that we should check if it has been run
	ownerExist, err := x.Dialect().IsColumnExist(x.DB(), context.Background(), "webhook", "owner_id")
	if err != nil {
		return err
	}

	if ownerExist {
		orgExist, err := x.Dialect().IsColumnExist(x.DB(), context.Background(), "webhook", "org_id")
		if err != nil {
			return err
		}
		if !orgExist {
			return nil
		}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync(new(Webhook)); err != nil {
		return err
	}

	if ownerExist {
		if err := base.DropTableColumns(sess, "webhook", "owner_id"); err != nil {
			return err
		}
	}

	switch {
	case setting.Database.Type.IsMySQL():
		inferredTable, err := x.TableInfo(new(Webhook))
		if err != nil {
			return err
		}
		sqlType := x.Dialect().SQLType(inferredTable.GetColumn("org_id"))
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `webhook` CHANGE org_id owner_id %s", sqlType)); err != nil {
			return err
		}
	case setting.Database.Type.IsMSSQL():
		if _, err := sess.Exec("sp_rename 'webhook.org_id', 'owner_id', 'COLUMN'"); err != nil {
			return err
		}
	default:
		if _, err := sess.Exec("ALTER TABLE `webhook` RENAME COLUMN org_id TO owner_id"); err != nil {
			return err
		}
	}

	return sess.Commit()
}
