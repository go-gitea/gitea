// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conan

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestDecodeJSONBody(t *testing.T) {
	type packageReferences struct {
		References []string `json:"package_ids"`
	}

	t.Run("Normal", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "POST /")
		ctx.Req.Body = io.NopCloser(strings.NewReader(`{"package_ids":["a","b"]}`))

		var ids packageReferences
		assert.True(t, decodeJSONBody(ctx, &ids, http.StatusBadRequest))
		assert.Equal(t, []string{"a", "b"}, ids.References)
		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("Invalid", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "POST /")
		ctx.Req.Body = io.NopCloser(strings.NewReader(`{"package_ids":`))

		var ids packageReferences
		assert.False(t, decodeJSONBody(ctx, &ids, http.StatusBadRequest))
		assert.Equal(t, http.StatusBadRequest, resp.Code)
	})

	t.Run("TooLarge", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "POST /")
		body := `{"package_ids":["` + strings.Repeat("a", maxJSONBodySize) + `"]}`
		ctx.Req.Body = io.NopCloser(strings.NewReader(body))

		var ids packageReferences
		assert.False(t, decodeJSONBody(ctx, &ids, http.StatusBadRequest))
		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.Code)
	})
}
