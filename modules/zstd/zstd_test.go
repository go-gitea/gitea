// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package zstd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PrepareTestData prepares test data to test compression.
// Random data is not suitable for testing compression,
// so it collects code files from the project to get enough data.
func PrepareTestData(t *testing.T, size int) []byte {
	// .../gitea/modules/zstd
	dir, err := os.Getwd()
	require.NoError(t, err)
	// .../gitea/
	dir = filepath.Join(dir, "../../")

	textExt := []string{".go", ".tmpl", ".ts", ".yml", ".css"} // add more if not enough data collected
	isText := func(info os.FileInfo) bool {
		if info.Size() == 0 {
			return false
		}
		for _, ext := range textExt {
			if strings.HasSuffix(info.Name(), ext) {
				return true
			}
		}
		return false
	}

	ret := make([]byte, size)
	n := 0
	count := 0

	queue := []string{dir}
	for len(queue) > 0 && n < size {
		file := queue[0]
		queue = queue[1:]
		info, err := os.Stat(file)
		require.NoError(t, err)
		if info.IsDir() {
			entries, err := os.ReadDir(file)
			require.NoError(t, err)
			for _, entry := range entries {
				queue = append(queue, filepath.Join(file, entry.Name()))
			}
			continue
		}
		if !isText(info) { // text file only
			continue
		}
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		n += copy(ret[n:], data)
		count++
	}

	if n < size {
		require.Failf(t, "Not enough data", "Only %d bytes collected from %d files", n, count)
	}
	return ret
}

func TestWriterReader(t *testing.T) {
	testData := PrepareTestData(t, 50_000_000)

	result := bytes.NewBuffer(nil)

	t.Run("regular", func(t *testing.T) {
		result.Reset()
		writer, err := NewWriter(result)
		require.NoError(t, err)

		_, err = io.Copy(writer, bytes.NewReader(testData))
		require.NoError(t, err)

		require.NoError(t, writer.Close())

		t.Logf("original size: %d, compressed size: %d, rate: %.2f%%", len(testData), result.Len(), float64(result.Len())/float64(len(testData))*100)

		reader, err := NewReader(result)
		require.NoError(t, err)

		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())

		assert.Equal(t, testData, data)
	})

	t.Run("with options", func(t *testing.T) {
		result.Reset()
		writer, err := NewWriter(result, WithEncoderLevel(3))
		require.NoError(t, err)

		_, err = io.Copy(writer, bytes.NewReader(testData))
		require.NoError(t, err)

		require.NoError(t, writer.Close())

		t.Logf("original size: %d, compressed size: %d, rate: %.2f%%", len(testData), result.Len(), float64(result.Len())/float64(len(testData))*100)

		reader, err := NewReader(result)
		require.NoError(t, err)

		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())

		assert.Equal(t, testData, data)
	})
}
