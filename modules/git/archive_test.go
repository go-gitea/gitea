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

	for _, tc := range []struct{ repo, commit, version string }{
		{"repo1_bare", "ce064814f4a0d337b333e646ece456cd39fab612", "# v2 git bundle"},
		{"repo1_bare_sha256", "9433b2a62b964c17a4485ae180f45f595d3e69d31b786087775e28c6b6399df0", "# v3 git bundle"},
	} {
		buf := &bytes.Buffer{}
		require.NoError(t, CreateBundle(t.Context(), mockRepository(tc.repo), tc.commit, buf))

		header, _, ok := strings.Cut(buf.String(), "\n\n")
		require.True(t, ok)
		assert.Equal(t, tc.version, strings.Split(header, "\n")[0])
		// without a refs/heads/* ref and a HEAD, a clone of the bundle has no branch and no checkout
		assert.Contains(t, header, tc.commit+" refs/heads/bundle")
		assert.Contains(t, header, tc.commit+" HEAD")
	}
}
