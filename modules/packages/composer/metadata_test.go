// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package composer

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
)

const (
	name        = "gitea/composer-package"
	description = "Package Description"
	packageType = "composer-plugin"
	author      = "Gitea Authors"
	email       = "no.reply@gitea.io"
	homepage    = "https://gitea.io"
	license     = "MIT"
)

const composerContent = `{
    "name": "` + name + `",
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
    }
}`

func TestLicenseUnmarshal(t *testing.T) {
	var l Licenses
	assert.NoError(t, json.NewDecoder(strings.NewReader(`["MIT"]`)).Decode(&l))
	assert.Len(t, l, 1)
	assert.Equal(t, "MIT", l[0])
	assert.NoError(t, json.NewDecoder(strings.NewReader(`"MIT"`)).Decode(&l))
	assert.Len(t, l, 1)
	assert.Equal(t, "MIT", l[0])
}

func TestParsePackage(t *testing.T) {
	createArchive := func(name, content string) []byte {
		var buf bytes.Buffer
		archive := zip.NewWriter(&buf)
		w, _ := archive.Create(name)
		w.Write([]byte(content))
		archive.Close()
		return buf.Bytes()
	}

	t.Run("MissingComposerFile", func(t *testing.T) {
		data := createArchive("dummy.txt", "")

		cp, err := ParsePackage(bytes.NewReader(data), int64(len(data)))
		assert.Nil(t, cp)
		assert.ErrorIs(t, err, ErrMissingComposerFile)
	})

	t.Run("MissingComposerFileInRoot", func(t *testing.T) {
		data := createArchive("sub/sub/composer.json", "")

		cp, err := ParsePackage(bytes.NewReader(data), int64(len(data)))
		assert.Nil(t, cp)
		assert.ErrorIs(t, err, ErrMissingComposerFile)
	})

	t.Run("InvalidComposerFile", func(t *testing.T) {
		data := createArchive("composer.json", "")

		cp, err := ParsePackage(bytes.NewReader(data), int64(len(data)))
		assert.Nil(t, cp)
		assert.Error(t, err)
	})

	t.Run("Valid", func(t *testing.T) {
		data := createArchive("composer.json", composerContent)

		cp, err := ParsePackage(bytes.NewReader(data), int64(len(data)))
		assert.NoError(t, err)
		assert.NotNil(t, cp)
	})
}

func TestParseComposerFile(t *testing.T) {
	t.Run("InvalidPackageName", func(t *testing.T) {
		cp, err := ParseComposerFile(strings.NewReader(`{}`))
		assert.Nil(t, cp)
		assert.ErrorIs(t, err, ErrInvalidName)
	})

	t.Run("InvalidPackageVersion", func(t *testing.T) {
		cp, err := ParseComposerFile(strings.NewReader(`{"name": "gitea/composer-package", "version": "1.a.3"}`))
		assert.Nil(t, cp)
		assert.ErrorIs(t, err, ErrInvalidVersion)
	})

	t.Run("Valid", func(t *testing.T) {
		cp, err := ParseComposerFile(strings.NewReader(composerContent))
		assert.NoError(t, err)
		assert.NotNil(t, cp)

		assert.Equal(t, name, cp.Name)
		assert.Empty(t, cp.Version)
		assert.Equal(t, description, cp.Metadata.Description)
		assert.Len(t, cp.Metadata.Authors, 1)
		assert.Equal(t, author, cp.Metadata.Authors[0].Name)
		assert.Equal(t, email, cp.Metadata.Authors[0].Email)
		assert.Equal(t, homepage, cp.Metadata.Homepage)
		assert.Equal(t, packageType, cp.Type)
		assert.Len(t, cp.Metadata.License, 1)
		assert.Equal(t, license, cp.Metadata.License[0])
	})
}
