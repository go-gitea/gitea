// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/packages/maven"

	"github.com/stretchr/testify/assert"
)

func TestPackageMaven(t *testing.T) {
	defer prepareTestEnv(t)()
	repository := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: repository.OwnerID}).(*models.User)

	groupID := "com.gitea"
	artifactID := "test-project"
	packageName := groupID + "-" + artifactID
	packageVersion := "1.0.1"
	packageDescription := "Test Description"

	root := fmt.Sprintf("/api/v1/repos/%s/%s/packages/maven/%s/%s", user.Name, repository.Name, strings.ReplaceAll(groupID, ".", "/"), artifactID)
	filename := fmt.Sprintf("%s-%s.jar", packageName, packageVersion)

	putFile := func(t *testing.T, path, content string, expectedStatus int) {
		req := NewRequestWithBody(t, "PUT", root+path, strings.NewReader(content))
		req = AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, expectedStatus)
	}

	t.Run("Upload", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		putFile(t, fmt.Sprintf("/%s/%s", packageVersion, filename), "test", http.StatusCreated)
		putFile(t, "/maven-metadata.xml", "test", http.StatusOK)

		ps, err := models.GetPackagesByRepositoryAndType(repository.ID, models.PackageMaven)
		assert.NoError(t, err)
		assert.Len(t, ps, 1)
		assert.Equal(t, packageName, ps[0].Name)
		assert.Equal(t, packageVersion, ps[0].Version)

		pfs, err := ps[0].GetFiles()
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, filename, pfs[0].Name)
		assert.Equal(t, int64(4), pfs[0].Size)
	})

	t.Run("UploadExists", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		putFile(t, fmt.Sprintf("/%s/%s", packageVersion, filename), "test", http.StatusBadRequest)
	})

	t.Run("Download", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s", root, packageVersion, filename))
		req = AddBasicAuthHeader(req, user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, []byte("test"), resp.Body.Bytes())
	})

	t.Run("UploadVerifySHA1", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		t.Run("Missmatch", func(t *testing.T) {
			defer PrintCurrentTest(t)()

			putFile(t, fmt.Sprintf("/%s/%s.sha1", packageVersion, filename), "test", http.StatusBadRequest)
		})
		t.Run("Valid", func(t *testing.T) {
			defer PrintCurrentTest(t)()

			putFile(t, fmt.Sprintf("/%s/%s.sha1", packageVersion, filename), "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3", http.StatusOK)
		})
	})

	t.Run("UploadPOM", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		pomContent := `<?xml version="1.0"?>
<project xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
  <groupId>` + groupID + `</groupId>
  <artifactId>` + artifactID + `</artifactId>
  <version>` + packageVersion + `</version>
  <description>` + packageDescription + `</description>
</project>`

		ps, err := models.GetPackagesByRepositoryAndType(repository.ID, models.PackageMaven)
		assert.NoError(t, err)
		assert.Len(t, ps, 1)

		var m *maven.Metadata
		err = json.Unmarshal([]byte(ps[0].MetadataRaw), &m)
		assert.NoError(t, err)
		assert.Empty(t, m.Description)

		putFile(t, fmt.Sprintf("/%s/%s.pom", packageVersion, filename), pomContent, http.StatusCreated)

		ps, err = models.GetPackagesByRepositoryAndType(repository.ID, models.PackageMaven)
		assert.NoError(t, err)
		assert.Len(t, ps, 1)

		err = json.Unmarshal([]byte(ps[0].MetadataRaw), &m)
		assert.NoError(t, err)
		assert.Equal(t, packageDescription, m.Description)
	})
}
