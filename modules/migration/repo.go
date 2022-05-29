// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migration

// Repository defines a standard repository information
type Repository struct {
	Name          string
	Owner         string
	IsPrivate     bool `yaml:"is_private"`
	IsMirror      bool `yaml:"is_mirror"`
	Description   string
	CloneURL      string `yaml:"clone_url"`
	OriginalURL   string `yaml:"original_url"`
	DefaultBranch string
}
