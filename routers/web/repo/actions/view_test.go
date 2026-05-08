// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/translation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToViewModel(t *testing.T) {
	task := &actions_model.ActionTask{
		Status: actions_model.StatusSuccess,
		Steps: []*actions_model.ActionTaskStep{
			{Name: "Run step-name", Index: 0, Status: actions_model.StatusSuccess, LogLength: 1, Started: timeutil.TimeStamp(1), Stopped: timeutil.TimeStamp(5)},
		},
		Stopped: timeutil.TimeStamp(20),
	}

	viewJobSteps, _, err := convertToViewModel(t.Context(), translation.MockLocale{}, nil, task)
	require.NoError(t, err)

	expectedViewJobs := []*ViewJobStep{
		{
			Summary:  "Set up job",
			Duration: "0s",
			Status:   "success",
		},
		{
			Summary:  "Run step-name",
			Duration: "4s",
			Status:   "success",
		},
		{
			Summary:  "Complete job",
			Duration: "15s",
			Status:   "success",
		},
	}
	assert.Equal(t, expectedViewJobs, viewJobSteps)
}

func resetArtifactPreviewV4ZipListCacheForTest() {
	artifactPreviewV4ZipListCache.mu.Lock()
	defer artifactPreviewV4ZipListCache.mu.Unlock()
	artifactPreviewV4ZipListCache.entries = map[string]artifactPreviewV4ZipListCacheEntry{}
	artifactPreviewV4ZipListCache.order = nil
}

func TestArtifactPreviewV4ZipListCacheSetGet(t *testing.T) {
	resetArtifactPreviewV4ZipListCacheForTest()

	artifact := &actions_model.ActionArtifact{
		ID:          1,
		UpdatedUnix: timeutil.TimeStamp(2),
		StoragePath: "artifact/path.zip",
	}
	paths := []string{"index.html", "logs/output.txt"}
	setArtifactPreviewV4ZipListCache(artifact, paths)

	paths[0] = "changed"
	got, ok := getArtifactPreviewV4ZipListFromCache(artifact)
	require.True(t, ok)
	assert.Equal(t, []string{"index.html", "logs/output.txt"}, got)

	got[0] = "changed-again"
	got2, ok := getArtifactPreviewV4ZipListFromCache(artifact)
	require.True(t, ok)
	assert.Equal(t, []string{"index.html", "logs/output.txt"}, got2)
}

func TestArtifactPreviewV4ZipListCacheExpires(t *testing.T) {
	resetArtifactPreviewV4ZipListCacheForTest()

	artifact := &actions_model.ActionArtifact{
		ID:          2,
		UpdatedUnix: timeutil.TimeStamp(3),
		StoragePath: "artifact/expired.zip",
	}
	key := artifactPreviewV4ZipListCacheKey(artifact)

	artifactPreviewV4ZipListCache.mu.Lock()
	artifactPreviewV4ZipListCache.entries[key] = artifactPreviewV4ZipListCacheEntry{
		paths:     []string{"expired.txt"},
		expiresAt: time.Now().Add(-time.Second),
	}
	artifactPreviewV4ZipListCache.order = []string{key}
	artifactPreviewV4ZipListCache.mu.Unlock()

	_, ok := getArtifactPreviewV4ZipListFromCache(artifact)
	require.False(t, ok)

	artifactPreviewV4ZipListCache.mu.Lock()
	_, exists := artifactPreviewV4ZipListCache.entries[key]
	order := append([]string(nil), artifactPreviewV4ZipListCache.order...)
	artifactPreviewV4ZipListCache.mu.Unlock()
	assert.False(t, exists)
	assert.NotContains(t, order, key)
}

func TestArtifactPreviewV4ZipListCacheEvictsOldest(t *testing.T) {
	resetArtifactPreviewV4ZipListCacheForTest()

	for i := range artifactPreviewV4ZipListCacheMaxEntries + 1 {
		artifact := &actions_model.ActionArtifact{
			ID:          int64(i + 1),
			UpdatedUnix: timeutil.TimeStamp(i + 1),
			StoragePath: "artifact/cache-entry.zip",
		}
		setArtifactPreviewV4ZipListCache(artifact, []string{"file.txt"})
	}

	oldest := &actions_model.ActionArtifact{
		ID:          1,
		UpdatedUnix: timeutil.TimeStamp(1),
		StoragePath: "artifact/cache-entry.zip",
	}
	_, ok := getArtifactPreviewV4ZipListFromCache(oldest)
	assert.False(t, ok)

	newest := &actions_model.ActionArtifact{
		ID:          artifactPreviewV4ZipListCacheMaxEntries + 1,
		UpdatedUnix: timeutil.TimeStamp(artifactPreviewV4ZipListCacheMaxEntries + 1),
		StoragePath: "artifact/cache-entry.zip",
	}
	_, ok = getArtifactPreviewV4ZipListFromCache(newest)
	assert.True(t, ok)
}
