// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// GeneralRepoSettings contains general repo settings exposed by api
type GeneralRepoSettings struct {
	MirrorsDisabled  bool `json:"mirrors_disabled"`
	HTTPGitDisabled  bool `json:"http_git_disabled"`
	MaxCreationLimit int  `json:"max_creation_limit"`
}
