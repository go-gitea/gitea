// Copyright 2022 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/models/db"
)

// ForeignReference represents external references
type ForeignReference struct {
	LocalID   int64  `xorm:"INDEX"`
	ForeignID int64  `xorm:"INDEX UNIQUE(external_reference_index)"`
	RepoID    int64  `xorm:"INDEX UNIQUE(external_reference_index)"`
	Type      string `xorm:"INDEX UNIQUE(external_reference_index)"`
}

func init() {
	db.RegisterModel(new(ForeignReference))
}
