// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"time"

	"xorm.io/core"
	"xorm.io/xorm"
)

func addLoginSourceSyncEnabledColumn(x *xorm.Engine) error {
	// LoginSource see models/login_source.go
	type LoginSource struct {
		ID            int64 `xorm:"pk autoincr"`
		Type          int
		Name          string          `xorm:"UNIQUE"`
		IsActived     bool            `xorm:"INDEX NOT NULL DEFAULT false"`
		IsSyncEnabled bool            `xorm:"INDEX NOT NULL DEFAULT false"`
		Cfg           core.Conversion `xorm:"TEXT"`

		Created     time.Time `xorm:"-"`
		CreatedUnix int64     `xorm:"INDEX"`
		Updated     time.Time `xorm:"-"`
		UpdatedUnix int64     `xorm:"INDEX"`
	}

	if err := x.Sync2(new(LoginSource)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
