// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addGPGKeyImport(x *xorm.Engine) error {
	type GPGKeyImport struct {
		KeyID   string `xorm:"pk CHAR(16) NOT NULL"`
		Content string `xorm:"TEXT NOT NULL"`
	}

	return x.Sync2(new(GPGKeyImport))
}
