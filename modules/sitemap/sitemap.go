// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sitemap

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"time"
)

// sitemapFileLimit contains the maximum size of a sitemap file
const sitemapFileLimit = 50 * 1024 * 1024

// Url represents a single sitemap entry
type URL struct {
	URL     string     `xml:"loc"`
	LastMod *time.Time `xml:"lastmod,omitempty"`
}

// SitemapUrl represents a sitemap
type Sitemap struct {
	XMLName   xml.Name
	Namespace string `xml:"xmlns,attr"`

	URLs []URL `xml:"url"`
}

// NewSitemap creates a sitemap
func NewSitemap() *Sitemap {
	return &Sitemap{
		XMLName:   xml.Name{Local: "urlset"},
		Namespace: "http://www.sitemaps.org/schemas/sitemap/0.9",
	}
}

// NewSitemap creates a sitemap index.
func NewSitemapIndex() *Sitemap {
	return &Sitemap{
		XMLName:   xml.Name{Local: "sitemapindex"},
		Namespace: "http://www.sitemaps.org/schemas/sitemap/0.9",
	}
}

// Add adds a URL to the sitemap
func (s *Sitemap) Add(u URL) {
	s.URLs = append(s.URLs, u)
}

// Write writes the sitemap to a response
func (s *Sitemap) WriteTo(w io.Writer) (int64, error) {
	if len(s.URLs) > 50000 {
		return 0, fmt.Errorf("The sitemap contains too many URLs: %d", len(s.URLs))
	}
	buf := bytes.NewBufferString(xml.Header)
	if err := xml.NewEncoder(buf).Encode(s); err != nil {
		return 0, err
	}
	if err := buf.WriteByte('\n'); err != nil {
		return 0, err
	}
	if buf.Len() > sitemapFileLimit {
		return 0, fmt.Errorf("The sitemap is too big: %d", buf.Len())
	}
	return buf.WriteTo(w)
}
