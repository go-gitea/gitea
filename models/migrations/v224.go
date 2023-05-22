// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addDescriptionAndReadmeColsForPackage(x *xorm.Engine) error {
	type Package struct {
		Description string `xorm:"TEXT"`
		Readme      string `xorm:"LONGBLOB"`
	}

	return x.Sync(new(Package))
}
