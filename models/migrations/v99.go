// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"github.com/go-xorm/xorm"
)

func specifyWebhookSignatureType(x *xorm.Engine) error {
	var err error

	switch x.Dialect().DriverName() {
	case "mysql":
		_, err = x.Exec("ALTER TABLE `webhook` CHANGE COLUMN `signature` TO `signature_sha1`")
	case "postgres":
		_, err = x.Exec("ALTER TABLE `webhook` RENAME COLUMN `signature` TO `signature_sha1`")
	case "mssql":
		_, err = x.Exec("sp_rename 'webhook.signature', 'signature_sha1', 'COLUMN'")
	}

	if err != nil {
		return fmt.Errorf("Error renaming webhook signature column to signature_sha1: %v", err)
	}

	return nil
}
