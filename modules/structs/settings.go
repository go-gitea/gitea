// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// GeneralRepoSettings contains global repository settings exposed by API
type GeneralRepoSettings struct {
	MirrorsDisabled bool `json:"mirrors_disabled"`
	HTTPGitDisabled bool `json:"http_git_disabled"`
}

// GeneralUISettings contains global ui settings exposed by API
type GeneralUISettings struct {
	AllowedReactions []string `json:"allowed_reactions"`
}
