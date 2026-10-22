// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestKeepLatestAttemptArtifacts(t *testing.T) {
	arts := []*ActionArtifact{
		{ID: 1, RunAttemptID: 1, ArtifactName: "inherited"},
		{ID: 2, RunAttemptID: 1, ArtifactName: "shadowed", ArtifactPath: "a.txt"},
		{ID: 3, RunAttemptID: 1, ArtifactName: "shadowed", ArtifactPath: "b.txt"},
		{ID: 4, RunAttemptID: 2, ArtifactName: "shadowed", ArtifactPath: "c.txt"},
		{ID: 5, RunAttemptID: 2, ArtifactName: "own"},
	}

	// the whole "shadowed" group of attempt 1 is dropped, its multi-file rows must not mix with attempt 2
	var ids []int64
	for _, art := range keepLatestAttemptArtifacts(arts) {
		ids = append(ids, art.ID)
	}
	assert.Equal(t, []int64{1, 4, 5}, ids)
}

func TestArtifactExpiry(t *testing.T) {
	defer timeutil.MockSet(time.Unix(1_000_000_000, 0))()
	now := timeutil.TimeStampNow()
	days := func(n int64) timeutil.TimeStamp { return now.Add(n * timeutil.Day) }
	requested := optional.Some[timeutil.TimeStamp]

	defer test.MockVariableValue(&setting.Actions.ArtifactRetentionDays, 90)()
	assert.Equal(t, days(90), artifactExpiry(optional.None[timeutil.TimeStamp]()))
	assert.Equal(t, days(5), artifactExpiry(requested(days(5))))
	assert.Equal(t, days(1), artifactExpiry(requested(days(-2))), "runner clock behind must not expire the artifact at once")
	assert.Equal(t, days(90), artifactExpiry(requested(days(1000))), "runner must not exceed the instance retention")

	setting.Actions.ArtifactRetentionDays = 0
	assert.Equal(t, artifactKeepForever, artifactExpiry(optional.None[timeutil.TimeStamp]()))
	assert.Equal(t, days(1000), artifactExpiry(requested(days(1000))))
	assert.Equal(t, days(1), artifactExpiry(requested(now)))
}
