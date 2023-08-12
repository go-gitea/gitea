// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package maven

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/encoding/charmap"
)

const (
	groupID              = "org.gitea"
	artifactID           = "my-project"
	version              = "1.0.1"
	name                 = "My Gitea Project"
	description          = "Package Description"
	projectURL           = "https://gitea.io"
	license              = "MIT"
	dependencyGroupID    = "org.gitea.core"
	dependencyArtifactID = "git"
	dependencyVersion    = "5.0.0"
)

const pomContent = `<?xml version="1.0"?>
<project xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
  <groupId>` + groupID + `</groupId>
  <artifactId>` + artifactID + `</artifactId>
  <version>` + version + `</version>
  <name>` + name + `</name>
  <description>` + description + `</description>
  <url>` + projectURL + `</url>
  <licenses>
    <license>
      <name>` + license + `</name>
    </license>
  </licenses>
  <dependencies>
    <dependency>
      <groupId>` + dependencyGroupID + `</groupId>
      <artifactId>` + dependencyArtifactID + `</artifactId>
      <version>` + dependencyVersion + `</version>
    </dependency>
  </dependencies>
</project>`

func TestParsePackageMetaData(t *testing.T) {
	t.Run("InvalidFile", func(t *testing.T) {
		m, err := ParsePackageMetaData(strings.NewReader(""))
		assert.Nil(t, m)
		assert.Error(t, err)
	})

	t.Run("Valid", func(t *testing.T) {
		m, err := ParsePackageMetaData(strings.NewReader(pomContent))
		assert.NoError(t, err)
		assert.NotNil(t, m)

		assert.Equal(t, groupID, m.GroupID)
		assert.Equal(t, artifactID, m.ArtifactID)
		assert.Equal(t, name, m.Name)
		assert.Equal(t, description, m.Description)
		assert.Equal(t, projectURL, m.ProjectURL)
		assert.Len(t, m.Licenses, 1)
		assert.Equal(t, license, m.Licenses[0])
		assert.Len(t, m.Dependencies, 1)
		assert.Equal(t, dependencyGroupID, m.Dependencies[0].GroupID)
		assert.Equal(t, dependencyArtifactID, m.Dependencies[0].ArtifactID)
		assert.Equal(t, dependencyVersion, m.Dependencies[0].Version)
	})

	t.Run("Encoding", func(t *testing.T) {
		// UTF-8 is default but the metadata could be encoded differently
		pomContent8859_1, err := charmap.ISO8859_1.NewEncoder().String(
			strings.ReplaceAll(
				pomContent,
				`<?xml version="1.0"?>`,
				`<?xml version="1.0" encoding="ISO-8859-1"?>`,
			),
		)
		assert.NoError(t, err)

		m, err := ParsePackageMetaData(strings.NewReader(pomContent8859_1))
		assert.NoError(t, err)
		assert.NotNil(t, m)
	})
}
