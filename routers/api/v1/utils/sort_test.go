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
	m := map[string]map[string]db.SearchOrderBy{"asc": {"id": "x"}, "desc": {"id": "x"}}
	for _, path := range []string{"GET /?sort=bogus", "GET /?sort=id&order=bogus"} {
		t.Run(path, func(t *testing.T) {
			ctx, _ := contexttest.MockAPIContext(t, path)
			_, ok := ResolveSortOrder(ctx, m, "")
			assert.False(t, ok)
			assert.Equal(t, http.StatusUnprocessableEntity, ctx.Resp.WrittenStatus())
		})
	}
}
