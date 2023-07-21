// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"math"

	"code.gitea.io/gitea/models/db"
)

// ParsePaginator parses a db.Paginator into a skip and limit
func ParsePaginator(paginator db.Paginator) (int, int) {
	if paginator == nil || paginator.IsListAll() {
		// Use a very large number to list all
		return 0, math.MaxInt
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
		// Use a very large number to indicate no limit
		return 0, math.MaxInt
	}

	return paginator.GetSkipTake()
}
