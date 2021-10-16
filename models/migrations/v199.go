// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addRemoteVersionTable(x *xorm.Engine) error {
	type RemoteVersion struct {
		ID      int64  `xorm:"pk autoincr"`
		Version string `xorm:"VARCHAR(50)"`
	}

	if err := x.Sync2(new(RemoteVersion)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
