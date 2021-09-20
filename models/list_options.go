// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

// Paginator is the base for different ListOptions types
type Paginator interface {
	GetSkipTake() (skip, take int)
	GetStartEnd() (start, end int)
}

// getPaginatedSession creates a paginated database session
func getPaginatedSession(p Paginator) *xorm.Session {
	skip, take := p.GetSkipTake()

	return db.DefaultContext().Engine().Limit(take, skip)
}

// setSessionPagination sets pagination for a database session
func setSessionPagination(sess *xorm.Session, p Paginator) *xorm.Session {
	skip, take := p.GetSkipTake()

	return sess.Limit(take, skip)
}

// setSessionPagination sets pagination for a database engine
func setEnginePagination(e db.Engine, p Paginator) db.Engine {
	skip, take := p.GetSkipTake()

	return e.Limit(take, skip)
}

// ListOptions options to paginate results
type ListOptions struct {
	PageSize int
	Page     int // start from 1
}

// GetSkipTake returns the skip and take values
func (opts *ListOptions) GetSkipTake() (skip, take int) {
	opts.setDefaultValues()
	return (opts.Page - 1) * opts.PageSize, opts.PageSize
}

// GetStartEnd returns the start and end of the ListOptions
func (opts *ListOptions) GetStartEnd() (start, end int) {
	start, take := opts.GetSkipTake()
	end = start + take
	return
}

func (opts *ListOptions) setDefaultValues() {
	if opts.PageSize <= 0 {
		opts.PageSize = setting.API.DefaultPagingNum
	}
	if opts.PageSize > setting.API.MaxResponseItems {
		opts.PageSize = setting.API.MaxResponseItems
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}
}

// AbsoluteListOptions absolute options to paginate results
type AbsoluteListOptions struct {
	skip int
	take int
}

// NewAbsoluteListOptions creates a list option with applied limits
func NewAbsoluteListOptions(skip, take int) *AbsoluteListOptions {
	if skip < 0 {
		skip = 0
	}
	if take <= 0 {
		take = setting.API.DefaultPagingNum
	}
	if take > setting.API.MaxResponseItems {
		take = setting.API.MaxResponseItems
	}
	return &AbsoluteListOptions{skip, take}
}

// GetSkipTake returns the skip and take values
func (opts *AbsoluteListOptions) GetSkipTake() (skip, take int) {
	return opts.skip, opts.take
}

// GetStartEnd returns the start and end values
func (opts *AbsoluteListOptions) GetStartEnd() (start, end int) {
	return opts.skip, opts.skip + opts.take
}
