// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// GeneralRepoSettings contains global repository settings exposed by API
type GeneralRepoSettings struct {
	// MirrorsDisabled indicates if repository mirroring is disabled
	MirrorsDisabled bool `json:"mirrors_disabled"`
	// HTTPGitDisabled indicates if HTTP Git operations are disabled
	HTTPGitDisabled bool `json:"http_git_disabled"`
	// MigrationsDisabled indicates if repository migrations are disabled
	MigrationsDisabled bool `json:"migrations_disabled"`
	// StarsDisabled indicates if repository starring is disabled
	StarsDisabled bool `json:"stars_disabled"`
	// TimeTrackingDisabled indicates if time tracking is disabled
	TimeTrackingDisabled bool `json:"time_tracking_disabled"`
	// LFSDisabled indicates if Git LFS support is disabled
	LFSDisabled bool `json:"lfs_disabled"`
}

// GeneralUISettings contains global ui settings exposed by API
type GeneralUISettings struct {
	// DefaultTheme is the default UI theme
	DefaultTheme string `json:"default_theme"`
	// AllowedReactions contains the list of allowed emoji reactions
	AllowedReactions []string `json:"allowed_reactions"`
	// CustomEmojis contains the list of custom emojis
	CustomEmojis []string `json:"custom_emojis"`
}

// GeneralAPISettings contains global api settings exposed by it
type GeneralAPISettings struct {
	// MaxResponseItems is the maximum number of items returned in API responses
	MaxResponseItems int `json:"max_response_items"`
	// DefaultPagingNum is the default number of items per page
	DefaultPagingNum int `json:"default_paging_num"`
	// DefaultGitTreesPerPage is the default number of Git tree items per page
	DefaultGitTreesPerPage int `json:"default_git_trees_per_page"`
	// DefaultMaxBlobSize is the default maximum blob size for API responses
	DefaultMaxBlobSize int64 `json:"default_max_blob_size"`
	// DefaultMaxResponseSize is the default maximum response size
	DefaultMaxResponseSize int64 `json:"default_max_response_size"`
}

// GeneralAttachmentSettings contains global Attachment settings exposed by API
type GeneralAttachmentSettings struct {
	// Enabled indicates if file attachments are enabled
	Enabled bool `json:"enabled"`
	// AllowedTypes contains the allowed file types for attachments
	AllowedTypes string `json:"allowed_types"`
	// MaxSize is the maximum size for individual attachments
	MaxSize int64 `json:"max_size"`
	// MaxFiles is the maximum number of files per attachment
	MaxFiles int `json:"max_files"`
}
