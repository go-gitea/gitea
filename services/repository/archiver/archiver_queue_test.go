// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package archiver

import (
	"encoding/json"
	"testing"

	repo_model "gitea.dev/models/repo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveQueueItem_UnmarshalJSON_v125Payload(t *testing.T) {
	raw := []byte(`{"RepoID":42,"Type":1,"CommitID":"deadbeef"}`)
	var item archiveQueueItem
	require.NoError(t, json.Unmarshal(raw, &item))
	assert.Equal(t, int64(42), item.RepoID)
	assert.Equal(t, repo_model.ArchiveType(1), item.Type)
	assert.Equal(t, "deadbeef", item.CommitID)
}

func TestArchiveQueueItem_UnmarshalJSON_v126RepoPayload(t *testing.T) {
	raw := []byte(`{"Repo":{"id":99,"name":"demo"},"Type":2,"CommitID":"cafebabe","Paths":["agents"]}`)
	var item archiveQueueItem
	require.NoError(t, json.Unmarshal(raw, &item))
	assert.Equal(t, int64(99), item.RepoID)
	assert.Equal(t, repo_model.ArchiveType(2), item.Type)
	assert.Equal(t, "cafebabe", item.CommitID)
	assert.Equal(t, []string{"agents"}, item.Paths)
}

func TestArchiveQueueItem_RoundTrip(t *testing.T) {
	orig := &archiveQueueItem{
		RepoID:              7,
		Type:                repo_model.ArchiveZip,
		CommitID:            "abc123",
		Paths:               []string{"agents"},
		ArchiveRefShortName: "main",
	}
	bs, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded archiveQueueItem
	require.NoError(t, json.Unmarshal(bs, &decoded))
	assert.Equal(t, *orig, decoded)
}
