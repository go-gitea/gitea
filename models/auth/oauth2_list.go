// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"code.gitea.io/gitea/models/db"

	"xorm.io/builder"
)

type FindOAuth2ApplicationsOptions struct {
	db.ListOptions
	// OwnerID is the user id or org id of the owner of the application
	OwnerID int64
	// find global applications, if true, then OwnerID will be igonred
	IsGlobal bool
}

func (opts FindOAuth2ApplicationsOptions) ToConds() builder.Cond {
	conds := builder.NewCond()
	if opts.IsGlobal {
		conds = conds.And(builder.Eq{"uid": 0})
	} else if opts.OwnerID != 0 {
		conds = conds.And(builder.Eq{"uid": opts.OwnerID})
	}
	return conds
}

func (opts FindOAuth2ApplicationsOptions) ToOrders() string {
	return "id DESC"
}
