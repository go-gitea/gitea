// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepository_GetTags(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepository(bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	tags, err := bareRepo1.GetTagInfos()
	assert.NoError(t, err)
	assert.Len(t, tags, 1)
	assert.EqualValues(t, "test", tags[0].Name)
	assert.EqualValues(t, "3ad28a9149a2864384548f3d17ed7f38014c9e8a", tags[0].ID.String())
	assert.EqualValues(t, "tag", tags[0].Type)
}

func TestRepository_GetTag(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	clonedPath, err := cloneRepo(bareRepo1Path, testReposDir, "repo1_TestRepository_GetTag")
	assert.NoError(t, err)
	defer os.RemoveAll(clonedPath)

	bareRepo1, err := OpenRepository(clonedPath)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	lTagCommitID := "6fbd69e9823458e6c4a2fc5c0f6bc022b2f2acd1"
	lTagName := "lightweightTag"
	bareRepo1.CreateTag(lTagName, lTagCommitID)

	aTagCommitID := "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0"
	aTagName := "annotatedTag"
	aTagMessage := "my annotated message"
	bareRepo1.CreateAnnotatedTag(aTagName, aTagMessage, aTagCommitID)
	aTagID, _ := bareRepo1.GetTagID(aTagName)

	lTag, err := bareRepo1.GetTag(lTagName)
	lTag.repo = nil
	assert.NoError(t, err)
	assert.NotNil(t, lTag)
	assert.EqualValues(t, lTagName, lTag.Name)
	assert.EqualValues(t, lTagCommitID, lTag.ID.String())
	assert.EqualValues(t, lTagCommitID, lTag.Object.String())
	assert.EqualValues(t, "commit", lTag.Type)

	aTag, err := bareRepo1.GetTag(aTagName)
	assert.NoError(t, err)
	assert.NotNil(t, aTag)
	assert.EqualValues(t, aTagName, aTag.Name)
	assert.EqualValues(t, aTagID, aTag.ID.String())
	assert.NotEqual(t, aTagID, aTag.Object.String())
	assert.EqualValues(t, aTagCommitID, aTag.Object.String())
	assert.EqualValues(t, "tag", aTag.Type)

	rTagCommitID := "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0"
	rTagName := "release/" + lTagName
	bareRepo1.CreateTag(rTagName, rTagCommitID)
	rTagID, err := bareRepo1.GetTagID(rTagName)
	assert.NoError(t, err)
	assert.EqualValues(t, rTagCommitID, rTagID)
	oTagID, err := bareRepo1.GetTagID(lTagName)
	assert.NoError(t, err)
	assert.EqualValues(t, lTagCommitID, oTagID)
}

func TestRepository_GetAnnotatedTag(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	clonedPath, err := cloneRepo(bareRepo1Path, testReposDir, "repo1_TestRepository_GetTag")
	assert.NoError(t, err)
	defer os.RemoveAll(clonedPath)

	bareRepo1, err := OpenRepository(clonedPath)
	assert.NoError(t, err)
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
	assert.NoError(t, err)
	assert.NotNil(t, tag)
	assert.EqualValues(t, aTagName, tag.Name)
	assert.EqualValues(t, aTagID, tag.ID.String())
	assert.EqualValues(t, "tag", tag.Type)

	// Annotated tag's Commit ID should fail
	tag2, err := bareRepo1.GetAnnotatedTag(aTagCommitID)
	assert.Error(t, err)
	assert.True(t, IsErrNotExist(err))
	assert.Nil(t, tag2)

	// Annotated tag's name should fail
	tag3, err := bareRepo1.GetAnnotatedTag(aTagName)
	assert.Error(t, err)
	assert.Errorf(t, err, "Length must be 40: %d", len(aTagName))
	assert.Nil(t, tag3)

	// Lightweight Tag should fail
	tag4, err := bareRepo1.GetAnnotatedTag(lTagCommitID)
	assert.Error(t, err)
	assert.True(t, IsErrNotExist(err))
	assert.Nil(t, tag4)
}
