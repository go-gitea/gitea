// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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

// MarkupOption markup options
type MarkupOption struct {
	// Text markup to render
	//
	// in: body
	Text string
	// Mode to render (comment, gfm, markdown, file)
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
	// File path for detecting extension in file mode
	//
	// in: body
	FilePath string
}

// MarkupRender is a rendered markup document
// swagger:response MarkupRender
type MarkupRender string

// MarkdownOption markdown options
type MarkdownOption struct {
	// Text markdown to render
	//
	// in: body
	Text string
	// Mode to render (comment, gfm, markdown)
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

// GitignoreTemplateInfo name and text of a gitignore template
type GitignoreTemplateInfo struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// LicensesListEntry is used for the API
type LicensesTemplateListEntry struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// LicensesInfo contains information about a License
type LicenseTemplateInfo struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	Implementation string `json:"implementation"`
	Body           string `json:"body"`
}

// APIError is an api error with a message
type APIError struct {
	Message string `json:"message"`
	URL     string `json:"url"`
}
