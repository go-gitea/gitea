// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

// GlobalUISettings represent the global ui settings of a gitea instance witch is exposed by API
type GlobalUISettings struct {
	AllowedReactions []string `json:"allowed_reactions"`
}

// GlobalRepoSettings represent the global repository settings of a gitea instance witch is exposed by API
type GlobalRepoSettings struct {
	MirrorsDisabled bool `json:"mirrors_disabled"`
	HTTPGitDisabled bool `json:"http_git_disabled"`
}

// GlobalAPISettings contains global api settings exposed by it
type GlobalAPISettings struct {
	MaxResponseItems       int   `json:"max_response_items"`
	DefaultPagingNum       int   `json:"default_paging_num"`
	DefaultGitTreesPerPage int   `json:"default_git_trees_per_page"`
	DefaultMaxBlobSize     int64 `json:"default_max_blob_size"`
}

// GlobalAttachmentSettings contains global Attachment settings exposed by API
type GlobalAttachmentSettings struct {
	Enabled      bool   `json:"enabled"`
	AllowedTypes string `json:"allowed_types"`
	MaxSize      int64  `json:"max_size"`
	MaxFiles     int    `json:"max_files"`
}

// GetGlobalUISettings get global ui settings witch are exposed by API
func (c *Client) GetGlobalUISettings() (settings *GlobalUISettings, err error) {
	if err := c.CheckServerVersionConstraint(">=1.13.0"); err != nil {
		return nil, err
	}
	conf := new(GlobalUISettings)
	return conf, c.getParsedResponse("GET", "/settings/ui", jsonHeader, nil, &conf)
}

// GetGlobalRepoSettings get global repository settings witch are exposed by API
func (c *Client) GetGlobalRepoSettings() (settings *GlobalRepoSettings, err error) {
	if err := c.CheckServerVersionConstraint(">=1.13.0"); err != nil {
		return nil, err
	}
	conf := new(GlobalRepoSettings)
	return conf, c.getParsedResponse("GET", "/settings/repository", jsonHeader, nil, &conf)
}

// GetGlobalAPISettings get global api settings witch are exposed by it
func (c *Client) GetGlobalAPISettings() (settings *GlobalAPISettings, err error) {
	if err := c.CheckServerVersionConstraint(">=1.13.0"); err != nil {
		return nil, err
	}
	conf := new(GlobalAPISettings)
	return conf, c.getParsedResponse("GET", "/settings/api", jsonHeader, nil, &conf)
}

// GetGlobalAttachmentSettings get global repository settings witch are exposed by API
func (c *Client) GetGlobalAttachmentSettings() (settings *GlobalAttachmentSettings, err error) {
	if err := c.CheckServerVersionConstraint(">=1.13.0"); err != nil {
		return nil, err
	}
	conf := new(GlobalAttachmentSettings)
	return conf, c.getParsedResponse("GET", "/settings/attachment", jsonHeader, nil, &conf)
}
