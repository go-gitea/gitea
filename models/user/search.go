// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// SearchUserOptions contains the options for searching
type SearchUserOptions struct {
	db.ListOptions

	Keyword       string
	Type          UserType
	UID           int64
	OrderBy       db.SearchOrderBy
	Visible       []structs.VisibleType
	Actor         *User // The user doing the search
	SearchByEmail bool  // Search by email as well as username/full name

	IsActive           util.OptionalBool
	IsAdmin            util.OptionalBool
	IsRestricted       util.OptionalBool
	IsTwoFactorEnabled util.OptionalBool
	IsProhibitLogin    util.OptionalBool

	ExtraParamStrings map[string]string
}

func (opts *SearchUserOptions) toSearchQueryBase() *xorm.Session {
	var cond builder.Cond = builder.Eq{"type": opts.Type}
	if len(opts.Keyword) > 0 {
		lowerKeyword := strings.ToLower(opts.Keyword)
		keywordCond := builder.Or(
			builder.Like{"lower_name", lowerKeyword},
			builder.Like{"LOWER(full_name)", lowerKeyword},
		)
		if opts.SearchByEmail {
			keywordCond = keywordCond.Or(builder.Like{"LOWER(email)", lowerKeyword})
		}

		cond = cond.And(keywordCond)
	}

	// If visibility filtered
	if len(opts.Visible) > 0 {
		cond = cond.And(builder.In("visibility", opts.Visible))
	}

	if opts.Actor != nil {
		exprCond := builder.Expr("org_user.org_id = `user`.id")

		// If Admin - they see all users!
		if !opts.Actor.IsAdmin {
			// Force visibility for privacy
			var accessCond builder.Cond
			if !opts.Actor.IsRestricted {
				accessCond = builder.Or(
					builder.In("id", builder.Select("org_id").From("org_user").LeftJoin("`user`", exprCond).Where(builder.And(builder.Eq{"uid": opts.Actor.ID}, builder.Eq{"visibility": structs.VisibleTypePrivate}))),
					builder.In("visibility", structs.VisibleTypePublic, structs.VisibleTypeLimited))
			} else {
				// restricted users only see orgs they are a member of
				accessCond = builder.In("id", builder.Select("org_id").From("org_user").LeftJoin("`user`", exprCond).Where(builder.And(builder.Eq{"uid": opts.Actor.ID})))
			}
			// Don't forget about self
			accessCond = accessCond.Or(builder.Eq{"id": opts.Actor.ID})
			cond = cond.And(accessCond)
		}

	} else {
		// Force visibility for privacy
		// Not logged in - only public users
		cond = cond.And(builder.In("visibility", structs.VisibleTypePublic))
	}

	if opts.UID > 0 {
		cond = cond.And(builder.Eq{"id": opts.UID})
	}

	if !opts.IsActive.IsNone() {
		cond = cond.And(builder.Eq{"is_active": opts.IsActive.IsTrue()})
	}

	if !opts.IsAdmin.IsNone() {
		cond = cond.And(builder.Eq{"is_admin": opts.IsAdmin.IsTrue()})
	}

	if !opts.IsRestricted.IsNone() {
		cond = cond.And(builder.Eq{"is_restricted": opts.IsRestricted.IsTrue()})
	}

	if !opts.IsProhibitLogin.IsNone() {
		cond = cond.And(builder.Eq{"prohibit_login": opts.IsProhibitLogin.IsTrue()})
	}

	e := db.GetEngine(db.DefaultContext)
	if opts.IsTwoFactorEnabled.IsNone() {
		return e.Where(cond)
	}

	// 2fa filter uses LEFT JOIN to check whether a user has a 2fa record
	// While using LEFT JOIN, sometimes the performance might not be good, but it won't be a problem now, such SQL is seldom executed.
	// There are some possible methods to refactor this SQL in future when we really need to optimize the performance (but not now):
	// (1) add a column in user table (2) add a setting value in user_setting table (3) use search engines (bleve/elasticsearch)
	if opts.IsTwoFactorEnabled.IsTrue() {
		cond = cond.And(builder.Expr("two_factor.uid IS NOT NULL"))
	} else {
		cond = cond.And(builder.Expr("two_factor.uid IS NULL"))
	}

	return e.Join("LEFT OUTER", "two_factor", "two_factor.uid = `user`.id").
		Where(cond)
}

// SearchUsers takes options i.e. keyword and part of user name to search,
// it returns results in given range and number of total results.
func SearchUsers(opts *SearchUserOptions) (users []*User, _ int64, _ error) {
	sessCount := opts.toSearchQueryBase()
	defer sessCount.Close()
	count, err := sessCount.Count(new(User))
	if err != nil {
		return nil, 0, fmt.Errorf("Count: %v", err)
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = db.SearchOrderByAlphabetically
	}

	sessQuery := opts.toSearchQueryBase().OrderBy(opts.OrderBy.String())
	defer sessQuery.Close()
	if opts.Page != 0 {
		sessQuery = db.SetSessionPagination(sessQuery, opts)
	}

	// the sql may contain JOIN, so we must only select User related columns
	sessQuery = sessQuery.Select("`user`.*")
	users = make([]*User, 0, opts.PageSize)
	return users, count, sessQuery.Find(&users)
}

// IterateUser iterate users
func IterateUser(f func(user *User) error) error {
	var start int
	batchSize := setting.Database.IterateBufferSize
	for {
		users := make([]*User, 0, batchSize)
		if err := db.GetEngine(db.DefaultContext).Limit(batchSize, start).Find(&users); err != nil {
			return err
		}
		if len(users) == 0 {
			return nil
		}
		start += len(users)

		for _, user := range users {
			if err := f(user); err != nil {
				return err
			}
		}
	}
}
