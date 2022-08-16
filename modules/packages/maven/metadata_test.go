// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package maven

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
}
