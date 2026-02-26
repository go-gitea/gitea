// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_AddPublishedUnixToRelease(t *testing.T) {
	type Release struct {
		ID          int64 `xorm:"pk autoincr"`
		IsDraft     bool  `xorm:"NOT NULL DEFAULT false"`
		IsTag       bool  `xorm:"NOT NULL DEFAULT false"`
		CreatedUnix int64 `xorm:"INDEX"`
	}

	x, deferrable := base.PrepareTestEnv(t, 0, new(Release))
	defer deferrable()

	_, err := x.Insert(&Release{IsDraft: false, IsTag: false, CreatedUnix: 1000000})
	require.NoError(t, err)
	_, err = x.Insert(&Release{IsDraft: true, IsTag: false, CreatedUnix: 2000000})
	require.NoError(t, err)
	_, err = x.Insert(&Release{IsDraft: false, IsTag: true, CreatedUnix: 3000000})
	require.NoError(t, err)

	require.NoError(t, AddPublishedUnixToRelease(x))

	type ReleasePost struct {
		ID            int64 `xorm:"pk autoincr"`
		CreatedUnix   int64
		PublishedUnix int64
	}

	var releases []ReleasePost
	require.NoError(t, x.Table("release").OrderBy("id").Find(&releases))
	require.Len(t, releases, 3)

	assert.Equal(t, int64(1000000), releases[0].PublishedUnix) // published: backfilled from created_unix
	assert.Equal(t, int64(0), releases[1].PublishedUnix)       // draft: stays 0
	assert.Equal(t, int64(0), releases[2].PublishedUnix)       // tag: stays 0
}
