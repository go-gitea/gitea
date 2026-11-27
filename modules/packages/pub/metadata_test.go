// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pub

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"

	"gitea.dev/modules/packages"
	"gitea.dev/modules/test"

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
	createArchive := func(files map[string]string) *bytes.Reader {
		return bytes.NewReader(test.WriteTarCompression(gzip.NewWriter, files).Bytes())
	}

	t.Run("MissingPubspecFile", func(t *testing.T) {
		data := createArchive(map[string]string{"dummy.txt": ""})

		pp, err := ParsePackage(data)
		assert.Nil(t, pp)
		assert.ErrorIs(t, err, ErrMissingPubspecFile)
	})

	t.Run("DecompressionLimit", func(t *testing.T) {
		defer test.MockVariableValue(&packages.ArchiveDecompressionFloor, 0)()
		_, err := ParsePackage(createArchive(map[string]string{"dummy.txt": strings.Repeat("\x00", 1024*1024)}))
		assert.ErrorIs(t, err, packages.ErrContentTooLarge)
	})

	t.Run("PubspecFileTooLarge", func(t *testing.T) {
		data := createArchive(map[string]string{"pubspec.yaml": strings.Repeat("\x00", 200*1024)})

		pp, err := ParsePackage(data)
		assert.Nil(t, pp)
		assert.ErrorIs(t, err, ErrPubspecFileTooLarge)
	})

	t.Run("InvalidPubspecFile", func(t *testing.T) {
		data := createArchive(map[string]string{"pubspec.yaml": ""})

		pp, err := ParsePackage(data)
		assert.Nil(t, pp)
		assert.Error(t, err)
	})

	t.Run("Valid", func(t *testing.T) {
		data := createArchive(map[string]string{"pubspec.yaml": pubspecContent})

		pp, err := ParsePackage(data)
		assert.NoError(t, err)
		assert.NotNil(t, pp)
		assert.Empty(t, pp.Metadata.Readme)
	})

	t.Run("ValidWithReadme", func(t *testing.T) {
		data := createArchive(map[string]string{"pubspec.yaml": pubspecContent, "README.md": "readme"})

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
