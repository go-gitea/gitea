// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// SearchResults results of a successful search
type SearchResults struct {
	// OK indicates if the search was successful
	OK bool `json:"ok"`
	// Data contains the repository search results
	Data []*Repository `json:"data"`
}

// SearchError error of a failed search
type SearchError struct {
	// OK indicates the search status (always false for errors)
	OK bool `json:"ok"`
	// Error contains the error message
	Error string `json:"error"`
}

// MarkupOption markup options
type MarkupOption struct {
	// Text markup to render
	//
	// in: body
	Text string
	// Mode to render (markdown, comment, wiki, file)
	//
	// in: body
	Mode string
	// URL path for rendering issue, media and file links
	// Expected format: /subpath/{user}/{repo}/src/{branch, commit, tag}/{identifier/path}/{file/dir}
	//
	// in: body
	Context string
	// Is it a wiki page? (use mode=wiki instead)
	//
	// Deprecated: true
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
	// Mode to render (markdown, comment, wiki, file)
	//
	// in: body
	Mode string
	// URL path for rendering issue, media and file links
	// Expected format: /subpath/{user}/{repo}/src/{branch, commit, tag}/{identifier/path}/{file/dir}
	//
	// in: body
	Context string
	// Is it a wiki page? (use mode=wiki instead)
	//
	// Deprecated: true
	// in: body
	Wiki bool
}

// MarkdownRender is a rendered markdown document
// swagger:response MarkdownRender
type MarkdownRender string

// ServerVersion wraps the version of the server
type ServerVersion struct {
	// Version is the server version string
	Version string `json:"version"`
}

// GitignoreTemplateInfo name and text of a gitignore template
type GitignoreTemplateInfo struct {
	// Name is the name of the gitignore template
	Name string `json:"name"`
	// Source contains the content of the gitignore template
	Source string `json:"source"`
}

// LicensesListEntry is used for the API
type LicensesTemplateListEntry struct {
	// Key is the unique identifier for the license template
	Key string `json:"key"`
	// Name is the display name of the license
	Name string `json:"name"`
	// URL is the reference URL for the license
	URL string `json:"url"`
}

// LicensesInfo contains information about a License
type LicenseTemplateInfo struct {
	// Key is the unique identifier for the license template
	Key string `json:"key"`
	// Name is the display name of the license
	Name string `json:"name"`
	// URL is the reference URL for the license
	URL string `json:"url"`
	// Implementation contains license implementation details
	Implementation string `json:"implementation"`
	// Body contains the full text of the license
	Body string `json:"body"`
}

// APIError is an api error with a message
type APIError struct {
	// Message contains the error description
	Message string `json:"message"`
	// URL contains the documentation URL for this error
	URL string `json:"url"`
}
