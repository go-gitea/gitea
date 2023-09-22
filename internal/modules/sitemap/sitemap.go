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

const (
	sitemapFileLimit = 50 * 1024 * 1024 // the maximum size of a sitemap file
	urlsLimit        = 50000

	schemaURL        = "http://www.sitemaps.org/schemas/sitemap/0.9"
	urlsetName       = "urlset"
	sitemapindexName = "sitemapindex"
)

// URL represents a single sitemap entry
type URL struct {
	URL     string     `xml:"loc"`
	LastMod *time.Time `xml:"lastmod,omitempty"`
}

// Sitemap represents a sitemap
type Sitemap struct {
	XMLName   xml.Name
	Namespace string `xml:"xmlns,attr"`

	URLs     []URL `xml:"url"`
	Sitemaps []URL `xml:"sitemap"`
}

// NewSitemap creates a sitemap
func NewSitemap() *Sitemap {
	return &Sitemap{
		XMLName:   xml.Name{Local: urlsetName},
		Namespace: schemaURL,
	}
}

// NewSitemapIndex creates a sitemap index.
func NewSitemapIndex() *Sitemap {
	return &Sitemap{
		XMLName:   xml.Name{Local: sitemapindexName},
		Namespace: schemaURL,
	}
}

// Add adds a URL to the sitemap
func (s *Sitemap) Add(u URL) {
	if s.XMLName.Local == sitemapindexName {
		s.Sitemaps = append(s.Sitemaps, u)
	} else {
		s.URLs = append(s.URLs, u)
	}
}

// WriteTo writes the sitemap to a response
func (s *Sitemap) WriteTo(w io.Writer) (int64, error) {
	if l := len(s.URLs); l > urlsLimit {
		return 0, fmt.Errorf("The sitemap contains %d URLs, but only %d are allowed", l, urlsLimit)
	}
	if l := len(s.Sitemaps); l > urlsLimit {
		return 0, fmt.Errorf("The sitemap contains %d sub-sitemaps, but only %d are allowed", l, urlsLimit)
	}
	buf := bytes.NewBufferString(xml.Header)
	if err := xml.NewEncoder(buf).Encode(s); err != nil {
		return 0, err
	}
	if err := buf.WriteByte('\n'); err != nil {
		return 0, err
	}
	if buf.Len() > sitemapFileLimit {
		return 0, fmt.Errorf("The sitemap has %d bytes, but only %d are allowed", buf.Len(), sitemapFileLimit)
	}
	return buf.WriteTo(w)
}
