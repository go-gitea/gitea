// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"strconv"

	"gitea.dev/models/db"
	"gitea.dev/modules/structs"

	"xorm.io/builder"
)

// ExternalAccountLink attributes content migrated from a git service to a local user, without allowing sign-in
type ExternalAccountLink struct {
	ID          int64                  `xorm:"pk autoincr"`
	ServiceType structs.GitServiceType `xorm:"UNIQUE(s)"`
	ExternalID  int64                  `xorm:"UNIQUE(s)"`
	UserID      int64                  `xorm:"INDEX"`
}

func init() {
	db.RegisterModel(new(ExternalAccountLink))
}

type FindExternalAccountLinksOptions struct {
	db.ListOptions
	UserID int64
}

func (opts FindExternalAccountLinksOptions) ToConds() builder.Cond {
	return builder.Eq{"user_id": opts.UserID}
}

func (opts FindExternalAccountLinksOptions) ToOrders() string {
	return "id"
}

func GetUserIDByExternalUserID(ctx context.Context, tp structs.GitServiceType, externalID int64) (int64, error) {
	var userID int64
	has, err := db.GetEngine(ctx).Table(new(ExternalAccountLink)).Select("user_id").
		Where("service_type=? AND external_id=?", tp, externalID).Get(&userID)
	if err != nil || has {
		return userID, err
	}
	_, err = db.GetEngine(ctx).Table(new(ExternalLoginUser)).Select("user_id").
		Where("provider=? AND external_id=?", tp.Name(), strconv.FormatInt(externalID, 10)).Get(&userID)
	return userID, err
}
