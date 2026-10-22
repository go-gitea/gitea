// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"

	"gitea.dev/modules/packages"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseChartArchiveBoundsInflatedSize(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	size := int64(maxArchiveSize + 1)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "chart/filler", Typeflag: tar.TypeReg, Size: size}))
	zeros := make([]byte, 1024*1024)
	for written := int64(0); written < size; written += int64(len(zeros)) {
		_, err := tw.Write(zeros[:min(int64(len(zeros)), size-written)])
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	_, err := ParseChartArchive(&buf)
	assert.ErrorIs(t, err, packages.ErrContentTooLarge)
}
