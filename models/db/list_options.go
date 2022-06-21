// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

const (
	// DefaultMaxInSize represents default variables number on IN () in SQL
	DefaultMaxInSize = 50
)

// Paginator is the base for different ListOptions types
type Paginator interface {
	GetSkipTake() (skip, take int)
	GetStartEnd() (start, end int)
}

// GetPaginatedSession creates a paginated database session
func GetPaginatedSession(p Paginator) *xorm.Session {
	skip, take := p.GetSkipTake()

	return x.Limit(take, skip)
}

// SetSessionPagination sets pagination for a database session
func SetSessionPagination(sess Engine, p Paginator) *xorm.Session {
	skip, take := p.GetSkipTake()

	return sess.Limit(take, skip)
}

// SetEnginePagination sets pagination for a database engine
func SetEnginePagination(e Engine, p Paginator) Engine {
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
	opts.SetDefaultValues()
	return (opts.Page - 1) * opts.PageSize, opts.PageSize
}

// GetStartEnd returns the start and end of the ListOptions
func (opts *ListOptions) GetStartEnd() (start, end int) {
	start, take := opts.GetSkipTake()
	end = start + take
	return start, end
}

// SetDefaultValues sets default values
func (opts *ListOptions) SetDefaultValues() {
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
