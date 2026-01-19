// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_GetTag(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	clonedPath, err := cloneRepo(t, bareRepo1Path)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	bareRepo1, err := OpenRepository(t.Context(), clonedPath)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	defer bareRepo1.Close()

	// LIGHTWEIGHT TAGS
	lTagCommitID := "6fbd69e9823458e6c4a2fc5c0f6bc022b2f2acd1"
	lTagName := "lightweightTag"

	// Create the lightweight tag
	err = bareRepo1.CreateTag(lTagName, lTagCommitID)
	if err != nil {
		assert.NoError(t, err, "Unable to create the lightweight tag: %s for ID: %s. Error: %v", lTagName, lTagCommitID, err)
		return
	}

	// and try to get the Tag for lightweight tag
	lTag, err := bareRepo1.GetTag(lTagName)
	require.NoError(t, err)
	require.NotNil(t, lTag, "nil lTag: %s", lTagName)

	assert.Equal(t, lTagName, lTag.Name)
	assert.Equal(t, lTagCommitID, lTag.ID.String())
	assert.Equal(t, lTagCommitID, lTag.Object.String())
	assert.Equal(t, "commit", lTag.Type)

	// ANNOTATED TAGS
	aTagCommitID := "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0"
	aTagName := "annotatedTag"
	aTagMessage := "my annotated message \n - test two line"

	// Create the annotated tag
	err = bareRepo1.CreateAnnotatedTag(aTagName, aTagMessage, aTagCommitID)
	if err != nil {
		assert.NoError(t, err, "Unable to create the annotated tag: %s for ID: %s. Error: %v", aTagName, aTagCommitID, err)
		return
	}

	// Now try to get the tag for the annotated Tag
	aTagID, err := bareRepo1.GetTagID(aTagName)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	aTag, err := bareRepo1.GetTag(aTagName)
	require.NoError(t, err)
	require.NotNil(t, aTag, "nil aTag: %s", aTagName)

	assert.Equal(t, aTagName, aTag.Name)
	assert.Equal(t, aTagID, aTag.ID.String())
	assert.NotEqual(t, aTagID, aTag.Object.String())
	assert.Equal(t, aTagCommitID, aTag.Object.String())
	assert.Equal(t, "tag", aTag.Type)

	// RELEASE TAGS

	rTagCommitID := "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0"
	rTagName := "release/" + lTagName

	err = bareRepo1.CreateTag(rTagName, rTagCommitID)
	if err != nil {
		assert.NoError(t, err, "Unable to create the  tag: %s for ID: %s. Error: %v", rTagName, rTagCommitID, err)
		return
	}

	rTagID, err := bareRepo1.GetTagID(rTagName)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	assert.Equal(t, rTagCommitID, rTagID)

	oTagID, err := bareRepo1.GetTagID(lTagName)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	assert.Equal(t, lTagCommitID, oTagID)
}

func TestRepository_GetAnnotatedTag(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	clonedPath, err := cloneRepo(t, bareRepo1Path)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	bareRepo1, err := OpenRepository(t.Context(), clonedPath)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	defer bareRepo1.Close()

	lTagCommitID := "6fbd69e9823458e6c4a2fc5c0f6bc022b2f2acd1"
	lTagName := "lightweightTag"
	bareRepo1.CreateTag(lTagName, lTagCommitID)

	aTagCommitID := "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0"
	aTagName := "annotatedTag"
	aTagMessage := "my annotated message"
	bareRepo1.CreateAnnotatedTag(aTagName, aTagMessage, aTagCommitID)
	aTagID, _ := bareRepo1.GetTagID(aTagName)

	// Try an annotated tag
	tag, err := bareRepo1.GetAnnotatedTag(aTagID)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	assert.NotNil(t, tag)
	assert.Equal(t, aTagName, tag.Name)
	assert.Equal(t, aTagID, tag.ID.String())
	assert.Equal(t, "tag", tag.Type)

	// Annotated tag's Commit ID should fail
	tag2, err := bareRepo1.GetAnnotatedTag(aTagCommitID)
	assert.Error(t, err)
	assert.True(t, IsErrNotExist(err))
	assert.Nil(t, tag2)

	// Annotated tag's name should fail
	tag3, err := bareRepo1.GetAnnotatedTag(aTagName)
	assert.Errorf(t, err, "Length must be 40: %d", len(aTagName))
	assert.Nil(t, tag3)

	// Lightweight Tag should fail
	tag4, err := bareRepo1.GetAnnotatedTag(lTagCommitID)
	assert.Error(t, err)
	assert.True(t, IsErrNotExist(err))
	assert.Nil(t, tag4)
}
