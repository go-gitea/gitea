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
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
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
	dumper := NewDumper("zip", &buf)

	testContent := "test content"
	testReader := io.NopCloser(strings.NewReader(testContent))
	testInfo := &testFileInfo{name: "test.txt", size: int64(len(testContent))}

	err := dumper.AddReader(testReader, testInfo, "test.txt")
	assert.NoError(t, err)

	err = dumper.Close()
	assert.NoError(t, err)

	assert.Positive(t, buf.Len(), "Archive should contain data")
}

type testFileInfo struct {
	name string
	size int64
}

func (t *testFileInfo) Name() string       { return t.name }
func (t *testFileInfo) Size() int64        { return t.size }
func (t *testFileInfo) Mode() os.FileMode  { return 0o644 }
func (t *testFileInfo) ModTime() time.Time { return time.Now() }
func (t *testFileInfo) IsDir() bool        { return false }
func (t *testFileInfo) Sys() any           { return nil }

func TestDumper(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, "include/exclude1"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "include/exclude2"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "include/sub"), 0o755)
	_ = os.WriteFile(filepath.Join(tmpDir, "include/a"), []byte("content-a"), 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "include/sub/b"), []byte("content-b"), 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "include/exclude1/a-1"), []byte("content-a-1"), 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "include/exclude2/a-2"), []byte("content-a-2"), 0o644)

	var buf1 bytes.Buffer
	dumper1 := NewDumper("tar", &buf1)
	dumper1.GlobalExcludeAbsPath(filepath.Join(tmpDir, "include/exclude1"))
	err := dumper1.AddRecursiveExclude("include", filepath.Join(tmpDir, "include"), []string{filepath.Join(tmpDir, "include/exclude2")})
	assert.NoError(t, err)
	err = dumper1.Close()
	assert.NoError(t, err)

	files1 := extractTarFileNames(t, &buf1)
	sortStrings := func(s []string) []string {
		sort.Strings(s)
		return s
	}

	expected1 := []string{"include/a", "include/sub", "include/sub/b"}
	assert.Equal(t, sortStrings(expected1), sortStrings(files1))

	var buf2 bytes.Buffer
	dumper2 := NewDumper("tar", &buf2)
	err = dumper2.AddRecursiveExclude("include", filepath.Join(tmpDir, "include"), nil)
	assert.NoError(t, err)
	err = dumper2.Close()
	assert.NoError(t, err)

	files2 := extractTarFileNames(t, &buf2)
	expected2 := []string{
		"include/exclude2", "include/exclude2/a-2",
		"include/a", "include/sub", "include/sub/b",
		"include/exclude1", "include/exclude1/a-1",
	}
	assert.Equal(t, sortStrings(expected2), sortStrings(files2))
}

func extractTarFileNames(t *testing.T, buf *bytes.Buffer) []string {
	var fileNames []string

	tr := tar.NewReader(buf)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err, "Error reading tar archive")

		if hdr.Typeflag == tar.TypeReg || hdr.Typeflag == tar.TypeDir {
			fileNames = append(fileNames, hdr.Name)
		}
	}

	return fileNames
}
