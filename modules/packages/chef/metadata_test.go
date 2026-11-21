// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package chef

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
	packageName          = "gitea"
	packageVersion       = "1.0.1"
	packageAuthor        = "KN4CK3R"
	packageDescription   = "Package Description"
	packageRepositoryURL = "https://gitea.io/gitea/gitea"
)

func TestParsePackage(t *testing.T) {
	for _, name := range []string{"MissingMetadataFile", "Valid"} {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{}
			if name == "Valid" {
				files[packageName+"/metadata.json"] = `{"name":"` + packageName + `","version":"` + packageVersion + `"}`
			}

			p, err := ParsePackage(bytes.NewReader(test.WriteTarCompression(gzip.NewWriter, files).Bytes()))
			if name == "MissingMetadataFile" {
				assert.Nil(t, p)
				assert.ErrorIs(t, err, ErrMissingMetadataFile)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, p)
				assert.Equal(t, packageName, p.Name)
				assert.Equal(t, packageVersion, p.Version)
				assert.NotNil(t, p.Metadata)
			}
		})
	}

	t.Run("DecompressionLimit", func(t *testing.T) {
		defer test.MockVariableValue(&packages.ArchiveDecompressionFloor, 0)()
		_, err := ParsePackage(bytes.NewReader(test.WriteTarCompression(gzip.NewWriter, map[string]string{"dummy.txt": strings.Repeat("\x00", 1024*1024)}).Bytes()))
		assert.ErrorIs(t, err, packages.ErrContentTooLarge)
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
