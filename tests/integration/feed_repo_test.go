// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/xml"
	"net/http"
	"testing"

	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestFeedRepo(t *testing.T) {
	t.Run("RSS", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()

		req := NewRequest(t, "GET", "/user2/repo1.rss")
		resp := MakeRequest(t, req, http.StatusOK)

		data := resp.Body.String()
		assert.Contains(t, data, `<rss version="2.0"`)

		var rss RSS
		err := xml.Unmarshal(resp.Body.Bytes(), &rss)
		assert.NoError(t, err)
		assert.Contains(t, rss.Channel.Link, "/user2/repo1")
		assert.NotEmpty(t, rss.Channel.PubDate)
		assert.Len(t, rss.Channel.Items, 1)
		assert.EqualValues(t, "issue5", rss.Channel.Items[0].Description)
		assert.NotEmpty(t, rss.Channel.Items[0].PubDate)
	})
}
