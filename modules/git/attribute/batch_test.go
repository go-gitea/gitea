// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package attribute

import (
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_nulSeparatedAttributeWriter_ReadAttribute(t *testing.T) {
	wr := &nulSeparatedAttributeWriter{
		attributes: make(chan attributeTriple, 5),
	}

	testStr := ".gitignore\"\n\x00linguist-vendored\x00unspecified\x00"

	n, err := wr.Write([]byte(testStr))

	assert.Len(t, testStr, n)
	assert.NoError(t, err)
	select {
	case attr := <-wr.ReadAttribute():
		assert.Equal(t, ".gitignore\"\n", attr.Filename)
		assert.Equal(t, LinguistVendored, attr.Attribute)
		assert.Equal(t, "unspecified", attr.Value)
	case <-time.After(100 * time.Millisecond):
		assert.FailNow(t, "took too long to read an attribute from the list")
	}
	// Write a second attribute again
	n, err = wr.Write([]byte(testStr))

	assert.Len(t, testStr, n)
	assert.NoError(t, err)

	select {
	case attr := <-wr.ReadAttribute():
		assert.Equal(t, ".gitignore\"\n", attr.Filename)
		assert.Equal(t, LinguistVendored, attr.Attribute)
		assert.Equal(t, "unspecified", attr.Value)
	case <-time.After(100 * time.Millisecond):
		assert.FailNow(t, "took too long to read an attribute from the list")
	}

	// Write a partial attribute
	_, err = wr.Write([]byte("incomplete-file"))
	assert.NoError(t, err)
	_, err = wr.Write([]byte("name\x00"))
	assert.NoError(t, err)

	select {
	case <-wr.ReadAttribute():
		assert.FailNow(t, "There should not be an attribute ready to read")
	case <-time.After(100 * time.Millisecond):
	}
	_, err = wr.Write([]byte("attribute\x00"))
	assert.NoError(t, err)
	select {
	case <-wr.ReadAttribute():
		assert.FailNow(t, "There should not be an attribute ready to read")
	case <-time.After(100 * time.Millisecond):
	}

	_, err = wr.Write([]byte("value\x00"))
	assert.NoError(t, err)

	attr := <-wr.ReadAttribute()
	assert.Equal(t, "incomplete-filename", attr.Filename)
	assert.Equal(t, "attribute", attr.Attribute)
	assert.Equal(t, "value", attr.Value)

	_, err = wr.Write([]byte("shouldbe.vendor\x00linguist-vendored\x00set\x00shouldbe.vendor\x00linguist-generated\x00unspecified\x00shouldbe.vendor\x00linguist-language\x00unspecified\x00"))
	assert.NoError(t, err)
	attr = <-wr.ReadAttribute()
	assert.NoError(t, err)
	assert.Equal(t, attributeTriple{
		Filename:  "shouldbe.vendor",
		Attribute: LinguistVendored,
		Value:     "set",
	}, attr)
	attr = <-wr.ReadAttribute()
	assert.NoError(t, err)
	assert.Equal(t, attributeTriple{
		Filename:  "shouldbe.vendor",
		Attribute: LinguistGenerated,
		Value:     "unspecified",
	}, attr)
	attr = <-wr.ReadAttribute()
	assert.NoError(t, err)
	assert.Equal(t, attributeTriple{
		Filename:  "shouldbe.vendor",
		Attribute: LinguistLanguage,
		Value:     "unspecified",
	}, attr)
}

func expectedAttrs() *Attributes {
	return &Attributes{
		m: map[string]Attribute{
			LinguistGenerated:     "unspecified",
			LinguistDetectable:    "unspecified",
			LinguistDocumentation: "unspecified",
			LinguistVendored:      "unspecified",
			LinguistLanguage:      "Python",
			GitlabLanguage:        "unspecified",
		},
	}
}

func Test_BatchChecker(t *testing.T) {
	setting.AppDataPath = t.TempDir()
	repoPath := "../tests/repos/language_stats_repo"
	gitRepo, err := git.OpenRepository(t.Context(), repoPath)
	require.NoError(t, err)
	defer gitRepo.Close()

	commitID := "8fee858da5796dfb37704761701bb8e800ad9ef3"

	t.Run("Create index file to run git check-attr", func(t *testing.T) {
		defer test.MockVariableValue(&git.DefaultFeatures().SupportCheckAttrOnBare, false)()
		checker, err := NewBatchChecker(gitRepo, commitID, LinguistAttributes)
		assert.NoError(t, err)
		defer checker.Close()
		attributes, err := checker.CheckPath("i-am-a-python.p")
		assert.NoError(t, err)
		assert.Equal(t, expectedAttrs(), attributes)
	})

	// run git check-attr on work tree
	t.Run("Run git check-attr on git work tree", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "test-repo")
		err := git.Clone(t.Context(), repoPath, dir, git.CloneRepoOptions{
			Shared: true,
			Branch: "master",
		})
		assert.NoError(t, err)

		tempRepo, err := git.OpenRepository(t.Context(), dir)
		assert.NoError(t, err)
		defer tempRepo.Close()

		checker, err := NewBatchChecker(tempRepo, "", LinguistAttributes)
		assert.NoError(t, err)
		defer checker.Close()
		attributes, err := checker.CheckPath("i-am-a-python.p")
		assert.NoError(t, err)
		assert.Equal(t, expectedAttrs(), attributes)
	})

	if !git.DefaultFeatures().SupportCheckAttrOnBare {
		t.Skip("git version 2.40 is required to support run check-attr on bare repo")
		return
	}

	t.Run("Run git check-attr in bare repository", func(t *testing.T) {
		checker, err := NewBatchChecker(gitRepo, commitID, LinguistAttributes)
		assert.NoError(t, err)
		defer checker.Close()

		attributes, err := checker.CheckPath("i-am-a-python.p")
		assert.NoError(t, err)
		assert.Equal(t, expectedAttrs(), attributes)
	})
}
