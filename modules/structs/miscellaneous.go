// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// SearchResults results of a successful search
type SearchResults struct {
	OK   bool          `json:"ok"`
	Data []*Repository `json:"data"`
}

// SearchError error of a failed search
type SearchError struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

// MarkdownOption markdown options
type MarkdownOption struct {
	// Text markdown to render
	//
	// in: body
	Text string
	// Mode to render
	//
	// in: body
	Mode string
	// Context to render
	//
	// in: body
	Context string
	// Is it a wiki page ?
	//
	// in: body
	Wiki bool
}

// MarkdownRender is a rendered markdown document
// swagger:response MarkdownRender
type MarkdownRender string

// ServerVersion wraps the version of the server
type ServerVersion struct {
	Version string `json:"version"`
}
