// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sitemap

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewSitemap(t *testing.T) {
	ts := time.Unix(1651322008, 0).UTC()

	tests := []struct {
		name string
		urls []URL
		want string
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSitemap()
			for _, url := range tt.urls {
				s.Add(url)
			}
			buf := &bytes.Buffer{}
			_, err := s.WriteTo(buf)
			assert.NoError(t, nil, err)
			assert.Equalf(t, tt.want, buf.String(), "NewSitemap()")
		})
	}
}

func TestNewSitemapIndex(t *testing.T) {
	ts := time.Unix(1651322008, 0).UTC()

	tests := []struct {
		name string
		urls []URL
		want string
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSitemapIndex()
			for _, url := range tt.urls {
				s.Add(url)
			}
			buf := &bytes.Buffer{}
			_, err := s.WriteTo(buf)
			assert.NoError(t, nil, err)
			assert.Equalf(t, tt.want, buf.String(), "NewSitemapIndex()")
		})
	}
}

func TestTooManyURLs(t *testing.T) {
	s := NewSitemap()
	for i := 0; i < 50001; i++ {
		s.Add(URL{URL: fmt.Sprintf("https://gitea.io/test%d", i)})
	}
	buf := &bytes.Buffer{}
	_, err := s.WriteTo(buf)
	assert.EqualError(t, err, "The sitemap contains too many URLs: 50001")
}

func TestSitemapTooBig(t *testing.T) {
	s := NewSitemap()
	s.Add(URL{URL: strings.Repeat("b", sitemapFileLimit)})
	buf := &bytes.Buffer{}
	_, err := s.WriteTo(buf)
	assert.EqualError(t, err, "The sitemap is too big: 52428931")
}
