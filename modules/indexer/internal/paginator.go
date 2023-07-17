// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"math"

	"code.gitea.io/gitea/models/db"
)

// ParsePaginator parses a db.Paginator into a skip and limit
func ParsePaginator(paginator db.Paginator) (int, int) {
	if paginator == nil {
		// Use default values
		return 0, 50
	}

	if paginator.IsListAll() {
		// Use a very large number to list all
		return 0, math.MaxInt
	}

	return paginator.GetSkipTake()
}
