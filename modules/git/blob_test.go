// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var repoSelf = &Repository{
	Path: "./",
}

var testBlob = &Blob{
	repo: repoSelf,
	TreeEntry: &TreeEntry{
		ID: MustIDFromString("a8d4b49dd073a4a38a7e58385eeff7cc52568697"),
		ptree: &Tree{
			repo: repoSelf,
		},
	},
}

func TestBlob_Data(t *testing.T) {
	output := `Copyright (c) 2016 The Gitea Authors
Copyright (c) 2015 The Gogs Authors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
`

	r, err := testBlob.Data()
	assert.NoError(t, err)
	require.NotNil(t, r)

	data, err := ioutil.ReadAll(r)
	assert.NoError(t, err)
	assert.Equal(t, output, string(data))
}

func Benchmark_Blob_Data(b *testing.B) {
	for i := 0; i < b.N; i++ {
		r, err := testBlob.Data()
		if err != nil {
			b.Fatal(err)
		}
		ioutil.ReadAll(r)
	}
}

func Benchmark_Blob_DataPipeline(b *testing.B) {
	stdout := new(bytes.Buffer)
	for i := 0; i < b.N; i++ {
		stdout.Reset()
		if err := testBlob.DataPipeline(stdout, nil); err != nil {
			b.Fatal(err)
		}
	}
}
