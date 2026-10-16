// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror_test

import (
	"testing"
	"time"

	mirror_service "gitea.dev/services/mirror"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupNameAdvancesPastPrunedBaseAndClockRollback(t *testing.T) {
	prefix := "refs/heads/mirror-backup/branch/main-"
	now := time.Date(2026, time.October, 16, 12, 0, 0, 0, time.UTC)
	existing := []string{prefix + "20261016-120000-2"}

	assert.Equal(t, prefix+"20261016-120000-3", mirror_service.NextBackupRefNameForTest(prefix, existing, now))
	assert.Equal(t, prefix+"20261016-120001", mirror_service.NextBackupRefNameForTest(prefix, existing, now.Add(time.Second)))
	assert.Equal(t, prefix+"20261016-120000-3", mirror_service.NextBackupRefNameForTest(prefix, existing, now.Add(-time.Second)))
}

func TestBackupOrderUsesNumericSequenceAndReadsLegacyNames(t *testing.T) {
	prefix := "refs/tags/mirror-backup/tag/v1-"
	names := []string{
		prefix + "20261016-120000-2",
		prefix + "20261016-120000-10",
		prefix + "20261016-120000-9",
		prefix + "20261016-115959",
	}
	assert.Equal(t, []string{
		prefix + "20261016-120000-10",
		prefix + "20261016-120000-9",
		prefix + "20261016-120000-2",
		prefix + "20261016-115959",
	}, mirror_service.SortBackupRefNamesForTest(prefix, names))

	refType, shortName, sequence, ok := mirror_service.ParseBackupRefForTest(prefix + "20261016-115959")
	require.True(t, ok)
	assert.Equal(t, "tag", refType)
	assert.Equal(t, "v1", shortName)
	assert.Equal(t, 1, sequence)
}

func TestMirrorRefBlockersRespectPruneInBothDirections(t *testing.T) {
	for _, tc := range []struct {
		local  string
		remote string
	}{
		{"refs/heads/release", "refs/heads/release/v1"},
		{"refs/heads/release/v1", "refs/heads/release"},
	} {
		blocking, err := mirror_service.FindBlockingMirrorRefsForTest([]string{tc.local}, []string{tc.remote}, true)
		require.NoError(t, err)
		assert.Equal(t, []string{tc.local}, blocking)

		_, err = mirror_service.FindBlockingMirrorRefsForTest([]string{tc.local}, []string{tc.remote}, false)
		require.ErrorContains(t, err, tc.local)
	}
}

func TestMirrorRefNamespaceRejectsConflictingRemoteRefs(t *testing.T) {
	for _, name := range []string{
		"refs/mirror-fetch",
		"refs/mirror-fetch/heads/main",
		"refs/heads/mirror-backup",
		"refs/heads/mirror-backup/branch/main",
		"refs/tags/mirror-backup/tag/v1",
	} {
		require.ErrorContains(t, mirror_service.ValidateRemoteRefNameForTest(name), name)
	}
	assert.NoError(t, mirror_service.ValidateRemoteRefNameForTest("refs/notes/commits"))
}
