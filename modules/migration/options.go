// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import "gitea.dev/modules/structs"

// MigrateOptions defines the way a repository gets migrated
// this is for internal usage by migrations module and func who interact with it
type MigrateOptions struct {
	// required: true
	CloneAddr             string `json:"clone_addr" binding:"Required"`
	CloneAddrEncrypted    string `json:"clone_addr_encrypted,omitempty"`
	AuthUsername          string `json:"auth_username"`
	AuthPassword          string `json:"-"`
	AuthPasswordEncrypted string `json:"auth_password_encrypted,omitempty"`
	AuthToken             string `json:"-"`
	AuthTokenEncrypted    string `json:"auth_token_encrypted,omitempty"`
	// required: true
	UID int `json:"uid" binding:"Required"`
	// required: true
	RepoName        string `json:"repo_name" binding:"Required"`
	Mirror          bool   `json:"mirror"`
	LFS             bool   `json:"lfs"`
	LFSEndpoint     string `json:"lfs_endpoint"`
	Private         bool   `json:"private"`
	Description     string `json:"description"`
	OriginalURL     string
	GitServiceType  structs.GitServiceType
	Wiki            bool
	Issues          bool
	Milestones      bool
	Labels          bool
	Releases        bool
	Comments        bool
	PullRequests    bool
	ReleaseAssets   bool
	MigrateToRepoID int64
	MirrorInterval  string `json:"mirror_interval"`

	// mirror-only: enable read-only metadata sync of issues/pull requests
	// on each mirror update.
	SyncIssues       bool
	SyncPullRequests bool

	// SkipReactions omits per-issue and per-comment reaction fetches. Reactions
	// are an N+1 call storm (one request per issue and per comment) that
	// dominates the API budget and, on a single transient failure, aborts the
	// whole pass. The mirror metadata reflection sets this so large repositories
	// complete in a sane window; reactions are the lowest-value metadata for a
	// read-only mirror.
	SkipReactions bool

	AWSAccessKeyID     string
	AWSSecretAccessKey string `json:",omitempty"`

	AWSSecretAccessKeyEncrypted string `json:"aws_secret_access_key_encrypted,omitempty"`
}
