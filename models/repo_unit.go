// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"

	"github.com/go-xorm/xorm"
)

// RepoUnit describes all units of a repository
type RepoUnit struct {
	ID          int64
	RepoID      int64    `xorm:"INDEX(s)"`
	Type        UnitType `xorm:"INDEX(s)"`
	Index       int
	Config      map[string]string `xorm:"JSON"`
	CreatedUnix int64             `xorm:"INDEX CREATED"`
	Created     time.Time         `xorm:"-"`
}

// AfterSet is invoked from XORM after setting the value of a field of this object.
func (r *RepoUnit) AfterSet(colName string, _ xorm.Cell) {
	switch colName {
	case "created_unix":
		r.Created = time.Unix(r.CreatedUnix, 0).Local()
	}
}

// Unit returns Unit
func (r *RepoUnit) Unit() Unit {
	return Units[r.Type]
}
