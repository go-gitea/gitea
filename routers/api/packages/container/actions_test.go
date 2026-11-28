// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTaskUploadBinding(t *testing.T) {
	baseline := signTaskUpload(1, 2, "at-auth", "upload")
	require.Len(t, baseline, 64)
	require.Equal(t, baseline, signTaskUpload(1, 2, "at-auth", "upload"))
	require.NotEqual(t, baseline, signTaskUpload(3, 2, "at-auth", "upload"))
	require.NotEqual(t, baseline, signTaskUpload(1, 3, "at-auth", "upload"))
	require.NotEqual(t, baseline, signTaskUpload(1, 2, "other", "upload"))
	require.NotEqual(t, baseline, signTaskUpload(1, 2, "at-auth", "other"))
}
