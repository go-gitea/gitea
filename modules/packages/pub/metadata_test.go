// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pub

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	packageName      = "gitea"
	packageVersion   = "1.0.1"
	description      = "Package Description"
	projectURL       = "https://gitea.com"
	repositoryURL    = "https://gitea.com/gitea/gitea"
	documentationURL = "https://docs.gitea.com"
)

const pubspecContent = `name: ` + packageName + `
version: ` + packageVersion + `
description: ` + description + `
homepage: ` + projectURL + `
repository: ` + repositoryURL + `
documentation: ` + documentationURL + `

environment:
  sdk: '>=2.16.0 <3.0.0'

dependencies:
  flutter:
    sdk: flutter
  path: '>=1.8.0 <3.0.0'

dev_dependencies:
  http: '>=0.13.0'`

func TestParsePackage(t *testing.T) {
	createArchive := func(files map[string][]byte) io.Reader {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(zw)
		for filename, content := range files {
			hdr := &tar.Header{
				Name: filename,
				Mode: 0o600,
				Size: int64(len(content)),
			}
			tw.WriteHeader(hdr)
			tw.Write(content)
		}
		tw.Close()
		zw.Close()
		return &buf
	}

	t.Run("MissingPubspecFile", func(t *testing.T) {
		data := createArchive(map[string][]byte{"dummy.txt": {}})

		pp, err := ParsePackage(data)
		assert.Nil(t, pp)
		assert.ErrorIs(t, err, ErrMissingPubspecFile)
	})

	t.Run("PubspecFileTooLarge", func(t *testing.T) {
		data := createArchive(map[string][]byte{"pubspec.yaml": make([]byte, 200*1024)})

		pp, err := ParsePackage(data)
		assert.Nil(t, pp)
		assert.ErrorIs(t, err, ErrPubspecFileTooLarge)
	})

	t.Run("InvalidPubspecFile", func(t *testing.T) {
		data := createArchive(map[string][]byte{"pubspec.yaml": {}})

		pp, err := ParsePackage(data)
		assert.Nil(t, pp)
		assert.Error(t, err)
	})

	t.Run("Valid", func(t *testing.T) {
		data := createArchive(map[string][]byte{"pubspec.yaml": []byte(pubspecContent)})

		pp, err := ParsePackage(data)
		assert.NoError(t, err)
		assert.NotNil(t, pp)
		assert.Empty(t, pp.Metadata.Readme)
	})

	t.Run("ValidWithReadme", func(t *testing.T) {
		data := createArchive(map[string][]byte{"pubspec.yaml": []byte(pubspecContent), "README.md": []byte("readme")})

		pp, err := ParsePackage(data)
		assert.NoError(t, err)
		assert.NotNil(t, pp)
		assert.Equal(t, "readme", pp.Metadata.Readme)
	})
}

func TestParsePubspecMetadata(t *testing.T) {
	t.Run("InvalidName", func(t *testing.T) {
		for _, name := range []string{"123abc", "ab-cd"} {
			pp, err := ParsePubspecMetadata(strings.NewReader(`name: ` + name))
			assert.Nil(t, pp)
			assert.ErrorIs(t, err, ErrInvalidName)
		}
	})

	t.Run("InvalidVersion", func(t *testing.T) {
		pp, err := ParsePubspecMetadata(strings.NewReader(`name: dummy
version: invalid`))
		assert.Nil(t, pp)
		assert.ErrorIs(t, err, ErrInvalidVersion)
	})

	t.Run("Valid", func(t *testing.T) {
		pp, err := ParsePubspecMetadata(strings.NewReader(pubspecContent))
		assert.NoError(t, err)
		assert.NotNil(t, pp)

		assert.Equal(t, packageName, pp.Name)
		assert.Equal(t, packageVersion, pp.Version)
		assert.Equal(t, description, pp.Metadata.Description)
		assert.Equal(t, projectURL, pp.Metadata.ProjectURL)
		assert.Equal(t, repositoryURL, pp.Metadata.RepositoryURL)
		assert.Equal(t, documentationURL, pp.Metadata.DocumentationURL)
		assert.NotNil(t, pp.Metadata.Pubspec)
	})
}
