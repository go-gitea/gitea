// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"math"

	"code.gitea.io/gitea/models/db"
)

// ParsePaginator parses a db.Paginator into a skip and limit
func ParsePaginator(paginator db.Paginator, max ...int) (int, int) {
	// Use a very large number to indicate no limit
	unlimited := math.MaxInt32
	if len(max) > 0 {
		// Some indexer engines have a limit on the page size, respect that
		unlimited = max[0]
	}

	if paginator == nil || paginator.IsListAll() {
		return 0, unlimited
	}

	// Warning: Do not use GetSkipTake() for *db.ListOptions
	// Its implementation could reset the page size with setting.API.MaxResponseItems
	if listOptions, ok := paginator.(*db.ListOptions); ok {
		if listOptions.Page >= 0 && listOptions.PageSize > 0 {
			var start int
			if listOptions.Page == 0 {
				start = 0
			} else {
				start = (listOptions.Page - 1) * listOptions.PageSize
			}
			return start, listOptions.PageSize
		}
		return 0, unlimited
	}

	return paginator.GetSkipTake()
}
