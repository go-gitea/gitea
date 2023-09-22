// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rotatingfilewriter

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompressOldFile(t *testing.T) {
	tmpDir := t.TempDir()
	fname := filepath.Join(tmpDir, "test")
	nonGzip := filepath.Join(tmpDir, "test-nonGzip")

	f, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0o660)
	assert.NoError(t, err)
	ng, err := os.OpenFile(nonGzip, os.O_CREATE|os.O_WRONLY, 0o660)
	assert.NoError(t, err)

	for i := 0; i < 999; i++ {
		f.WriteString("This is a test file\n")
		ng.WriteString("This is a test file\n")
	}
	f.Close()
	ng.Close()

	err = compressOldFile(fname, gzip.DefaultCompression)
	assert.NoError(t, err)

	_, err = os.Lstat(fname + ".gz")
	assert.NoError(t, err)

	f, err = os.Open(fname + ".gz")
	assert.NoError(t, err)
	zr, err := gzip.NewReader(f)
	assert.NoError(t, err)
	data, err := io.ReadAll(zr)
	assert.NoError(t, err)
	original, err := os.ReadFile(nonGzip)
	assert.NoError(t, err)
	assert.Equal(t, original, data)
}
