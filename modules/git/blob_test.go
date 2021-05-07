// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlob_Data(t *testing.T) {
	output := "file2\n"
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	repo, err := OpenRepository(bareRepo1Path)
	if !assert.NoError(t, err) {
		t.Fatal()
	}
	defer repo.Close()

	testBlob, err := repo.GetBlob("6c493ff740f9380390d5c9ddef4af18697ac9375")
	assert.NoError(t, err)

	r, err := testBlob.DataAsync()
	assert.NoError(t, err)
	require.NotNil(t, r)
	defer r.Close()

	data, err := ioutil.ReadAll(r)
	assert.NoError(t, err)
	assert.Equal(t, output, string(data))
}

func Benchmark_Blob_Data(b *testing.B) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	repo, err := OpenRepository(bareRepo1Path)
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
		defer r.Close()
		ioutil.ReadAll(r)
	}
}
