// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"strings"
	"testing"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBundle(t *testing.T) {
	setting.AppDataPath = t.TempDir()

	buf := &bytes.Buffer{}
	require.NoError(t, CreateBundle(t.Context(), mockRepository("repo1_bare"), "ce064814f4a0d337b333e646ece456cd39fab612", buf))

	header, _, ok := strings.Cut(buf.String(), "\n\n")
	require.True(t, ok)
	assert.Equal(t, "# v2 git bundle", strings.Split(header, "\n")[0])
	// a bundle whose only ref sits outside refs/heads/* clones into an empty repository
	assert.Contains(t, header, "ce064814f4a0d337b333e646ece456cd39fab612 refs/heads/bundle")
}
