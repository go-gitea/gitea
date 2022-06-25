// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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

func TestOk(t *testing.T) {
	testReal := func(s *Sitemap, name string, urls []URL, expected string) {
		for _, url := range urls {
			s.Add(url)
		}
		buf := &bytes.Buffer{}
		_, err := s.WriteTo(buf)
		assert.NoError(t, nil, err)
		assert.Equal(t, xml.Header+"<"+name+" xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">"+expected+"</"+name+">\n", buf.String())
	}
	test := func(urls []URL, expected string) {
		testReal(NewSitemap(), "urlset", urls, expected)
		testReal(NewSitemapIndex(), "sitemapindex", urls, expected)
	}

	ts := time.Unix(1651322008, 0).UTC()

	test(
		[]URL{},
		"",
	)
	test(
		[]URL{
			{URL: "https://gitea.io/test1", LastMod: &ts},
		},
		"<url><loc>https://gitea.io/test1</loc><lastmod>2022-04-30T12:33:28Z</lastmod></url>",
	)
	test(
		[]URL{
			{URL: "https://gitea.io/test2", LastMod: nil},
		},
		"<url><loc>https://gitea.io/test2</loc></url>",
	)
	test(
		[]URL{
			{URL: "https://gitea.io/test1", LastMod: &ts},
			{URL: "https://gitea.io/test2", LastMod: nil},
		},
		"<url><loc>https://gitea.io/test1</loc><lastmod>2022-04-30T12:33:28Z</lastmod></url>"+
			"<url><loc>https://gitea.io/test2</loc></url>",
	)
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
