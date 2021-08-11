// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
)

func TestPackageGeneric(t *testing.T) {
	defer prepareTestEnv(t)()
	repository := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: repository.OwnerID}).(*models.User)
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

		ps, err := models.GetPackagesByRepositoryAndType(repository.ID, models.PackageGeneric)
		assert.NoError(t, err)
		assert.Len(t, ps, 1)
		assert.Equal(t, packageName, ps[0].Name)
		assert.Equal(t, packageVersion, ps[0].Version)

		pfs, err := ps[0].GetFiles()
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, filename, pfs[0].Name)
		assert.Equal(t, int64(len(content)), pfs[0].Size)
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
	})

	t.Run("Delete", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "DELETE", url)
		MakeRequest(t, req, http.StatusOK)

		ps, err := models.GetPackagesByRepositoryAndType(repository.ID, models.PackageGeneric)
		assert.NoError(t, err)
		assert.Empty(t, ps)
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
