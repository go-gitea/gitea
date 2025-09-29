// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func Test_AddIndexForReleaseSha1(t *testing.T) {
	type Release struct {
		ID               int64  `xorm:"pk autoincr"`
		RepoID           int64  `xorm:"INDEX UNIQUE(n)"`
		PublisherID      int64  `xorm:"INDEX"`
		TagName          string `xorm:"INDEX UNIQUE(n)"`
		OriginalAuthor   string
		OriginalAuthorID int64 `xorm:"index"`
		LowerTagName     string
		Target           string
		Title            string
		Sha1             string `xorm:"VARCHAR(64)"`
		NumCommits       int64
		Note             string             `xorm:"TEXT"`
		IsDraft          bool               `xorm:"NOT NULL DEFAULT false"`
		IsPrerelease     bool               `xorm:"NOT NULL DEFAULT false"`
		IsTag            bool               `xorm:"NOT NULL DEFAULT false"` // will be true only if the record is a tag and has no related releases
		CreatedUnix      timeutil.TimeStamp `xorm:"INDEX"`
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(Release))
	defer deferable()

	assert.NoError(t, AddIndexForReleaseSha1(x))
}
