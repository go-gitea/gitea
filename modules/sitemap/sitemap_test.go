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

	tests := []struct {
		name    string
		urls    []URL
		want    string
		wantErr string
	}{
		{
			name: "empty",
			urls: []URL{},
			want: xml.Header + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` +
				"" +
				"</urlset>\n",
		},
		{
			name: "regular",
			urls: []URL{
				{URL: "https://gitea.io/test1", LastMod: &ts},
			},
			want: xml.Header + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` +
				"<url><loc>https://gitea.io/test1</loc><lastmod>2022-04-30T12:33:28Z</lastmod></url>" +
				"</urlset>\n",
		},
		{
			name: "without lastmod",
			urls: []URL{
				{URL: "https://gitea.io/test1"},
			},
			want: xml.Header + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` +
				"<url><loc>https://gitea.io/test1</loc></url>" +
				"</urlset>\n",
		},
		{
			name: "multiple",
			urls: []URL{
				{URL: "https://gitea.io/test1", LastMod: &ts},
				{URL: "https://gitea.io/test2", LastMod: nil},
			},
			want: xml.Header + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` +
				"<url><loc>https://gitea.io/test1</loc><lastmod>2022-04-30T12:33:28Z</lastmod></url>" +
				"<url><loc>https://gitea.io/test2</loc></url>" +
				"</urlset>\n",
		},
		{
			name:    "too many urls",
			urls:    make([]URL, 50001),
			wantErr: "The sitemap contains 50001 URLs, but only 50000 are allowed",
		},
		{
			name: "too big file",
			urls: []URL{
				{URL: strings.Repeat("b", 50*1024*1024+1)},
			},
			wantErr: "The sitemap has 52428932 bytes, but only 52428800 are allowed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSitemap()
			for _, url := range tt.urls {
				s.Add(url)
			}
			buf := &bytes.Buffer{}
			_, err := s.WriteTo(buf)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equalf(t, tt.want, buf.String(), "NewSitemap()")
			}
		})
	}
}

func TestNewSitemapIndex(t *testing.T) {
	ts := time.Unix(1651322008, 0).UTC()

	tests := []struct {
		name    string
		urls    []URL
		want    string
		wantErr string
	}{
		{
			name: "empty",
			urls: []URL{},
			want: xml.Header + `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` +
				"" +
				"</sitemapindex>\n",
		},
		{
			name: "regular",
			urls: []URL{
				{URL: "https://gitea.io/test1", LastMod: &ts},
			},
			want: xml.Header + `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` +
				"<sitemap><loc>https://gitea.io/test1</loc><lastmod>2022-04-30T12:33:28Z</lastmod></sitemap>" +
				"</sitemapindex>\n",
		},
		{
			name: "without lastmod",
			urls: []URL{
				{URL: "https://gitea.io/test1"},
			},
			want: xml.Header + `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` +
				"<sitemap><loc>https://gitea.io/test1</loc></sitemap>" +
				"</sitemapindex>\n",
		},
		{
			name: "multiple",
			urls: []URL{
				{URL: "https://gitea.io/test1", LastMod: &ts},
				{URL: "https://gitea.io/test2", LastMod: nil},
			},
			want: xml.Header + `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` +
				"<sitemap><loc>https://gitea.io/test1</loc><lastmod>2022-04-30T12:33:28Z</lastmod></sitemap>" +
				"<sitemap><loc>https://gitea.io/test2</loc></sitemap>" +
				"</sitemapindex>\n",
		},
		{
			name:    "too many sitemaps",
			urls:    make([]URL, 50001),
			wantErr: "The sitemap contains 50001 sub-sitemaps, but only 50000 are allowed",
		},
		{
			name: "too big file",
			urls: []URL{
				{URL: strings.Repeat("b", 50*1024*1024+1)},
			},
			wantErr: "The sitemap has 52428952 bytes, but only 52428800 are allowed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSitemapIndex()
			for _, url := range tt.urls {
				s.Add(url)
			}
			buf := &bytes.Buffer{}
			_, err := s.WriteTo(buf)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equalf(t, tt.want, buf.String(), "NewSitemapIndex()")
			}
		})
	}
}
