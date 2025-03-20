// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	mathRand "math/rand/v2"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

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
		assert.Equal(t, AttributeLinguistVendored, attr.Attribute)
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
		assert.Equal(t, AttributeLinguistVendored, attr.Attribute)
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
	assert.EqualValues(t, attributeTriple{
		Filename:  "shouldbe.vendor",
		Attribute: AttributeLinguistVendored,
		Value:     "set",
	}, attr)
	attr = <-wr.ReadAttribute()
	assert.NoError(t, err)
	assert.EqualValues(t, attributeTriple{
		Filename:  "shouldbe.vendor",
		Attribute: AttributeLinguistGenerated,
		Value:     "unspecified",
	}, attr)
	attr = <-wr.ReadAttribute()
	assert.NoError(t, err)
	assert.EqualValues(t, attributeTriple{
		Filename:  "shouldbe.vendor",
		Attribute: AttributeLinguistLanguage,
		Value:     "unspecified",
	}, attr)
}

func TestAttributeReader(t *testing.T) {
	t.Skip() // for debug purpose only, do not run in CI

	ctx := t.Context()

	timeout := 1 * time.Second
	repoPath := filepath.Join(testReposDir, "language_stats_repo")
	commitRef := "HEAD"

	oneRound := func(t *testing.T, roundIdx int) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		_ = cancel
		gitRepo, err := OpenRepository(ctx, repoPath)
		require.NoError(t, err)
		defer gitRepo.Close()

		commit, err := gitRepo.GetCommit(commitRef)
		require.NoError(t, err)

		files, err := gitRepo.LsFiles()
		require.NoError(t, err)

		randomFiles := slices.Clone(files)
		randomFiles = append(randomFiles, "any-file-1", "any-file-2")

		t.Logf("Round %v with %d files", roundIdx, len(randomFiles))

		attrReader, deferrable := gitRepo.CheckAttributeReader(commit.ID.String())
		defer deferrable()

		wg := sync.WaitGroup{}
		wg.Add(1)

		go func() {
			for {
				file := randomFiles[mathRand.IntN(len(randomFiles))]
				_, err := attrReader.CheckPath(file)
				if err != nil {
					for i := 0; i < 10; i++ {
						_, _ = attrReader.CheckPath(file)
					}
					break
				}
			}
			wg.Done()
		}()
		wg.Wait()
	}

	for i := 0; i < 100; i++ {
		oneRound(t, i)
	}
}
