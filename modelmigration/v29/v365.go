// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm"
)

type pagesSite struct {
	Deleted              bool   `xorm:"NOT NULL DEFAULT false INDEX(deleted_updated)"`
	Enabled              bool   `xorm:"NOT NULL DEFAULT false"`
	SourceType           string `xorm:"VARCHAR(16) NOT NULL DEFAULT '' INDEX(source_type_next)"`
	SourceBranch         string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	SourcePath           string `xorm:"VARCHAR(1024) NOT NULL DEFAULT ''"`
	SourceCommitID       string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	SourceNextCheckUnix  int64  `xorm:"NOT NULL DEFAULT 0 INDEX(source_type_next)"`
	SourceErrorCode      string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	CurrentVersionID     int64  `xorm:"NOT NULL DEFAULT 0"`
	PagesUpdatedUnixNano int64  `xorm:"NOT NULL INDEX(updated_repo) INDEX(deleted_updated)"`
	RepoID               int64  `xorm:"pk INDEX(updated_repo) INDEX(deleted_updated)"`
}

func (pagesSite) TableName() string { return "pages_site" }

type pagesVersion struct {
	ID                int64              `xorm:"pk autoincr"`
	RepoID            int64              `xorm:"NOT NULL UNIQUE(repo_digest) INDEX"`
	TarGzSHA256       string             `xorm:"'tar_gz_sha256' CHAR(64) NOT NULL UNIQUE(repo_digest)"`
	OriginType        string             `xorm:"VARCHAR(16) NOT NULL"`
	OriginActionRunID int64              `xorm:"NOT NULL DEFAULT 0"`
	CreatedUnix       timeutil.TimeStamp `xorm:"created NOT NULL"`
}

func (pagesVersion) TableName() string { return "pages_version" }

type pagesInstance struct {
	ID                    int64              `xorm:"pk autoincr"`
	RootURL               string             `xorm:"VARCHAR(512) NOT NULL DEFAULT ''"`
	RootUpdatedUnixNano   int64              `xorm:"NOT NULL DEFAULT 0"`
	LastCommunicationUnix timeutil.TimeStamp `xorm:"NOT NULL DEFAULT 0"`
	TokenHash             string             `xorm:"VARCHAR(64) NOT NULL UNIQUE"`
}

func (pagesInstance) TableName() string { return "pages_instance" }

type pagesDomain struct {
	ID                     int64  `xorm:"pk autoincr"`
	RepoID                 int64  `xorm:"NOT NULL INDEX(repo_state) INDEX(state_repo)"`
	Host                   string `xorm:"VARCHAR(253) NOT NULL UNIQUE"`
	TXTToken               string `xorm:"VARCHAR(64) NOT NULL"`
	State                  string `xorm:"VARCHAR(16) NOT NULL INDEX(repo_state) INDEX(state_repo)"`
	TXTResult              string `xorm:"VARCHAR(32) NOT NULL"`
	TXTLastSuccessUnixNano int64  `xorm:"NOT NULL DEFAULT 0"`
	TXTLastCheckUnix       int64  `xorm:"NOT NULL DEFAULT 0 INDEX(txt_last_check)"`
	CreatedUnixNano        int64  `xorm:"NOT NULL"`
}

func (pagesDomain) TableName() string { return "pages_domain" }

type pagesInstanceSite struct {
	InstanceID                   int64  `xorm:"pk INDEX(repo_instance)"`
	RepoID                       int64  `xorm:"pk INDEX(repo_instance)"`
	AppliedPagesUpdatedUnixNano  int64  `xorm:"NOT NULL DEFAULT 0"`
	AppliedRootUpdatedUnixNano   int64  `xorm:"NOT NULL DEFAULT 0"`
	AppliedRequireGiteaTXTRecord bool   `xorm:"NOT NULL DEFAULT false"`
	ConfigApplied                bool   `xorm:"NOT NULL DEFAULT false"`
	ActualTarGzSHA256            string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	DefaultPublished             bool   `xorm:"NOT NULL DEFAULT false"`
	CustomPublished              bool   `xorm:"NOT NULL DEFAULT false"`
	ErrorCode                    string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
}

func (pagesInstanceSite) TableName() string { return "pages_instance_site" }

// AddPagesTables creates the Pages tables.
func AddPagesTables(_ context.Context, x base.EngineMigration) error {
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	},
		new(pagesSite),
		new(pagesVersion),
		new(pagesInstance),
		new(pagesDomain),
		new(pagesInstanceSite),
	)
	return err
}
