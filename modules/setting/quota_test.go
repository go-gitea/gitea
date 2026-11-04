// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"math"
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuotaSettings(t *testing.T) {
	defer test.MockVariableValue(&Quota)()
	for _, tc := range []struct {
		value string
		size  int64
		valid bool
	}{
		{"-1", -1, true},
		{"0", 0, true},
		{" 1 KiB ", 1024, true},
		{"1.5 MiB", 1536 * 1024, true},
		{"9223372036854775807", math.MaxInt64, true},
		{"-2", 0, false},
		{"9223372036854775808", 0, false},
		{"nonsense", 0, false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			size, err := ParseQuotaLimit(tc.value)
			if !tc.valid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.size, size)
		})
	}
	cfg, err := NewConfigProviderFromData("[quota]\nDEFAULT_USER_LIMIT=2 GiB\nDEFAULT_REPO_GIT_LIMIT=0\n")
	require.NoError(t, err)
	require.NoError(t, loadQuotaFrom(cfg))
	assert.EqualValues(t, 0, Quota.DefaultRepoGitLimit)
	assert.EqualValues(t, -1, Quota.DefaultRepoLFSLimit)
	assert.EqualValues(t, 2*1024*1024*1024, Quota.DefaultUserLimit)
	assert.EqualValues(t, -1, Quota.DefaultOrgLimit)
}
