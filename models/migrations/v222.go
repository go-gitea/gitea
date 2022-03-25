// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addEnableSafeMirrorColToMirror(x *xorm.Engine) error {
	type Mirror struct {
		EnableSafeMirror bool `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync2(new(Mirror)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
