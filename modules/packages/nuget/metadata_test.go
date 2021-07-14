// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package nuget

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	id                = "System.Gitea"
	semver            = "1.0.1"
	authors           = "Gitea Authors"
	projectURL        = "https://gitea.com"
	description       = "Package Description"
	summary           = "Package Summary"
	releaseNotes      = "Package Release Notes"
	repositoryURL     = "https://gitea.com/gitea/gitea"
	targetFramework   = ".NETStandard2.1"
	dependencyID      = "System.Text.Json"
	dependencyVersion = "5.0.0"
)

const nuspecContent = `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://schemas.microsoft.com/packaging/2013/05/nuspec.xsd">
  <metadata>
    <id>` + id + `</id>
    <version>` + semver + `</version>
    <authors>` + authors + `</authors>
    <requireLicenseAcceptance>true</requireLicenseAcceptance>
    <projectUrl>` + projectURL + `</projectUrl>
    <description>` + description + `</description>
    <summary>` + summary + `</summary>
    <releaseNotes>` + releaseNotes + `</releaseNotes>
    <repository url="` + repositoryURL + `" />
    <dependencies>
      <group targetFramework="` + targetFramework + `">
        <dependency id="` + dependencyID + `" version="` + dependencyVersion + `" exclude="Build,Analyzers" />
      </group>
    </dependencies>
  </metadata>
</package>`

func TestParsePackageMetaData(t *testing.T) {
	createArchive := func(name, content string) []byte {
		var buf bytes.Buffer
		archive := zip.NewWriter(&buf)
		w, _ := archive.Create(name)
		w.Write([]byte(content))
		archive.Close()
		return buf.Bytes()
	}

	t.Run("MissingNuspecFile", func(t *testing.T) {
		data := createArchive("dummy.txt", "")

		m, err := ParsePackageMetaData(bytes.NewReader(data), int64(len(data)))
		assert.Nil(t, m)
		assert.ErrorIs(t, err, ErrMissingNuspecFile)
	})

	t.Run("MissingNuspecFileInRoot", func(t *testing.T) {
		data := createArchive("sub/package.nuspec", "")

		m, err := ParsePackageMetaData(bytes.NewReader(data), int64(len(data)))
		assert.Nil(t, m)
		assert.ErrorIs(t, err, ErrMissingNuspecFile)
	})

	t.Run("InvalidNuspecFile", func(t *testing.T) {
		data := createArchive("package.nuspec", "")

		m, err := ParsePackageMetaData(bytes.NewReader(data), int64(len(data)))
		assert.Nil(t, m)
		assert.Error(t, err)
	})

	t.Run("InvalidPackageId", func(t *testing.T) {
		data := createArchive("package.nuspec", `<?xml version="1.0" encoding="utf-8"?>
		<package xmlns="http://schemas.microsoft.com/packaging/2013/05/nuspec.xsd">
		  <metadata></metadata>
		</package>`)

		m, err := ParsePackageMetaData(bytes.NewReader(data), int64(len(data)))
		assert.Nil(t, m)
		assert.ErrorIs(t, err, ErrNuspecInvalidID)
	})

	t.Run("InvalidPackageVersion", func(t *testing.T) {
		data := createArchive("package.nuspec", `<?xml version="1.0" encoding="utf-8"?>
		<package xmlns="http://schemas.microsoft.com/packaging/2013/05/nuspec.xsd">
		  <metadata>
			<id>`+id+`</id>
		  </metadata>
		</package>`)

		m, err := ParsePackageMetaData(bytes.NewReader(data), int64(len(data)))
		assert.Nil(t, m)
		assert.ErrorIs(t, err, ErrNuspecInvalidVersion)
	})

	t.Run("Valid", func(t *testing.T) {
		data := createArchive("package.nuspec", nuspecContent)

		m, err := ParsePackageMetaData(bytes.NewReader(data), int64(len(data)))
		assert.NoError(t, err)
		assert.NotNil(t, m)
	})
}

func TestParseNuspecMetaData(t *testing.T) {
	m, err := ParseNuspecMetaData(strings.NewReader(nuspecContent))
	assert.NoError(t, err)
	assert.NotNil(t, m)

	assert.Equal(t, id, m.ID)
	assert.Equal(t, semver, m.Version)
	assert.Equal(t, authors, m.Authors)
	assert.Equal(t, projectURL, m.ProjectURL)
	assert.Equal(t, description, m.Description)
	assert.Equal(t, summary, m.Summary)
	assert.Equal(t, releaseNotes, m.ReleaseNotes)
	assert.Equal(t, repositoryURL, m.RepositoryURL)
	assert.Len(t, m.Dependencies, 1)
	assert.Contains(t, m.Dependencies, targetFramework)
	deps := m.Dependencies[targetFramework]
	assert.Len(t, deps, 1)
	assert.Equal(t, dependencyID, deps[0].ID)
	assert.Equal(t, dependencyVersion, deps[0].Version)
}
