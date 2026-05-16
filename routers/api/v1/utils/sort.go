// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/services/context"
)

// ResolveSortOrder reads "sort" and "order" query params and returns the matching
// SearchOrderBy from orderByMap. When "sort" is absent, returns defaultOrder.
// On invalid input it writes a 422 response and returns ok=false; callers should
// then return immediately.
func ResolveSortOrder(ctx *context.APIContext, orderByMap map[string]map[string]db.SearchOrderBy, defaultOrder db.SearchOrderBy) (db.SearchOrderBy, bool) {
	sortMode := ctx.FormString("sort")
	if sortMode == "" {
		return defaultOrder, true
	}
	sortOrder := ctx.FormString("order")
	if sortOrder == "" {
		sortOrder = "asc"
	}
	orderMap, ok := orderByMap[sortOrder]
	if !ok {
		ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("Invalid sort order: %q", sortOrder))
		return "", false
	}
	orderBy, ok := orderMap[sortMode]
	if !ok {
		ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("Invalid sort mode: %q", sortMode))
		return "", false
	}
	return orderBy, true
}
