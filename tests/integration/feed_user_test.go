// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/xml"
	"net/http"
	"testing"

	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

// RSS is a struct to unmarshal RSS feeds test only
type RSS struct {
	Channel struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		Description string `xml:"description"`
		PubDate     string `xml:"pubDate"`
		Items       []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
			PubDate     string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
}

func TestFeedUser(t *testing.T) {
	t.Run("User", func(t *testing.T) {
		t.Run("Atom", func(t *testing.T) {
			defer tests.PrepareTestEnv(t)()

			req := NewRequest(t, "GET", "/user2.atom")
			resp := MakeRequest(t, req, http.StatusOK)

			data := resp.Body.String()
			assert.Contains(t, data, `<feed xmlns="http://www.w3.org/2005/Atom"`)
		})

		t.Run("RSS", func(t *testing.T) {
			defer tests.PrepareTestEnv(t)()

			req := NewRequest(t, "GET", "/user2.rss")
			resp := MakeRequest(t, req, http.StatusOK)

			data := resp.Body.String()
			assert.Contains(t, data, `<rss version="2.0"`)

			var rss RSS
			err := xml.Unmarshal(resp.Body.Bytes(), &rss)
			assert.NoError(t, err)
			assert.Contains(t, rss.Channel.Link, "/user2")
			assert.NotEmpty(t, rss.Channel.PubDate)
		})
	})
}
