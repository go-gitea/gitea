// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_GitBatchOperatorsNormal(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	batch, err := NewBatchCatFile(context.Background(), bareRepo1Path)
	assert.NoError(t, err)
	assert.NotNil(t, batch)
	defer batch.Close()

	err = batch.Input("refs/heads/master")
	assert.NoError(t, err)
	rd := batch.Reader()
	assert.NotNil(t, rd)

	_, typ, size, err := ReadBatchLine(rd)
	assert.NoError(t, err)
	assert.Equal(t, "commit", typ)
	assert.Equal(t, int64(1075), size)

	// this step is very important, otherwise the next read will be wrong
	s, err := rd.Discard(int(size))
	assert.NoError(t, err)
	assert.EqualValues(t, size, s)

	err = batch.Input("ce064814f4a0d337b333e646ece456cd39fab612")
	assert.NoError(t, err)
	assert.NotNil(t, rd)

	_, typ, size, err = ReadBatchLine(rd)
	assert.NoError(t, err)
	assert.Equal(t, "commit", typ)
	assert.Equal(t, int64(1075), size)

	s, err = rd.Discard(int(size))
	assert.NoError(t, err)
	assert.EqualValues(t, size, s)

	kases := []struct {
		refname string
		size    int64
	}{
		{"refs/heads/master", 1075},
		{"feaf4ba6bc635fec442f46ddd4512416ec43c2c2", 1074},
		{"37991dec2c8e592043f47155ce4808d4580f9123", 239},
	}

	var inputs []string
	for _, kase := range kases {
		inputs = append(inputs, kase.refname)
	}

	// input once for 3 refs
	err = batch.Input(inputs...)
	assert.NoError(t, err)
	assert.NotNil(t, rd)

	for i := 0; i < 3; i++ {
		_, typ, size, err = ReadBatchLine(rd)
		assert.NoError(t, err)
		assert.Equal(t, "commit", typ)
		assert.Equal(t, kases[i].size, size)

		s, err := rd.Discard(int(size))
		assert.NoError(t, err)
		assert.EqualValues(t, size, s)
	}

	// input 3 times
	for _, input := range inputs {
		err = batch.Input(input)
		assert.NoError(t, err)
		assert.NotNil(t, rd)
	}

	for i := 0; i < 3; i++ {
		_, typ, size, err = ReadBatchLine(rd)
		assert.NoError(t, err)
		assert.Equal(t, "commit", typ)
		assert.Equal(t, kases[i].size, size)

		s, err := rd.Discard(int(size))
		assert.NoError(t, err)
		assert.EqualValues(t, size, s)
	}
}

func Test_GitBatchOperatorsCancel(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	batch, err := NewBatchCatFile(context.Background(), bareRepo1Path)
	assert.NoError(t, err)
	assert.NotNil(t, batch)
	defer batch.Close()

	err = batch.Input("refs/heads/master")
	assert.NoError(t, err)
	rd := batch.Reader()
	assert.NotNil(t, rd)

	_, typ, size, err := ReadBatchLine(rd)
	assert.NoError(t, err)
	assert.Equal(t, "commit", typ)
	assert.Equal(t, int64(1075), size)

	go func() {
		time.Sleep(time.Second)
		batch.Cancel()
	}()
	// block here to wait cancel
	_, err = io.ReadAll(rd)
	assert.NoError(t, err)
}

func Test_GitBatchOperatorsTimeout(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)

	batch, err := NewBatchCatFile(ctx, bareRepo1Path)
	assert.NoError(t, err)
	assert.NotNil(t, batch)
	defer batch.Close()

	err = batch.Input("refs/heads/master")
	assert.NoError(t, err)
	rd := batch.Reader()
	assert.NotNil(t, rd)

	_, typ, size, err := ReadBatchLine(rd)
	assert.NoError(t, err)
	assert.Equal(t, "commit", typ)
	assert.Equal(t, int64(1075), size)
	// block here until timeout
	_, err = io.ReadAll(rd)
	assert.NoError(t, err)
}
