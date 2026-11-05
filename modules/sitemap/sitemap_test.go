// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sitemap

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewSitemap(t *testing.T) {
	ts := time.Unix(1651322008, 0).UTC()
	for _, sitemap := range []struct {
		name        string
		newSitemap  func() *Sitemap
		root, entry string
		wantManyErr string
		wantSizeErr string
	}{
		{
			"NewSitemap", NewSitemap, "urlset", "url",
			"sitemap contains 50001 URLs, but only 50000 are allowed",
			"sitemap has 52428932 bytes, but only 52428800 are allowed",
		},
		{
			"NewSitemapIndex", NewSitemapIndex, "sitemapindex", "sitemap",
			"sitemap contains 50001 sub-sitemaps, but only 50000 are allowed",
			"sitemap has 52428952 bytes, but only 52428800 are allowed",
		},
	} {
		t.Run(sitemap.name, func(t *testing.T) {
			entryStart, entryEnd := "<"+sitemap.entry+">", "</"+sitemap.entry+">"
			tests := []struct {
				name    string
				urls    []URL
				want    string
				wantErr string
			}{
				{
					name: "empty",
					urls: []URL{},
				},
				{
					name: "regular",
					urls: []URL{
						{URL: "https://gitea.io/test1", LastMod: &ts},
					},
					want: entryStart + "<loc>https://gitea.io/test1</loc><lastmod>2022-04-30T12:33:28Z</lastmod>" + entryEnd,
				},
				{
					name: "without lastmod",
					urls: []URL{
						{URL: "https://gitea.io/test1"},
					},
					want: entryStart + "<loc>https://gitea.io/test1</loc>" + entryEnd,
				},
				{
					name: "multiple",
					urls: []URL{
						{URL: "https://gitea.io/test1", LastMod: &ts},
						{URL: "https://gitea.io/test2", LastMod: nil},
					},
					want: entryStart + "<loc>https://gitea.io/test1</loc><lastmod>2022-04-30T12:33:28Z</lastmod>" + entryEnd +
						entryStart + "<loc>https://gitea.io/test2</loc>" + entryEnd,
				},
				{
					name:    "too many " + sitemap.entry + "s",
					urls:    make([]URL, 50001),
					wantErr: sitemap.wantManyErr,
				},
				{
					name: "too big file",
					urls: []URL{
						{URL: strings.Repeat("b", 50*1024*1024+1)},
					},
					wantErr: sitemap.wantSizeErr,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					s := sitemap.newSitemap()
					for _, url := range tt.urls {
						s.Add(url)
					}
					buf := &bytes.Buffer{}
					_, err := s.WriteTo(buf)
					if tt.wantErr != "" {
						assert.EqualError(t, err, tt.wantErr)
					} else {
						assert.NoError(t, err)
						assert.Equalf(t,
							xml.Header+"<"+sitemap.root+` xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`+tt.want+"</"+sitemap.root+">\n",
							buf.String(), sitemap.name+"()",
						)
					}
				})
			}
		})
	}
}
