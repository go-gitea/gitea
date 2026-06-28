// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"testing"

	"gitea.dev/models/migrations/migrationtest"

	"github.com/stretchr/testify/require"
)

type packageBlobUploadBeforeV342 struct {
	ID            string `xorm:"pk"`
	BytesReceived int64  `xorm:"NOT NULL DEFAULT 0"`
}

func (packageBlobUploadBeforeV342) TableName() string {
	return "package_blob_upload"
}

type packageBlobUploadAfterV342 struct {
	ID            string `xorm:"pk"`
	CreatorID     int64  `xorm:"INDEX NOT NULL DEFAULT 0"`
	BytesReceived int64  `xorm:"NOT NULL DEFAULT 0"`
}

func (packageBlobUploadAfterV342) TableName() string {
	return "package_blob_upload"
}

func Test_AddCreatorIDToPackageBlobUpload(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(packageBlobUploadBeforeV342))
	defer deferable()

	_, err := x.Insert(&packageBlobUploadBeforeV342{
		ID:            "test-upload",
		BytesReceived: 12,
	})
	require.NoError(t, err)

	require.NoError(t, AddCreatorIDToPackageBlobUpload(x))

	var pbu packageBlobUploadAfterV342
	has, err := x.ID("test-upload").Get(&pbu)
	require.NoError(t, err)
	require.True(t, has)
	require.EqualValues(t, 0, pbu.CreatorID)
	require.EqualValues(t, 12, pbu.BytesReceived)
}
