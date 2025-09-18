// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Note contains information related to a git note
type Note struct {
	// The content message of the git note
	Message string `json:"message"`
	// The commit that this note is attached to
	Commit *Commit `json:"commit"`
}
