// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestRepository_GetTags(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepository(bareRepo1Path)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	defer bareRepo1.Close()

	tags, total, err := bareRepo1.GetTagInfos(0, 0)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	assert.Len(t, tags, 1)
	assert.Equal(t, len(tags), total)
	assert.EqualValues(t, "test", tags[0].Name)
	assert.EqualValues(t, "3ad28a9149a2864384548f3d17ed7f38014c9e8a", tags[0].ID.String())
	assert.EqualValues(t, "tag", tags[0].Type)
}

func TestRepository_GetTag(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	clonedPath, err := cloneRepo(bareRepo1Path, "TestRepository_GetTag")
	if err != nil {
		assert.NoError(t, err)
		return
	}
	defer util.RemoveAll(clonedPath)

	bareRepo1, err := OpenRepository(clonedPath)
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
	if err != nil {
		assert.NoError(t, err)
		return
	}
	if lTag == nil {
		assert.NotNil(t, lTag)
		assert.FailNow(t, "nil lTag: %s", lTagName)
		return
	}
	assert.EqualValues(t, lTagName, lTag.Name)
	assert.EqualValues(t, lTagCommitID, lTag.ID.String())
	assert.EqualValues(t, lTagCommitID, lTag.Object.String())
	assert.EqualValues(t, "commit", lTag.Type)

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
	if err != nil {
		assert.NoError(t, err)
		return
	}
	if aTag == nil {
		assert.NotNil(t, aTag)
		assert.FailNow(t, "nil aTag: %s", aTagName)
		return
	}
	assert.EqualValues(t, aTagName, aTag.Name)
	assert.EqualValues(t, aTagID, aTag.ID.String())
	assert.NotEqual(t, aTagID, aTag.Object.String())
	assert.EqualValues(t, aTagCommitID, aTag.Object.String())
	assert.EqualValues(t, "tag", aTag.Type)

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
	assert.EqualValues(t, rTagCommitID, rTagID)

	oTagID, err := bareRepo1.GetTagID(lTagName)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	assert.EqualValues(t, lTagCommitID, oTagID)
}

func TestRepository_GetAnnotatedTag(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	clonedPath, err := cloneRepo(bareRepo1Path, "TestRepository_GetAnnotatedTag")
	if err != nil {
		assert.NoError(t, err)
		return
	}
	defer util.RemoveAll(clonedPath)

	bareRepo1, err := OpenRepository(clonedPath)
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
