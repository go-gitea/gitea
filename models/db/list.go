// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"strings"

	"gitea.dev/modules/setting"

	"xorm.io/builder"
)

const (
	// DefaultMaxInSize represents default variables number on IN () in SQL
	DefaultMaxInSize     = 50
	defaultFindSliceSize = 10
)

// Paginator is the base for different ListOptions types
type Paginator interface {
	GetSkipTake() (skip, take int)
	IsListAll() bool
}

// SetSessionPagination sets pagination for a database session
func SetSessionPagination(sess Engine, p Paginator) {
	skip, take := p.GetSkipTake()
	sess.Limit(take, skip)
}

// ListOptions options to paginate results
type ListOptions struct {
	PageSize int
	Page     int  // start from 1
	ListAll  bool // if true, then PageSize and Page will not be taken
}

var ListOptionsAll = ListOptions{ListAll: true}

var (
	_ Paginator   = &ListOptions{}
	_ FindOptions = ListOptions{}
)

// GetSkipTake returns the skip and take values
func (opts *ListOptions) GetSkipTake() (skip, take int) {
	opts.SetDefaultValues()
	return (opts.Page - 1) * opts.PageSize, opts.PageSize
}

func (opts ListOptions) GetPage() int {
	return opts.Page
}

func (opts ListOptions) GetPageSize() int {
	return opts.PageSize
}

// IsListAll indicates PageSize and Page will be ignored
func (opts ListOptions) IsListAll() bool {
	return opts.ListAll
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

func (opts ListOptions) ToConds() builder.Cond {
	return builder.NewCond()
}

// AbsoluteListOptions absolute options to paginate results
type AbsoluteListOptions struct {
	skip int
	take int
}

var _ Paginator = &AbsoluteListOptions{}

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

// IsListAll will always return false
func (opts *AbsoluteListOptions) IsListAll() bool {
	return false
}

// GetSkipTake returns the skip and take values
func (opts *AbsoluteListOptions) GetSkipTake() (skip, take int) {
	return opts.skip, opts.take
}

// FindOptions represents a find options
type FindOptions interface {
	GetPage() int
	GetPageSize() int
	IsListAll() bool
	ToConds() builder.Cond
}

type JoinFunc func(sess Engine) error

type FindOptionsJoin interface {
	ToJoins() []JoinFunc
}

type FindOptionsOrder interface {
	ToOrders() string
}

var sqlIdentifierUnquoter = strings.NewReplacer("`", "", `"`, "", "[", "", "]", "")

func appendPrimaryKeyOrder(order, qualifiedPrimaryKey string) string {
	order = strings.TrimSpace(order)
	if qualifiedPrimaryKey == "" {
		return order
	}

	lastTermStart, depth := 0, 0
	for i, char := range order {
		switch char {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				lastTermStart = i + 1
			}
		}
	}
	fields := strings.Fields(sqlIdentifierUnquoter.Replace(order[lastTermStart:]))
	primaryKey := sqlIdentifierUnquoter.Replace(qualifiedPrimaryKey)
	if len(fields) > 0 && (strings.EqualFold(fields[0], primaryKey) ||
		strings.EqualFold(fields[0], primaryKey[strings.LastIndex(primaryKey, ".")+1:])) {
		return order
	}

	direction := "ASC"
	if len(fields) > 0 && strings.EqualFold(fields[len(fields)-1], "DESC") {
		direction = "DESC"
	}
	if order == "" {
		return qualifiedPrimaryKey + " " + direction
	}
	return order + ", " + qualifiedPrimaryKey + " " + direction
}

func orderWithPrimaryKey[T any](order string) string {
	var bean T
	table, err := xormEngine.TableInfo(&bean)
	if err != nil || len(table.PrimaryKeys) != 1 {
		return order
	}
	return appendPrimaryKeyOrder(order, xormEngine.Dialect().Quoter().Quote(
		table.Name+"."+table.PrimaryKeys[0],
	))
}

func applyOrder[T any](sess Engine, opts FindOptions, paginated bool) {
	var order string
	if orderOpt, ok := opts.(FindOptionsOrder); ok {
		order = orderOpt.ToOrders()
	}
	if order == "" && !paginated {
		return
	}
	if order = orderWithPrimaryKey[T](order); order != "" {
		sess.OrderBy(order)
	}
}

// Find represents a common find function which accept an options interface
func Find[T any](ctx context.Context, opts FindOptions) ([]*T, error) {
	sess := GetEngine(ctx).Where(opts.ToConds())

	if joinOpt, ok := opts.(FindOptionsJoin); ok {
		for _, joinFunc := range joinOpt.ToJoins() {
			if err := joinFunc(sess); err != nil {
				return nil, err
			}
		}
	}
	page, pageSize := opts.GetPage(), opts.GetPageSize()
	paginated := !opts.IsListAll() && pageSize > 0
	applyOrder[T](sess, opts, paginated)
	if paginated {
		if page == 0 {
			page = 1
		}
		sess.Limit(pageSize, (page-1)*pageSize)
	}

	findPageSize := defaultFindSliceSize
	if pageSize > 0 {
		findPageSize = pageSize
	}
	objects := make([]*T, 0, findPageSize)
	if err := sess.Find(&objects); err != nil {
		return nil, err
	}
	return objects, nil
}

// Count represents a common count function which accept an options interface
func Count[T any](ctx context.Context, opts FindOptions) (int64, error) {
	sess := GetEngine(ctx).Where(opts.ToConds())
	if joinOpt, ok := opts.(FindOptionsJoin); ok {
		for _, joinFunc := range joinOpt.ToJoins() {
			if err := joinFunc(sess); err != nil {
				return 0, err
			}
		}
	}

	var object T
	return sess.Count(&object)
}

// FindAndCount represents a common findandcount function which accept an options interface
func FindAndCount[T any](ctx context.Context, opts FindOptions) ([]*T, int64, error) {
	sess := GetEngine(ctx).Where(opts.ToConds())
	page, pageSize := opts.GetPage(), opts.GetPageSize()
	paginated := !opts.IsListAll() && pageSize > 0 && page >= 1
	if paginated {
		sess.Limit(pageSize, (page-1)*pageSize)
	}
	if joinOpt, ok := opts.(FindOptionsJoin); ok {
		for _, joinFunc := range joinOpt.ToJoins() {
			if err := joinFunc(sess); err != nil {
				return nil, 0, err
			}
		}
	}
	applyOrder[T](sess, opts, paginated)

	findPageSize := defaultFindSliceSize
	if pageSize > 0 {
		findPageSize = pageSize
	}
	objects := make([]*T, 0, findPageSize)
	cnt, err := sess.FindAndCount(&objects)
	if err != nil {
		return nil, 0, err
	}
	return objects, cnt, nil
}
