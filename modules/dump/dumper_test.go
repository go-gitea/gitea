// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dump

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/timeutil"

	"github.com/mholt/archiver/v3"
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

type testWriter struct {
	added []string
}

func (t *testWriter) Create(out io.Writer) error {
	return nil
}

func (t *testWriter) Write(f archiver.File) error {
	t.added = append(t.added, f.Name())
	return nil
}

func (t *testWriter) Close() error {
	return nil
}

func TestDumper(t *testing.T) {
	sortStrings := func(s []string) []string {
		sort.Strings(s)
		return s
	}
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, "include/exclude1"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "include/exclude2"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "include/sub"), 0o755)
	_ = os.WriteFile(filepath.Join(tmpDir, "include/a"), nil, 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "include/sub/b"), nil, 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "include/exclude1/a-1"), nil, 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "include/exclude2/a-2"), nil, 0o644)

	tw := &testWriter{}
	d := &Dumper{Writer: tw}
	d.GlobalExcludeAbsPath(filepath.Join(tmpDir, "include/exclude1"))
	err := d.AddRecursiveExclude("include", filepath.Join(tmpDir, "include"), []string{filepath.Join(tmpDir, "include/exclude2")})
	assert.NoError(t, err)
	assert.EqualValues(t, sortStrings([]string{"include/a", "include/sub", "include/sub/b"}), sortStrings(tw.added))

	tw = &testWriter{}
	d = &Dumper{Writer: tw}
	err = d.AddRecursiveExclude("include", filepath.Join(tmpDir, "include"), nil)
	assert.NoError(t, err)
	assert.EqualValues(t, sortStrings([]string{"include/exclude2", "include/exclude2/a-2", "include/a", "include/sub", "include/sub/b", "include/exclude1", "include/exclude1/a-1"}), sortStrings(tw.added))
}
