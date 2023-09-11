// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package alpine

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	packageName        = "gitea"
	packageVersion     = "1.0.1"
	packageDescription = "Package Description"
	packageProjectURL  = "https://gitea.io"
	packageMaintainer  = "KN4CK3R <dummy@gitea.io>"
)

func createPKGINFOContent(name, version string) []byte {
	return []byte(`pkgname = ` + name + `
pkgver = ` + version + `
pkgdesc = ` + packageDescription + `
url = ` + packageProjectURL + `
# comment
builddate = 1678834800
packager = Gitea <pack@ag.er>
size = 123456
arch = aarch64
origin = origin
commit = 1111e709613fbc979651b09ac2bc27c6591a9999
maintainer = ` + packageMaintainer + `
license = MIT
depend = common
install_if = value
depend = gitea
provides = common
provides = gitea`)
}

func TestParsePackage(t *testing.T) {
	createPackage := func(name string, content []byte) io.Reader {
		names := []string{"first.stream", name}
		contents := [][]byte{{0}, content}

		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)

		for i := range names {
			if i != 0 {
				zw.Close()
				zw.Reset(&buf)
			}

			tw := tar.NewWriter(zw)
			hdr := &tar.Header{
				Name: names[i],
				Mode: 0o600,
				Size: int64(len(contents[i])),
			}
			tw.WriteHeader(hdr)
			tw.Write(contents[i])
			tw.Close()
		}

		zw.Close()

		return &buf
	}

	t.Run("MissingPKGINFOFile", func(t *testing.T) {
		data := createPackage("dummy.txt", []byte{})

		pp, err := ParsePackage(data)
		assert.Nil(t, pp)
		assert.ErrorIs(t, err, ErrMissingPKGINFOFile)
	})

	t.Run("InvalidPKGINFOFile", func(t *testing.T) {
		data := createPackage(".PKGINFO", []byte{})

		pp, err := ParsePackage(data)
		assert.Nil(t, pp)
		assert.ErrorIs(t, err, ErrInvalidName)
	})

	t.Run("Valid", func(t *testing.T) {
		data := createPackage(".PKGINFO", createPKGINFOContent(packageName, packageVersion))

		p, err := ParsePackage(data)
		assert.NoError(t, err)
		assert.NotNil(t, p)

		assert.Equal(t, "Q1SRYURM5+uQDqfHSwTnNIOIuuDVQ=", p.FileMetadata.Checksum)
	})
}

func TestParsePackageInfo(t *testing.T) {
	t.Run("InvalidName", func(t *testing.T) {
		data := createPKGINFOContent("", packageVersion)

		p, err := ParsePackageInfo(bytes.NewReader(data))
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrInvalidName)
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
		assert.Equal(t, packageVersion, p.Version)
		assert.Equal(t, packageDescription, p.VersionMetadata.Description)
		assert.Equal(t, packageMaintainer, p.VersionMetadata.Maintainer)
		assert.Equal(t, packageProjectURL, p.VersionMetadata.ProjectURL)
		assert.Equal(t, "MIT", p.VersionMetadata.License)
		assert.Empty(t, p.FileMetadata.Checksum)
		assert.Equal(t, "Gitea <pack@ag.er>", p.FileMetadata.Packager)
		assert.EqualValues(t, 1678834800, p.FileMetadata.BuildDate)
		assert.EqualValues(t, 123456, p.FileMetadata.Size)
		assert.Equal(t, "aarch64", p.FileMetadata.Architecture)
		assert.Equal(t, "origin", p.FileMetadata.Origin)
		assert.Equal(t, "1111e709613fbc979651b09ac2bc27c6591a9999", p.FileMetadata.CommitHash)
		assert.Equal(t, "value", p.FileMetadata.InstallIf)
		assert.ElementsMatch(t, []string{"common", "gitea"}, p.FileMetadata.Provides)
		assert.ElementsMatch(t, []string{"common", "gitea"}, p.FileMetadata.Dependencies)
	})
}
