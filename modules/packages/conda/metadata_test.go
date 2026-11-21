// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conda

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"gitea.dev/modules/packages"
	"gitea.dev/modules/test"
	"gitea.dev/modules/zstd"

	"github.com/dsnet/compress/bzip2"
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

func TestParsePackage(t *testing.T) {
	createBZ2 := func(files map[string]string) *bytes.Reader {
		return bytes.NewReader(test.WriteTarCompression(func(w io.Writer) io.WriteCloser {
			bw, _ := bzip2.NewWriter(w, nil)
			return bw
		}, files).Bytes())
	}

	createConda := func(files map[string]string) *bytes.Reader {
		return bytes.NewReader(test.WriteZipArchive(map[string]string{"info-x.tar.zst": test.WriteTarCompression(func(w io.Writer) io.WriteCloser {
			zw, _ := zstd.NewWriter(w)
			return zw
		}, files).String()}).Bytes())
	}

	t.Run("MissingIndexFile", func(t *testing.T) {
		buf := test.WriteTarArchive(map[string]string{"dummy.txt": ""})

		p, err := parsePackageTar(buf)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrInvalidStructure)
	})

	t.Run("MissingAboutFile", func(t *testing.T) {
		buf := test.WriteTarArchive(map[string]string{"info/index.json": `{"name":"name","version":"1.0"}`})

		p, err := parsePackageTar(buf)
		assert.NotNil(t, p)
		assert.NoError(t, err)

		assert.Equal(t, "name", p.Name)
		assert.Equal(t, "1.0", p.Version)
		assert.Empty(t, p.VersionMetadata.ProjectURL)
	})

	t.Run("InvalidName", func(t *testing.T) {
		for _, name := range []string{"", "name!", "nAMe"} {
			buf := test.WriteTarArchive(map[string]string{"info/index.json": `{"name":"` + name + `","version":"1.0"}`})

			p, err := parsePackageTar(buf)
			assert.Nil(t, p)
			assert.ErrorIs(t, err, ErrInvalidName)
		}
	})

	t.Run("InvalidVersion", func(t *testing.T) {
		for _, version := range []string{"", "1.0-2"} {
			buf := test.WriteTarArchive(map[string]string{"info/index.json": `{"name":"name","version":"` + version + `"}`})

			p, err := parsePackageTar(buf)
			assert.Nil(t, p)
			assert.ErrorIs(t, err, ErrInvalidVersion)
		}
	})

	t.Run("Valid", func(t *testing.T) {
		buf := test.WriteTarArchive(map[string]string{
			"info/index.json": `{"name":"` + packageName + `","version":"` + packageVersion + `","subdir":"linux-64"}`,
			"info/about.json": `{"description":"` + description + `","dev_url":"` + repositoryURL + `","doc_url":"` + documentationURL + `","home":"` + projectURL + `"}`,
		})

		p, err := parsePackageTar(buf)
		assert.NotNil(t, p)
		assert.NoError(t, err)

		assert.Equal(t, packageName, p.Name)
		assert.Equal(t, packageVersion, p.Version)
		assert.Equal(t, "linux-64", p.Subdir)
		assert.Equal(t, description, p.VersionMetadata.Description)
		assert.Equal(t, projectURL, p.VersionMetadata.ProjectURL)
		assert.Equal(t, repositoryURL, p.VersionMetadata.RepositoryURL)
		assert.Equal(t, documentationURL, p.VersionMetadata.DocumentationURL)
	})

	t.Run("DecompressionLimit", func(t *testing.T) {
		defer test.MockVariableValue(&packages.ArchiveDecompressionFloor, 0)()
		files := map[string]string{"dummy.txt": strings.Repeat("\x00", 1024*1024)}

		_, err := ParsePackageBZ2(createBZ2(files))
		assert.ErrorIs(t, err, packages.ErrContentTooLarge)

		br := createConda(files)
		_, err = ParsePackageConda(br, br.Size())
		assert.ErrorIs(t, err, packages.ErrContentTooLarge)
	})

	t.Run(".tar.bz2", func(t *testing.T) {
		p, err := ParsePackageBZ2(createBZ2(map[string]string{
			"info/index.json": `{"name":"` + packageName + `","version":"` + packageVersion + `"}`,
		}))
		assert.NotNil(t, p)
		assert.NoError(t, err)

		assert.Equal(t, packageName, p.Name)
		assert.Equal(t, packageVersion, p.Version)
		assert.False(t, p.FileMetadata.IsCondaPackage)
	})

	t.Run(".conda", func(t *testing.T) {
		br := createConda(map[string]string{
			"info/index.json": `{"name":"` + packageName + `","version":"` + packageVersion + `"}`,
		})

		p, err := ParsePackageConda(br, br.Size())
		assert.NotNil(t, p)
		assert.NoError(t, err)

		assert.Equal(t, packageName, p.Name)
		assert.Equal(t, packageVersion, p.Version)
		assert.True(t, p.FileMetadata.IsCondaPackage)
	})
}
