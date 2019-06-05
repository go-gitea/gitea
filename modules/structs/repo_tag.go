// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// Tag represents a repository tag for the /tags API endpoint
type Tag struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	Commit struct {
		SHA string `json:"sha"`
		URL string `json:"url"`
	} `json:"commit"`
	ZipballURL string `json:"zipball_url"`
	TarballURL string `json:"tarball_url"`
}

// GitTag represents a git tag for the /git/tags API endpoint
type GitTag struct {
	Tag          string                     `json:"tag"`
	SHA          string                     `json:"sha"`
	URL          string                     `json:"url"`
	Message      string                     `json:"message"`
	Tagger       *CommitUser                `json:"tagger"`
	Object       *GitTagObject              `json:"object"`
	Verification *PayloadCommitVerification `json:"verification"`
}

// GitTagObject contains meta information of the tag object
type GitTagObject struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	SHA  string `json:"sha"`
}
