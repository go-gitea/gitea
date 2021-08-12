// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

// SessionPaginator to paginate database sessions
type SessionPaginator interface {
	GetPaginatedSession() *xorm.Session
	SetSessionPagination(sess *xorm.Session) *xorm.Session
}

// AbsoluteSessionPaginator to paginate results
type AbsoluteSessionPaginator struct {
	Skip int
	Take int
}

// GetPaginatedSession creates a paginated database session
func (opts *AbsoluteSessionPaginator) GetPaginatedSession() *xorm.Session {
	opts.setDefaultValues()

	return x.Limit(opts.Take, opts.Skip)
}

// SetSessionPagination sets pagination for a database session
func (opts *AbsoluteSessionPaginator) SetSessionPagination(sess *xorm.Session) *xorm.Session {
	opts.setDefaultValues()

	return sess.Limit(opts.Take, opts.Skip)
}

func (opts *AbsoluteSessionPaginator) setDefaultValues() {
	if opts.Take <= 0 {
		opts.Take = setting.API.DefaultPagingNum
	}
	if opts.Take > setting.API.MaxResponseItems {
		opts.Take = setting.API.MaxResponseItems
	}
}
