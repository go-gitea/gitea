// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package chef

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	packageName          = "gitea"
	packageVersion       = "1.0.1"
	packageAuthor        = "KN4CK3R"
	packageDescription   = "Package Description"
	packageRepositoryURL = "https://gitea.io/gitea/gitea"
)

func TestParsePackage(t *testing.T) {
	t.Run("MissingMetadataFile", func(t *testing.T) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(zw)
		tw.Close()
		zw.Close()

		p, err := ParsePackage(&buf)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrMissingMetadataFile)
	})

	t.Run("Valid", func(t *testing.T) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(zw)

		content := `{"name":"` + packageName + `","version":"` + packageVersion + `"}`

		hdr := &tar.Header{
			Name: packageName + "/metadata.json",
			Mode: 0o600,
			Size: int64(len(content)),
		}
		tw.WriteHeader(hdr)
		tw.Write([]byte(content))

		tw.Close()
		zw.Close()

		p, err := ParsePackage(&buf)
		assert.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, packageName, p.Name)
		assert.Equal(t, packageVersion, p.Version)
		assert.NotNil(t, p.Metadata)
	})
}

func TestParseChefMetadata(t *testing.T) {
	t.Run("InvalidName", func(t *testing.T) {
		for _, name := range []string{" test", "test "} {
			p, err := ParseChefMetadata(strings.NewReader(`{"name":"` + name + `","version":"1.0.0"}`))
			assert.Nil(t, p)
			assert.ErrorIs(t, err, ErrInvalidName)
		}
	})

	t.Run("InvalidVersion", func(t *testing.T) {
		for _, version := range []string{"1", "1.2.3.4", "1.0.0 "} {
			p, err := ParseChefMetadata(strings.NewReader(`{"name":"test","version":"` + version + `"}`))
			assert.Nil(t, p)
			assert.ErrorIs(t, err, ErrInvalidVersion)
		}
	})

	t.Run("Valid", func(t *testing.T) {
		p, err := ParseChefMetadata(strings.NewReader(`{"name":"` + packageName + `","version":"` + packageVersion + `","description":"` + packageDescription + `","maintainer":"` + packageAuthor + `","source_url":"` + packageRepositoryURL + `"}`))
		assert.NotNil(t, p)
		assert.NoError(t, err)

		assert.Equal(t, packageName, p.Name)
		assert.Equal(t, packageVersion, p.Version)
		assert.Equal(t, packageDescription, p.Metadata.Description)
		assert.Equal(t, packageAuthor, p.Metadata.Author)
		assert.Equal(t, packageRepositoryURL, p.Metadata.RepositoryURL)
	})
}
