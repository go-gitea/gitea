// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCopiedContentRequest(t *testing.T) {
	parent := strings.Repeat("b", 40)
	valid := &FinalizeCopiedContentRequest{
		RequestID: uuid.NewString(),
		UserID:    2,
		Destination: CopiedContentDestination{
			RepositoryID: 3,
			Branch:       "master",
			Commit:       strings.Repeat("c", 40),
			Parent:       &parent,
		},
		Files: []CopiedContentFileMapping{{
			SourceRepositoryID: 1,
			SourceCommit:       strings.Repeat("a", 40),
			SourcePath:         "src/a.txt",
			DestinationPath:    "dst/a.txt",
			Representation:     CopiedContentRepresentationGit,
		}},
	}
	require.NoError(t, validateCopiedContentRequest(valid))

	t.Run("TooManyFiles", func(t *testing.T) {
		req := *valid
		req.Files = make([]CopiedContentFileMapping, CopiedContentMaxFiles+1)
		for i := range req.Files {
			req.Files[i] = CopiedContentFileMapping{
				SourceRepositoryID: 1,
				SourceCommit:       strings.Repeat("a", 40),
				SourcePath:         "a.txt",
				DestinationPath:    "f" + strings.Repeat("x", i) + ".txt",
				Representation:     CopiedContentRepresentationGit,
			}
		}
		err := validateCopiedContentRequest(&req)
		var copied *CopiedContentError
		require.ErrorAs(t, err, &copied)
		assert.Equal(t, "too_large", copied.Code)
	})

	t.Run("DuplicateDestination", func(t *testing.T) {
		req := *valid
		req.Files = []CopiedContentFileMapping{valid.Files[0], valid.Files[0]}
		err := validateCopiedContentRequest(&req)
		var copied *CopiedContentError
		require.ErrorAs(t, err, &copied)
		assert.Equal(t, "invalid_request", copied.Code)
	})

	t.Run("AncestorPath", func(t *testing.T) {
		req := *valid
		req.Files = []CopiedContentFileMapping{
			{SourceRepositoryID: 1, SourceCommit: strings.Repeat("a", 40), SourcePath: "a.txt", DestinationPath: "dir", Representation: CopiedContentRepresentationGit},
			{SourceRepositoryID: 1, SourceCommit: strings.Repeat("a", 40), SourcePath: "b.txt", DestinationPath: "dir/b.txt", Representation: CopiedContentRepresentationGit},
		}
		err := validateCopiedContentRequest(&req)
		var copied *CopiedContentError
		require.ErrorAs(t, err, &copied)
		assert.Equal(t, "invalid_request", copied.Code)
	})

	t.Run("InvalidPath", func(t *testing.T) {
		req := *valid
		req.Files[0].DestinationPath = "../escape.txt"
		err := validateCopiedContentRequest(&req)
		var copied *CopiedContentError
		require.ErrorAs(t, err, &copied)
		assert.Equal(t, "invalid_request", copied.Code)
	})

	t.Run("InvalidUUID", func(t *testing.T) {
		req := *valid
		req.RequestID = "not-a-uuid"
		err := validateCopiedContentRequest(&req)
		var copied *CopiedContentError
		require.ErrorAs(t, err, &copied)
		assert.Equal(t, "invalid_request", copied.Code)
	})
}

func TestCopiedRepoPathAndBranch(t *testing.T) {
	assert.True(t, isCopiedRepoPath("dir/file.txt"))
	assert.False(t, isCopiedRepoPath("/abs"))
	assert.False(t, isCopiedRepoPath("foo/../bar"))
	assert.False(t, isCopiedRepoPath("foo\\bar"))
	assert.True(t, isCopiedBranchName("master"))
	assert.True(t, isCopiedBranchName("feature/x"))
	assert.False(t, isCopiedBranchName("refs/heads/master"))
	assert.False(t, isCopiedBranchName(""))
}
