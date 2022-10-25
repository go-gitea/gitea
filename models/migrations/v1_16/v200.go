// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_16 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func AddTableAppState(x *xorm.Engine) error {
	type AppState struct {
		ID       string `xorm:"pk varchar(200)"`
		Revision int64
		Content  string `xorm:"LONGTEXT"`
	}
	if err := x.Sync2(new(AppState)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
