// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"testing"
	"time"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToCommitMeta(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	headRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	sha1 := git.Sha1ObjectFormat
	signature := &git.Signature{Name: "Test Signature", Email: "test@email.com", When: time.Unix(0, 0)}
	tag := &git.Tag{
		Name:          "Test Tag",
		ID:            sha1.EmptyObjectID(),
		Object:        sha1.EmptyObjectID(),
		Type:          "Test Type",
		Tagger:        signature,
		CommitMessage: git.CommitMessage{MessageRaw: "Test Message"},
	}

	commitMeta := ToCommitMeta(headRepo, tag)

	assert.NotNil(t, commitMeta)
	assert.Equal(t, &api.CommitMeta{
		SHA:     sha1.EmptyObjectID().String(),
		URL:     headRepo.APIURL() + "/git/commits/" + sha1.EmptyObjectID().String(),
		Created: time.Unix(0, 0),
	}, commitMeta)
}

func TestToCommitTree(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	signature := &git.Signature{Name: "Test Signature", Email: "test@email.com", When: time.Unix(0, 0)}
	commit := &git.Commit{
		ID:        git.MustIDFromString("65f1bf27bc3bf70f64657658635e66094edbcb4d"),
		TreeID:    git.MustIDFromString("2a2f1d4670728a2e10049e345bd7a276468beab6"),
		Author:    signature,
		Committer: signature,
	}

	apiCommit, err := ToCommit(t.Context(), repo, nil, commit, nil, ToCommitOptions{})
	require.NoError(t, err)
	assert.Equal(t, "2a2f1d4670728a2e10049e345bd7a276468beab6", apiCommit.RepoCommit.Tree.SHA)
	assert.Equal(t, repo.APIURL()+"/git/trees/2a2f1d4670728a2e10049e345bd7a276468beab6", apiCommit.RepoCommit.Tree.URL)
	assert.Equal(t, "65f1bf27bc3bf70f64657658635e66094edbcb4d", apiCommit.SHA)
}
