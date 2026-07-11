// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"strconv"
	"testing"

	"gitea.dev/modules/util"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"
)

const (
	packageName        = "gitea"
	packageVersion     = "1.0.1"
	packageDescription = "Package Description"
	packageProjectURL  = "https://gitea.com"
	packagePackager    = "KN4CK3R <packager@gitea.com>"
)

func createPKGINFOContent(name, version string) []byte {
	return []byte(`pkgname = ` + name + `
pkgbase = ` + name + `
pkgver = ` + version + `
pkgdesc = ` + packageDescription + `
url = ` + packageProjectURL + `
# comment
group=group
builddate = 1678834800
size = 123456
arch = x86_64
license = MIT
packager = ` + packagePackager + `
depend = common
xdata = value
depend = gitea
provides = common
provides = gitea
optdepend = hex
replaces = gogs
checkdepend = common
makedepend = cmake
conflict = ninja
backup = usr/bin/paket1`)
}

func TestParsePackage(t *testing.T) {
	createPackage := func(compression string, files map[string][]byte) io.Reader {
		var buf bytes.Buffer
		var cw io.WriteCloser
		switch compression {
		case "zst":
			cw, _ = zstd.NewWriter(&buf)
		case "xz":
			cw, _ = xz.NewWriter(&buf)
		case "gz":
			cw = gzip.NewWriter(&buf)
		}
		tw := tar.NewWriter(cw)

		for name, content := range files {
			hdr := &tar.Header{
				Name: name,
				Mode: 0o600,
				Size: int64(len(content)),
			}
			tw.WriteHeader(hdr)
			tw.Write(content)
		}

		tw.Close()
		cw.Close()

		return &buf
	}

	for _, c := range []string{"gz", "xz", "zst"} {
		t.Run(c, func(t *testing.T) {
			t.Run("MissingPKGINFOFile", func(t *testing.T) {
				data := createPackage(c, map[string][]byte{"dummy.txt": {}})

				pp, err := ParsePackage(data)
				assert.Nil(t, pp)
				assert.ErrorIs(t, err, ErrMissingPKGINFOFile)
			})

			t.Run("InvalidPKGINFOFile", func(t *testing.T) {
				data := createPackage(c, map[string][]byte{".PKGINFO": {}})

				pp, err := ParsePackage(data)
				assert.Nil(t, pp)
				assert.ErrorIs(t, err, ErrInvalidName)
			})

			t.Run("Valid", func(t *testing.T) {
				data := createPackage(c, map[string][]byte{
					".PKGINFO":        createPKGINFOContent(packageName, packageVersion),
					"/test/dummy.txt": {},
				})

				p, err := ParsePackage(data)
				assert.NoError(t, err)
				assert.NotNil(t, p)

				assert.ElementsMatch(t, []string{"/test/dummy.txt"}, p.FileMetadata.Files)
			})
		})
	}
}

func TestParsePackageInfo(t *testing.T) {
	t.Run("InvalidName", func(t *testing.T) {
		data := createPKGINFOContent("", packageVersion)

		p, err := ParsePackageInfo(bytes.NewReader(data))
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrInvalidName)
	})

	t.Run("Regexp", func(t *testing.T) {
		assert.Regexp(t, versionPattern, "1.2_3~4+5")
		assert.Regexp(t, versionPattern, "1:2_3~4+5")
		assert.NotRegexp(t, versionPattern, "a:1.0.0-1")
		assert.NotRegexp(t, versionPattern, "0.0.1/1-1")
		assert.NotRegexp(t, versionPattern, "1.0.0 -1")
	})

	t.Run("InvalidVersion", func(t *testing.T) {
		data := createPKGINFOContent(packageName, "")

		p, err := ParsePackageInfo(bytes.NewReader(data))
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrInvalidVersion)
	})

	t.Run("Valid", func(t *testing.T) {
		data := createPKGINFOContent(packageName, packageVersion)

		p, err := ParsePackageInfo(bytes.NewReader(data))
		assert.NoError(t, err)
		assert.NotNil(t, p)

		assert.Equal(t, packageName, p.Name)
		assert.Equal(t, packageName, p.FileMetadata.Base)
		assert.Equal(t, packageVersion, p.Version)
		assert.Equal(t, packageDescription, p.VersionMetadata.Description)
		assert.Equal(t, packagePackager, p.FileMetadata.Packager)
		assert.Equal(t, packageProjectURL, p.VersionMetadata.ProjectURL)
		assert.ElementsMatch(t, []string{"MIT"}, p.VersionMetadata.Licenses)
		assert.EqualValues(t, 1678834800, p.FileMetadata.BuildDate)
		assert.EqualValues(t, 123456, p.FileMetadata.InstalledSize)
		assert.Equal(t, "x86_64", p.FileMetadata.Architecture)
		assert.ElementsMatch(t, []string{"value"}, p.FileMetadata.XData)
		assert.ElementsMatch(t, []string{"group"}, p.FileMetadata.Groups)
		assert.ElementsMatch(t, []string{"common", "gitea"}, p.FileMetadata.Provides)
		assert.ElementsMatch(t, []string{"common", "gitea"}, p.FileMetadata.Depends)
		assert.ElementsMatch(t, []string{"gogs"}, p.FileMetadata.Replaces)
		assert.ElementsMatch(t, []string{"hex"}, p.FileMetadata.OptDepends)
		assert.ElementsMatch(t, []string{"common"}, p.FileMetadata.CheckDepends)
		assert.ElementsMatch(t, []string{"ninja"}, p.FileMetadata.Conflicts)
		assert.ElementsMatch(t, []string{"cmake"}, p.FileMetadata.MakeDepends)
		assert.ElementsMatch(t, []string{"usr/bin/paket1"}, p.FileMetadata.Backup)
	})
}

// TestParsePackageTooManyFiles ensures the accumulated file list is bounded to prevent
// metadata amplification from a package with a huge number of (tiny) file entries.
func TestParsePackageTooManyFiles(t *testing.T) {
	old := maxFileEntries
	maxFileEntries = 3
	defer func() { maxFileEntries = old }()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	writeEntry := func(name string, content []byte) {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(content))}))
		_, err := tw.Write(content)
		require.NoError(t, err)
	}
	writeEntry(".PKGINFO", createPKGINFOContent(packageName, packageVersion))
	for i := 0; i <= maxFileEntries; i++ {
		writeEntry("file"+strconv.Itoa(i), []byte{})
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	p, err := ParsePackage(&buf)
	assert.Nil(t, p)
	assert.ErrorIs(t, err, util.ErrInvalidArgument)
}
