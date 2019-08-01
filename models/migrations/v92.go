// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package migrations

import "github.com/go-xorm/xorm"

func addProjectsInfo(x *xorm.Engine) error {

	type Repository struct {
		NumProjects       int `xorm:"NOT NULL DEFAULT 0"`
		NumClosedProjects int `xorm:"NOT NULL DEFAULT 0"`
		NumOpenProjects   int `xorm:"-"`
	}

	return x.Sync2(new(Repository))
}
