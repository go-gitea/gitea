// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

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
