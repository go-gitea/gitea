// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Note contains information related to a git note
type Note struct {
	Message string  `json:"message"`
	Commit  *Commit `json:"commit"`
}
