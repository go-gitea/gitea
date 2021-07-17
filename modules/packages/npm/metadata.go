// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package npm

// Metadata represents the metadata of a NPM package
type Metadata struct {
	Description  string            `json:"description"`
	Author       string            `json:"author"`
	License      string            `json:"license"`
	ProjectURL   string            `json:"project_url"`
	Dependencies map[string]string `json:"dependencies"`
	Readme       string            `json:"readme"`
}
