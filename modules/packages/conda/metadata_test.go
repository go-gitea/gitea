// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conda

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"io"
	"testing"

	"github.com/dsnet/compress/bzip2"
	"github.com/klauspost/compress/zstd"
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
	createArchive := func(files map[string][]byte) *bytes.Buffer {
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
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
		return &buf
	}

	t.Run("MissingIndexFile", func(t *testing.T) {
		buf := createArchive(map[string][]byte{"dummy.txt": {}})

		p, err := parsePackageTar(buf)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrInvalidStructure)
	})

	t.Run("MissingAboutFile", func(t *testing.T) {
		buf := createArchive(map[string][]byte{"info/index.json": []byte(`{"name":"name","version":"1.0"}`)})

		p, err := parsePackageTar(buf)
		assert.NotNil(t, p)
		assert.NoError(t, err)

		assert.Equal(t, "name", p.Name)
		assert.Equal(t, "1.0", p.Version)
		assert.Empty(t, p.VersionMetadata.ProjectURL)
	})

	t.Run("InvalidName", func(t *testing.T) {
		for _, name := range []string{"", "name!", "nAMe"} {
			buf := createArchive(map[string][]byte{"info/index.json": []byte(`{"name":"` + name + `","version":"1.0"}`)})

			p, err := parsePackageTar(buf)
			assert.Nil(t, p)
			assert.ErrorIs(t, err, ErrInvalidName)
		}
	})

	t.Run("InvalidVersion", func(t *testing.T) {
		for _, version := range []string{"", "1.0-2"} {
			buf := createArchive(map[string][]byte{"info/index.json": []byte(`{"name":"name","version":"` + version + `"}`)})

			p, err := parsePackageTar(buf)
			assert.Nil(t, p)
			assert.ErrorIs(t, err, ErrInvalidVersion)
		}
	})

	t.Run("Valid", func(t *testing.T) {
		buf := createArchive(map[string][]byte{
			"info/index.json": []byte(`{"name":"` + packageName + `","version":"` + packageVersion + `","subdir":"linux-64"}`),
			"info/about.json": []byte(`{"description":"` + description + `","dev_url":"` + repositoryURL + `","doc_url":"` + documentationURL + `","home":"` + projectURL + `"}`),
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

	t.Run(".tar.bz2", func(t *testing.T) {
		tarArchive := createArchive(map[string][]byte{
			"info/index.json": []byte(`{"name":"` + packageName + `","version":"` + packageVersion + `"}`),
		})

		var buf bytes.Buffer
		bw, _ := bzip2.NewWriter(&buf, nil)
		io.Copy(bw, tarArchive)
		bw.Close()

		br := bytes.NewReader(buf.Bytes())

		p, err := ParsePackageBZ2(br)
		assert.NotNil(t, p)
		assert.NoError(t, err)

		assert.Equal(t, packageName, p.Name)
		assert.Equal(t, packageVersion, p.Version)
		assert.False(t, p.FileMetadata.IsCondaPackage)
	})

	t.Run(".conda", func(t *testing.T) {
		tarArchive := createArchive(map[string][]byte{
			"info/index.json": []byte(`{"name":"` + packageName + `","version":"` + packageVersion + `"}`),
		})

		var infoBuf bytes.Buffer
		zsw, _ := zstd.NewWriter(&infoBuf)
		io.Copy(zsw, tarArchive)
		zsw.Close()

		var buf bytes.Buffer
		zpw := zip.NewWriter(&buf)
		w, _ := zpw.Create("info-x.tar.zst")
		w.Write(infoBuf.Bytes())
		zpw.Close()

		br := bytes.NewReader(buf.Bytes())

		p, err := ParsePackageConda(br, int64(br.Len()))
		assert.NotNil(t, p)
		assert.NoError(t, err)

		assert.Equal(t, packageName, p.Name)
		assert.Equal(t, packageVersion, p.Version)
		assert.True(t, p.FileMetadata.IsCondaPackage)
	})
}
