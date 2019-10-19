// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/models"

	"xorm.io/xorm"
)

func addModeToDeploKeys(x *xorm.Engine) error {
	type DeployKey struct {
		Mode models.AccessMode `xorm:"NOT NULL DEFAULT 1"`
	}

	if err := x.Sync2(new(DeployKey)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
