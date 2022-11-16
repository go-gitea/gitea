// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pypi

// Metadata represents the metadata of a PyPI package
type Metadata struct {
	Author          string `json:"author,omitempty"`
	Description     string `json:"description,omitempty"`
	LongDescription string `json:"long_description,omitempty"`
	Summary         string `json:"summary,omitempty"`
	ProjectURL      string `json:"project_url,omitempty"`
	License         string `json:"license,omitempty"`
	RequiresPython  string `json:"requires_python,omitempty"`
}
