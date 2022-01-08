// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

// GlobalUISettings represent the global ui settings of a gitea instance witch is exposed by API
type GlobalUISettings struct {
	DefaultTheme     string   `json:"default_theme"`
	AllowedReactions []string `json:"allowed_reactions"`
	CustomEmojis     []string `json:"custom_emojis"`
}

// GlobalRepoSettings represent the global repository settings of a gitea instance witch is exposed by API
type GlobalRepoSettings struct {
	MirrorsDisabled      bool `json:"mirrors_disabled"`
	HTTPGitDisabled      bool `json:"http_git_disabled"`
	MigrationsDisabled   bool `json:"migrations_disabled"`
	StarsDisabled        bool `json:"stars_disabled"`
	TimeTrackingDisabled bool `json:"time_tracking_disabled"`
	LFSDisabled          bool `json:"lfs_disabled"`
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
func (c *Client) GetGlobalUISettings() (*GlobalUISettings, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_13_0); err != nil {
		return nil, nil, err
	}
	conf := new(GlobalUISettings)
	resp, err := c.getParsedResponse("GET", "/settings/ui", jsonHeader, nil, &conf)
	return conf, resp, err
}

// GetGlobalRepoSettings get global repository settings witch are exposed by API
func (c *Client) GetGlobalRepoSettings() (*GlobalRepoSettings, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_13_0); err != nil {
		return nil, nil, err
	}
	conf := new(GlobalRepoSettings)
	resp, err := c.getParsedResponse("GET", "/settings/repository", jsonHeader, nil, &conf)
	return conf, resp, err
}

// GetGlobalAPISettings get global api settings witch are exposed by it
func (c *Client) GetGlobalAPISettings() (*GlobalAPISettings, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_13_0); err != nil {
		return nil, nil, err
	}
	conf := new(GlobalAPISettings)
	resp, err := c.getParsedResponse("GET", "/settings/api", jsonHeader, nil, &conf)
	return conf, resp, err
}

// GetGlobalAttachmentSettings get global repository settings witch are exposed by API
func (c *Client) GetGlobalAttachmentSettings() (*GlobalAttachmentSettings, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_13_0); err != nil {
		return nil, nil, err
	}
	conf := new(GlobalAttachmentSettings)
	resp, err := c.getParsedResponse("GET", "/settings/attachment", jsonHeader, nil, &conf)
	return conf, resp, err
}
