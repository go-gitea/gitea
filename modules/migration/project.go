// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

type Project struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Type        string `json:"type"` // "individual", "repository", or "organization"
}
