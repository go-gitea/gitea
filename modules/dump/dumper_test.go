// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dump

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareFileNameAndType(t *testing.T) {
	defer timeutil.MockSet(time.Unix(1234, 0))()
	test := func(argFile, argType, expFile, expType string) {
		outFile, outType := PrepareFileNameAndType(argFile, argType)
		assert.Equal(t,
			fmt.Sprintf("outFile=%s, outType=%s", expFile, expType),
			fmt.Sprintf("outFile=%s, outType=%s", outFile, outType),
			"argFile=%s, argType=%s", argFile, argType,
		)
	}

	test("", "", "gitea-dump-1234.zip", "zip")
	test("", "tar.gz", "gitea-dump-1234.tar.gz", "tar.gz")
	test("", "no-such", "", "")

	test("-", "", "-", "zip")
	test("-", "tar.gz", "-", "tar.gz")
	test("-", "no-such", "", "")

	test("a", "", "a", "zip")
	test("a", "tar.gz", "a", "tar.gz")
	test("a", "no-such", "", "")

	test("a.zip", "", "a.zip", "zip")
	test("a.zip", "tar.gz", "a.zip", "tar.gz")
	test("a.zip", "no-such", "", "")

	test("a.tar.gz", "", "a.tar.gz", "zip")
	test("a.tar.gz", "tar.gz", "a.tar.gz", "tar.gz")
	test("a.tar.gz", "no-such", "", "")
}

func TestIsSubDir(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, "include/sub"), 0o755)

	isSub, err := IsSubdir(filepath.Join(tmpDir, "include"), filepath.Join(tmpDir, "include"))
	assert.NoError(t, err)
	assert.True(t, isSub)

	isSub, err = IsSubdir(filepath.Join(tmpDir, "include"), filepath.Join(tmpDir, "include/sub"))
	assert.NoError(t, err)
	assert.True(t, isSub)

	isSub, err = IsSubdir(filepath.Join(tmpDir, "include/sub"), filepath.Join(tmpDir, "include"))
	assert.NoError(t, err)
	assert.False(t, isSub)
}

func TestDumperIntegration(t *testing.T) {
	var buf bytes.Buffer
	dumper, err := NewDumper(t.Context(), "zip", &buf)
	require.NoError(t, err)

	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "test.txt"), nil, 0o644)
	f, _ := os.Open(filepath.Join(tmpDir, "test.txt"))

	fi, _ := f.Stat()
	err = dumper.AddFileByReader(f, fi, "test.txt")
	require.NoError(t, err)

	err = dumper.Close()
	require.NoError(t, err)

	assert.Positive(t, buf.Len())
}

func TestDumper(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, "include/exclude1"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "include/exclude2"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "include/sub"), 0o755)
	_ = os.WriteFile(filepath.Join(tmpDir, "include/a"), nil, 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "include/sub/b"), nil, 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "include/exclude1/a-1"), nil, 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "include/exclude2/a-2"), nil, 0o644)

	sortStrings := func(s []string) []string {
		sort.Strings(s)
		return s
	}

	t.Run("IncludesWithExcludes", func(t *testing.T) {
		var buf bytes.Buffer
		dumper, err := NewDumper(t.Context(), "tar", &buf)
		require.NoError(t, err)
		dumper.GlobalExcludeAbsPath(filepath.Join(tmpDir, "include/exclude1"))
		err = dumper.AddRecursiveExclude("include", filepath.Join(tmpDir, "include"), []string{filepath.Join(tmpDir, "include/exclude2")})
		require.NoError(t, err)
		err = dumper.Close()
		require.NoError(t, err)

		files := extractTarFileNames(t, &buf)
		expected := []string{"include/a", "include/sub", "include/sub/b"}
		assert.Equal(t, sortStrings(expected), sortStrings(files))
	})

	t.Run("IncludesAll", func(t *testing.T) {
		var buf bytes.Buffer
		dumper, err := NewDumper(t.Context(), "tar", &buf)
		require.NoError(t, err)
		err = dumper.AddRecursiveExclude("include", filepath.Join(tmpDir, "include"), nil)
		require.NoError(t, err)
		err = dumper.Close()
		require.NoError(t, err)

		files := extractTarFileNames(t, &buf)
		expected := []string{
			"include/exclude2", "include/exclude2/a-2",
			"include/a", "include/sub", "include/sub/b",
			"include/exclude1", "include/exclude1/a-1",
		}
		assert.Equal(t, sortStrings(expected), sortStrings(files))
	})
}

func extractTarFileNames(t *testing.T, buf *bytes.Buffer) (fileNames []string) {
	tr := tar.NewReader(buf)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "Error reading tar archive")
		fileNames = append(fileNames, hdr.Name)
	}
	return fileNames
}
