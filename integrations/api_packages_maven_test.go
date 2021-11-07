// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/packages/maven"

	"github.com/stretchr/testify/assert"
)

func TestPackageMaven(t *testing.T) {
	defer prepareTestEnv(t)()
	repository := db.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	user := db.AssertExistsAndLoadBean(t, &models.User{ID: repository.OwnerID}).(*models.User)

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

		pvs, err := packages.GetVersionsByPackageType(repository.ID, packages.TypeMaven)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(pvs[0])
		assert.NoError(t, err)
		assert.Nil(t, pd.SemVer)
		assert.Nil(t, pd.Metadata)
		assert.Equal(t, packageName, pd.Package.Name)
		assert.Equal(t, packageVersion, pd.Version.Version)

		pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, filename, pfs[0].Name)
		assert.False(t, pfs[0].IsLead)

		pb, err := packages.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
		assert.NoError(t, err)
		assert.Equal(t, int64(4), pb.Size)
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

		pvs, err := packages.GetVersionsByPackageType(repository.ID, packages.TypeMaven)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)
		assert.Equal(t, int64(0), pvs[0].DownloadCount)
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

	pomContent := `<?xml version="1.0"?>
<project xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
  <groupId>` + groupID + `</groupId>
  <artifactId>` + artifactID + `</artifactId>
  <version>` + packageVersion + `</version>
  <description>` + packageDescription + `</description>
</project>`

	t.Run("UploadPOM", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		pvs, err := packages.GetVersionsByPackageType(repository.ID, packages.TypeMaven)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(pvs[0])
		assert.NoError(t, err)
		assert.Nil(t, pd.Metadata)

		putFile(t, fmt.Sprintf("/%s/%s.pom", packageVersion, filename), pomContent, http.StatusCreated)

		pvs, err = packages.GetVersionsByPackageType(repository.ID, packages.TypeMaven)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err = packages.GetPackageDescriptor(pvs[0])
		assert.NoError(t, err)
		assert.IsType(t, &maven.Metadata{}, pd.Metadata)
		assert.Equal(t, packageDescription, pd.Metadata.(*maven.Metadata).Description)

		pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 2)
		i := 0
		if strings.HasSuffix(pfs[1].Name, ".pom") {
			i = 1
		}
		assert.Equal(t, filename+".pom", pfs[i].Name)
		assert.True(t, pfs[i].IsLead)
	})

	t.Run("DownloadPOM", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s.pom", root, packageVersion, filename))
		req = AddBasicAuthHeader(req, user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, []byte(pomContent), resp.Body.Bytes())

		pvs, err := packages.GetVersionsByPackageType(repository.ID, packages.TypeMaven)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)
		assert.Equal(t, int64(1), pvs[0].DownloadCount)
	})
}
