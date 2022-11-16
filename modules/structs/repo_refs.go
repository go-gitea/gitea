// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Reference represents a Git reference.
type Reference struct {
	Ref    string     `json:"ref"`
	URL    string     `json:"url"`
	Object *GitObject `json:"object"`
}

// GitObject represents a Git object.
type GitObject struct {
	Type string `json:"type"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
}
