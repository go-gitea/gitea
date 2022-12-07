// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"

	"xorm.io/builder"
)

// SearchOrderBy is used to sort the result
type SearchOrderBy string

func (s SearchOrderBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	SearchOrderByAlphabetically        SearchOrderBy = "name ASC"
	SearchOrderByAlphabeticallyReverse SearchOrderBy = "name DESC"
	SearchOrderByLeastUpdated          SearchOrderBy = "updated_unix ASC"
	SearchOrderByRecentUpdated         SearchOrderBy = "updated_unix DESC"
	SearchOrderByOldest                SearchOrderBy = "created_unix ASC"
	SearchOrderByNewest                SearchOrderBy = "created_unix DESC"
	SearchOrderBySize                  SearchOrderBy = "size ASC"
	SearchOrderBySizeReverse           SearchOrderBy = "size DESC"
	SearchOrderByID                    SearchOrderBy = "id ASC"
	SearchOrderByIDReverse             SearchOrderBy = "id DESC"
	SearchOrderByStars                 SearchOrderBy = "num_stars ASC"
	SearchOrderByStarsReverse          SearchOrderBy = "num_stars DESC"
	SearchOrderByForks                 SearchOrderBy = "num_forks ASC"
	SearchOrderByForksReverse          SearchOrderBy = "num_forks DESC"
)

// FindObjects represents a common function to find Objects from database according cond and ListOptions
func FindObjects[Object any](ctx context.Context, cond builder.Cond, opts *ListOptions, objects *[]*Object) error {
	sess := GetEngine(ctx).Where(cond)
	if opts != nil && opts.PageSize > 0 {
		if opts.Page < 1 {
			opts.Page = 1
		}
		sess.Limit(opts.PageSize, opts.PageSize*(opts.Page-1))
	}
	return sess.Find(objects)
}
