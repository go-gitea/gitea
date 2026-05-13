// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestResolveSortOrder(t *testing.T) {
	m := map[string]map[string]db.SearchOrderBy{
		"asc":  {"id": "id ASC"},
		"desc": {"id": "id DESC"},
	}
	defaultOrder := db.SearchOrderBy("default")

	cases := []struct {
		path       string
		wantOK     bool
		wantOrder  db.SearchOrderBy
		wantStatus int
	}{
		{"GET /", true, defaultOrder, 0},
		{"GET /?sort=id", true, "id ASC", 0},
		{"GET /?sort=id&order=desc", true, "id DESC", 0},
		{"GET /?sort=bogus", false, "", http.StatusUnprocessableEntity},
		{"GET /?sort=id&order=bogus", false, "", http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			ctx, _ := contexttest.MockAPIContext(t, tc.path)
			got, ok := ResolveSortOrder(ctx, m, defaultOrder)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantOrder, got)
			assert.Equal(t, tc.wantStatus, ctx.Resp.WrittenStatus())
		})
	}
}
