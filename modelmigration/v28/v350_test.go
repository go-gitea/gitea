// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/require"
)

func TestAddPublishedUnixToRelease(t *testing.T) {
	type Release struct {
		ID          int64 `xorm:"pk autoincr"`
		IsDraft     bool  `xorm:"NOT NULL DEFAULT false"`
		IsTag       bool  `xorm:"NOT NULL DEFAULT false"`
		CreatedUnix int64 `xorm:"INDEX"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(Release))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(
		&Release{CreatedUnix: 1000000},
		&Release{IsDraft: true, CreatedUnix: 2000000},
		&Release{IsTag: true, CreatedUnix: 3000000},
	)
	require.NoError(t, err)

	require.NoError(t, AddPublishedUnixToRelease(t.Context(), x))

	var got []struct{ PublishedUnix int64 }
	require.NoError(t, x.Table("release").OrderBy("id").Find(&got))
	require.Equal(t, []int64{1000000, 0, 3000000}, []int64{got[0].PublishedUnix, got[1].PublishedUnix, got[2].PublishedUnix},
		"everything but drafts is backfilled")
}
