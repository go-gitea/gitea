// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package composer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/json"

	"github.com/dsnet/compress/bzip2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	name        = "gitea/composer-package"
	description = "Package Description"
	readme      = "Package Readme"
	comments    = "Package Comment"
	packageType = "composer-plugin"
	author      = "Gitea Authors"
	email       = "no.reply@gitea.io"
	homepage    = "https://gitea.io"
	license     = "MIT"
)

func buildComposerContent(version string) string {
	return `{
    "name": "` + name + `",
		"version": "` + version + `",
    "description": "` + description + `",
    "type": "` + packageType + `",
    "license": "` + license + `",
    "authors": [
        {
            "name": "` + author + `",
            "email": "` + email + `"
        }
    ],
    "homepage": "` + homepage + `",
    "autoload": {
        "psr-4": {"Gitea\\ComposerPackage\\": "src/"}
    },
    "require": {
        "php": ">=7.2 || ^8.0"
    },
    "_comment": "` + comments + `"
}`
}

func TestLicenseUnmarshal(t *testing.T) {
	var l Licenses
	assert.NoError(t, json.NewDecoder(strings.NewReader(`["MIT"]`)).Decode(&l))
	assert.Len(t, l, 1)
	assert.Equal(t, "MIT", l[0])
	assert.NoError(t, json.NewDecoder(strings.NewReader(`"MIT"`)).Decode(&l))
	assert.Len(t, l, 1)
	assert.Equal(t, "MIT", l[0])
}

func TestCommentsUnmarshal(t *testing.T) {
	var c Comments
	assert.NoError(t, json.NewDecoder(strings.NewReader(`["comment"]`)).Decode(&c))
	assert.Len(t, c, 1)
	assert.Equal(t, "comment", c[0])
	assert.NoError(t, json.NewDecoder(strings.NewReader(`"comment"`)).Decode(&c))
	assert.Len(t, c, 1)
	assert.Equal(t, "comment", c[0])
}

func TestParsePackage(t *testing.T) {
	createArchive := func(files map[string]string) []byte {
		var buf bytes.Buffer
		archive := zip.NewWriter(&buf)
		for name, content := range files {
			w, _ := archive.Create(name)
			_, _ = w.Write([]byte(content))
		}
		_ = archive.Close()
		return buf.Bytes()
	}

	createArchiveTar := func(comp func(io.Writer) io.WriteCloser, files map[string]string) []byte {
		var buf bytes.Buffer
		w := comp(&buf)
		archive := tar.NewWriter(w)
		for name, content := range files {
			hdr := &tar.Header{
				Name: name,
				Mode: 0o600,
				Size: int64(len(content)),
			}
			_ = archive.WriteHeader(hdr)
			_, _ = archive.Write([]byte(content))
		}
		_ = w.Close()
		_ = archive.Close()
		return buf.Bytes()
	}

	t.Run("MissingComposerFile", func(t *testing.T) {
		data := createArchive(map[string]string{"dummy.txt": ""})

		cp, err := ParsePackage(bytes.NewReader(data))
		assert.Nil(t, cp)
		assert.ErrorIs(t, err, ErrMissingComposerFile)
	})

	t.Run("MissingComposerFileInRoot", func(t *testing.T) {
		data := createArchive(map[string]string{"sub/sub/composer.json": ""})

		cp, err := ParsePackage(bytes.NewReader(data))
		assert.Nil(t, cp)
		assert.ErrorIs(t, err, ErrMissingComposerFile)
	})

	t.Run("InvalidComposerFile", func(t *testing.T) {
		data := createArchive(map[string]string{"composer.json": ""})

		cp, err := ParsePackage(bytes.NewReader(data))
		assert.Nil(t, cp)
		assert.Error(t, err)
	})

	t.Run("InvalidPackageName", func(t *testing.T) {
		data := createArchive(map[string]string{"composer.json": "{}"})

		cp, err := ParsePackage(bytes.NewReader(data))
		assert.Nil(t, cp)
		assert.ErrorIs(t, err, ErrInvalidName)
	})

	t.Run("InvalidPackageVersion", func(t *testing.T) {
		data := createArchive(map[string]string{"composer.json": `{"name": "gitea/composer-package", "version": "1.a.3"}`})

		cp, err := ParsePackage(bytes.NewReader(data))
		assert.Nil(t, cp)
		assert.ErrorIs(t, err, ErrInvalidVersion)
	})

	t.Run("InvalidReadmePath", func(t *testing.T) {
		data := createArchive(map[string]string{"composer.json": `{"name": "gitea/composer-package", "readme": "sub/README.md"}`})

		cp, err := ParsePackage(bytes.NewReader(data))
		assert.NoError(t, err)
		assert.NotNil(t, cp)

		assert.Empty(t, cp.Metadata.Readme)
	})

	assertValidPackage := func(t *testing.T, data []byte, version, filename string) {
		cp, err := ParsePackage(bytes.NewReader(data))
		require.NoError(t, err)
		assert.NotNil(t, cp)

		assert.Equal(t, filename, cp.Filename)
		assert.Equal(t, name, cp.Name)
		assert.Equal(t, version, cp.Version)
		assert.Equal(t, description, cp.Metadata.Description)
		assert.Equal(t, readme, cp.Metadata.Readme)
		assert.Len(t, cp.Metadata.Comments, 1)
		assert.Equal(t, comments, cp.Metadata.Comments[0])
		assert.Len(t, cp.Metadata.Authors, 1)
		assert.Equal(t, author, cp.Metadata.Authors[0].Name)
		assert.Equal(t, email, cp.Metadata.Authors[0].Email)
		assert.Equal(t, homepage, cp.Metadata.Homepage)
		assert.Equal(t, packageType, cp.Type)
		assert.Len(t, cp.Metadata.License, 1)
		assert.Equal(t, license, cp.Metadata.License[0])
	}

	t.Run("ValidZip", func(t *testing.T) {
		data := createArchive(map[string]string{"composer.json": buildComposerContent(""), "README.md": readme})
		assertValidPackage(t, data, "", "gitea-composer-package.zip")
	})

	t.Run("ValidTarBz2", func(t *testing.T) {
		data := createArchiveTar(func(w io.Writer) io.WriteCloser {
			bz2Writer, _ := bzip2.NewWriter(w, nil)
			return bz2Writer
		}, map[string]string{"composer.json": buildComposerContent("1.0"), "README.md": readme})
		assertValidPackage(t, data, "1.0", "gitea-composer-package.1.0.tar.bz2")
	})

	t.Run("ValidTarGz", func(t *testing.T) {
		data := createArchiveTar(func(w io.Writer) io.WriteCloser {
			return gzip.NewWriter(w)
		}, map[string]string{"composer.json": buildComposerContent(""), "README.md": readme})
		assertValidPackage(t, data, "", "gitea-composer-package.tar.gz")
	})
}
