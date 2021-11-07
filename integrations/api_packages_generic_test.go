// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"

	"github.com/stretchr/testify/assert"
)

func TestPackageGeneric(t *testing.T) {
	defer prepareTestEnv(t)()
	repository := db.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	user := db.AssertExistsAndLoadBean(t, &models.User{ID: repository.OwnerID}).(*models.User)
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)

	packageName := "te-st_pac.kage"
	packageVersion := "1.0.3"
	filename := "fi-le_na.me"
	content := []byte{1, 2, 3}

	url := fmt.Sprintf("/api/v1/repos/%s/%s/packages/generic/%s/%s/%s?token=%s", user.Name, repository.Name, packageName, packageVersion, filename, token)

	t.Run("Upload", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequestWithBody(t, "PUT", url, bytes.NewReader(content))
		MakeRequest(t, req, http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageType(repository.ID, packages.TypeGeneric)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(pvs[0])
		assert.NoError(t, err)
		assert.NotNil(t, pd.SemVer)
		assert.Nil(t, pd.Metadata)
		assert.Equal(t, packageName, pd.Package.Name)
		assert.Equal(t, packageVersion, pd.Version.Version)

		pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, filename, pfs[0].Name)
		assert.True(t, pfs[0].IsLead)

		pb, err := packages.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
		assert.NoError(t, err)
		assert.Equal(t, int64(len(content)), pb.Size)
	})

	t.Run("UploadExists", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequestWithBody(t, "PUT", url, bytes.NewReader(content))
		MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("Download", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", url)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, content, resp.Body.Bytes())

		pvs, err := packages.GetVersionsByPackageType(repository.ID, packages.TypeGeneric)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)
		assert.Equal(t, int64(1), pvs[0].DownloadCount)
	})

	t.Run("Delete", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "DELETE", url)
		MakeRequest(t, req, http.StatusOK)

		pvs, err := packages.GetVersionsByPackageType(repository.ID, packages.TypeGeneric)
		assert.NoError(t, err)
		assert.Empty(t, pvs)
	})

	t.Run("DownloadNotExists", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", url)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("DeleteNotExists", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "DELETE", url)
		MakeRequest(t, req, http.StatusNotFound)
	})
}
