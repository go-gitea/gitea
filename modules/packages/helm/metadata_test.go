// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package helm

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"strings"
	"testing"

	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
)

const chartFile = "apiVersion: v2\nname: test\nversion: 1.0.0\n"

func TestParseChartArchive(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		data := test.WriteTarCompression(gzip.NewWriter, map[string]string{"test/Chart.yaml": chartFile})
		metadata, err := ParseChartArchive(data)
		assert.NoError(t, err)
		assert.Equal(t, "test", metadata.Name)
	})

	t.Run("TooLarge", func(t *testing.T) {
		pr, pw := io.Pipe()
		go func() {
			gw, _ := gzip.NewWriterLevel(pw, gzip.BestSpeed)
			tw := tar.NewWriter(gw)
			_ = tw.WriteHeader(&tar.Header{Name: "test/padding", Mode: 0o600, Size: maxChartArchiveSize})
			_, _ = io.CopyN(tw, zeroReader{}, maxChartArchiveSize)
			_ = tw.WriteHeader(&tar.Header{Name: "test/Chart.yaml", Mode: 0o600, Size: int64(len(chartFile))})
			_, _ = tw.Write([]byte(chartFile))
			_ = tw.Close()
			_ = gw.Close()
			_ = pw.Close()
		}()
		metadata, err := ParseChartArchive(pr)
		_ = pr.Close()
		assert.Nil(t, metadata)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})
}

func TestParseChartFile(t *testing.T) {
	metadata, err := ParseChartFile(strings.NewReader(chartFile + "description: " + strings.Repeat("a", 9*1024*1024) + "\n"))
	assert.Nil(t, metadata)
	assert.ErrorIs(t, err, util.ErrInvalidArgument)
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
