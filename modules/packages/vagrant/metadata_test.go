// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package vagrant

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
)

const (
	author        = "gitea"
	description   = "Package Description"
	projectURL    = "https://gitea.io"
	repositoryURL = "https://gitea.io/gitea/gitea"
)

func TestParseMetadataFromBox(t *testing.T) {
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

	t.Run("MissingInfoFile", func(t *testing.T) {
		data := createArchive(map[string][]byte{"dummy.txt": {}})

		metadata, err := ParseMetadataFromBox(data)
		assert.NotNil(t, metadata)
		assert.NoError(t, err)
	})

	t.Run("Valid", func(t *testing.T) {
		content, err := json.Marshal(map[string]string{
			"description": description,
			"author":      author,
			"website":     projectURL,
			"repository":  repositoryURL,
		})
		assert.NoError(t, err)

		data := createArchive(map[string][]byte{"info.json": content})

		metadata, err := ParseMetadataFromBox(data)
		assert.NotNil(t, metadata)
		assert.NoError(t, err)

		assert.Equal(t, author, metadata.Author)
		assert.Equal(t, description, metadata.Description)
		assert.Equal(t, projectURL, metadata.ProjectURL)
		assert.Equal(t, repositoryURL, metadata.RepositoryURL)
	})
}

func TestParseInfoFile(t *testing.T) {
	t.Run("UnknownKeys", func(t *testing.T) {
		content, err := json.Marshal(map[string]string{
			"package": "",
			"dummy":   "",
		})
		assert.NoError(t, err)

		metadata, err := ParseInfoFile(bytes.NewReader(content))
		assert.NotNil(t, metadata)
		assert.NoError(t, err)

		assert.Empty(t, metadata.Author)
		assert.Empty(t, metadata.Description)
		assert.Empty(t, metadata.ProjectURL)
		assert.Empty(t, metadata.RepositoryURL)
	})

	t.Run("Valid", func(t *testing.T) {
		content, err := json.Marshal(map[string]string{
			"description": description,
			"author":      author,
			"website":     projectURL,
			"repository":  repositoryURL,
		})
		assert.NoError(t, err)

		metadata, err := ParseInfoFile(bytes.NewReader(content))
		assert.NotNil(t, metadata)
		assert.NoError(t, err)

		assert.Equal(t, author, metadata.Author)
		assert.Equal(t, description, metadata.Description)
		assert.Equal(t, projectURL, metadata.ProjectURL)
		assert.Equal(t, repositoryURL, metadata.RepositoryURL)
	})
}
