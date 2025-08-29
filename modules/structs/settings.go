// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// GeneralRepoSettings contains global repository settings exposed by API
type GeneralRepoSettings struct {
	MirrorsDisabled      bool `json:"mirrors_disabled"`
	HTTPGitDisabled      bool `json:"http_git_disabled"`
	MigrationsDisabled   bool `json:"migrations_disabled"`
	StarsDisabled        bool `json:"stars_disabled"`
	TimeTrackingDisabled bool `json:"time_tracking_disabled"`
	LFSDisabled          bool `json:"lfs_disabled"`
}

// GeneralUISettings contains global ui settings exposed by API
type GeneralUISettings struct {
	DefaultTheme     string   `json:"default_theme"`
	AllowedReactions []string `json:"allowed_reactions"`
	CustomEmojis     []string `json:"custom_emojis"`
}

// GeneralAPISettings contains global api settings exposed by it
type GeneralAPISettings struct {
	MaxResponseItems       int   `json:"max_response_items"`
	DefaultPagingNum       int   `json:"default_paging_num"`
	DefaultGitTreesPerPage int   `json:"default_git_trees_per_page"`
	DefaultMaxBlobSize     int64 `json:"default_max_blob_size"`
	DefaultMaxResponseSize int64 `json:"default_max_response_size"`
}

// GeneralAttachmentSettings contains global Attachment settings exposed by API
type GeneralAttachmentSettings struct {
	Enabled      bool   `json:"enabled"`
	AllowedTypes string `json:"allowed_types"`
	MaxSize      int64  `json:"max_size"`
	MaxFiles     int    `json:"max_files"`
}
