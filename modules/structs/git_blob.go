// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// GitBlobResponse represents a git blob
type GitBlobResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	URL      string `json:"url"`
	SHA      string `json:"sha"`
	Size     int64  `json:"size"`
}
