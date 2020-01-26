// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addFsckEnabledToRepo(x *xorm.Engine) error {
	type Repository struct {
		IsFsckEnabled bool `xorm:"NOT NULL DEFAULT true"`
	}

	if err := x.Sync2(new(Repository)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
