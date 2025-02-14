// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlob_Data(t *testing.T) {
	output := "file2\n"
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	repo, err := openRepositoryWithDefaultContext(bareRepo1Path)
	require.NoError(t, err)
	defer repo.Close()

	testBlob, err := repo.GetBlob("6c493ff740f9380390d5c9ddef4af18697ac9375")
	assert.NoError(t, err)

	r, err := testBlob.DataAsync()
	assert.NoError(t, err)
	require.NotNil(t, r)

	data, err := io.ReadAll(r)
	assert.NoError(t, r.Close())

	assert.NoError(t, err)
	assert.Equal(t, output, string(data))
}

func Benchmark_Blob_Data(b *testing.B) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	repo, err := openRepositoryWithDefaultContext(bareRepo1Path)
	if err != nil {
		b.Fatal(err)
	}
	defer repo.Close()

	testBlob, err := repo.GetBlob("6c493ff740f9380390d5c9ddef4af18697ac9375")
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		r, err := testBlob.DataAsync()
		if err != nil {
			b.Fatal(err)
		}
		io.ReadAll(r)
		_ = r.Close()
	}
}
