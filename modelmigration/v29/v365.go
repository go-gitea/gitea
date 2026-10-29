// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

type pagesSite struct {
	RepoID               int64  `xorm:"pk"`
	Deleted              bool   `xorm:"NOT NULL DEFAULT false INDEX(deleted_updated)"`
	Enabled              bool   `xorm:"NOT NULL DEFAULT false"`
	SourceType           string `xorm:"VARCHAR(16) NOT NULL DEFAULT '' INDEX(source_type_next)"`
	SourceBranch         string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	SourcePath           string `xorm:"VARCHAR(1024) NOT NULL DEFAULT ''"`
	SourceCommitID       string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	SourceNextCheckUnix  int64  `xorm:"NOT NULL DEFAULT 0 INDEX(source_type_next)"`
	SourceErrorCode      string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	TarGzSHA256          string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	PagesUpdatedUnixNano int64  `xorm:"NOT NULL INDEX(updated_repo) INDEX(deleted_updated)"`
}

func (pagesSite) TableName() string { return "pages_site" }

type pagesInstance struct {
	ID                  int64  `xorm:"pk autoincr"`
	RootURL             string `xorm:"VARCHAR(512) NOT NULL DEFAULT ''"`
	RootUpdatedUnixNano int64  `xorm:"NOT NULL DEFAULT 0"`
	TokenHash           string `xorm:"VARCHAR(64) NOT NULL UNIQUE"`
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
	TXTNextCheckUnix       int64  `xorm:"NOT NULL DEFAULT 0 INDEX(txt_next_check)"`
	CreatedUnixNano        int64  `xorm:"NOT NULL"`
}

func (pagesDomain) TableName() string { return "pages_domain" }

type pagesInstanceSite struct {
	InstanceID                      int64  `xorm:"pk INDEX(repo_instance)"`
	RepoID                          int64  `xorm:"pk INDEX(repo_instance)"`
	AppliedPagesUpdatedUnixNano     int64  `xorm:"NOT NULL DEFAULT 0"`
	AppliedRootUpdatedUnixNano      int64  `xorm:"NOT NULL DEFAULT 0"`
	AppliedTXTPolicyUpdatedUnixNano int64  `xorm:"NOT NULL DEFAULT 0"`
	ConfigApplied                   bool   `xorm:"NOT NULL DEFAULT false"`
	ActualTarGzSHA256               string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	DefaultPublished                bool   `xorm:"NOT NULL DEFAULT false"`
	CustomPublished                 bool   `xorm:"NOT NULL DEFAULT false"`
	ErrorCode                       string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
}

func (pagesInstanceSite) TableName() string { return "pages_instance_site" }

// AddPagesTables creates the four Pages configuration and result tables.
func AddPagesTables(_ context.Context, x base.EngineMigration) error {
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	},
		new(pagesSite),
		new(pagesInstance),
		new(pagesDomain),
		new(pagesInstanceSite),
	)
	return err
}
