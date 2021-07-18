// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pypi

// Metadata represents the metadata of a PyPI package
type Metadata struct {
	Author         string `json:"author"`
	Description    string `json:"description"`
	Summary        string `json:"summary"`
	ProjectURL     string `json:"project_url"`
	License        string `json:"license"`
	RequiresPython string `json:"requires_python"`
}
