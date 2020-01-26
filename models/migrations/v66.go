// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addLoginSourceIDToPublicKeyTable(x *xorm.Engine) error {
	type PublicKey struct {
		LoginSourceID int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	if err := x.Sync2(new(PublicKey)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
