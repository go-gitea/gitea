// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	container_model "code.gitea.io/gitea/models/packages/container"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	packages_service "code.gitea.io/gitea/services/packages"

	"github.com/stretchr/testify/assert"
)

func TestPackageAPI(t *testing.T) {
	defer prepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}).(*user_model.User)
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)

	packageName := "test-package"
	packageVersion := "1.0.3"
	filename := "file.bin"

	url := fmt.Sprintf("/api/packages/%s/generic/%s/%s/%s", user.Name, packageName, packageVersion, filename)
	req := NewRequestWithBody(t, "PUT", url, bytes.NewReader([]byte{}))
	AddBasicAuthHeader(req, user.Name)
	MakeRequest(t, req, http.StatusCreated)

	t.Run("ListPackages", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s?token=%s", user.Name, token))
		resp := MakeRequest(t, req, http.StatusOK)

		var apiPackages []*api.Package
		DecodeJSON(t, resp, &apiPackages)

		assert.Len(t, apiPackages, 1)
		assert.Equal(t, string(packages_model.TypeGeneric), apiPackages[0].Type)
		assert.Equal(t, packageName, apiPackages[0].Name)
		assert.Equal(t, packageVersion, apiPackages[0].Version)
		assert.NotNil(t, apiPackages[0].Creator)
		assert.Equal(t, user.Name, apiPackages[0].Creator.UserName)
	})

	t.Run("GetPackage", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s/dummy/%s/%s?token=%s", user.Name, packageName, packageVersion, token))
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s/generic/%s/%s?token=%s", user.Name, packageName, packageVersion, token))
		resp := MakeRequest(t, req, http.StatusOK)

		var p *api.Package
		DecodeJSON(t, resp, &p)

		assert.Equal(t, string(packages_model.TypeGeneric), p.Type)
		assert.Equal(t, packageName, p.Name)
		assert.Equal(t, packageVersion, p.Version)
		assert.NotNil(t, p.Creator)
		assert.Equal(t, user.Name, p.Creator.UserName)
	})

	t.Run("ListPackageFiles", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s/dummy/%s/%s/files?token=%s", user.Name, packageName, packageVersion, token))
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s/generic/%s/%s/files?token=%s", user.Name, packageName, packageVersion, token))
		resp := MakeRequest(t, req, http.StatusOK)

		var files []*api.PackageFile
		DecodeJSON(t, resp, &files)

		assert.Len(t, files, 1)
		assert.Equal(t, int64(0), files[0].Size)
		assert.Equal(t, filename, files[0].Name)
		assert.Equal(t, "d41d8cd98f00b204e9800998ecf8427e", files[0].HashMD5)
		assert.Equal(t, "da39a3ee5e6b4b0d3255bfef95601890afd80709", files[0].HashSHA1)
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", files[0].HashSHA256)
		assert.Equal(t, "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e", files[0].HashSHA512)
	})

	t.Run("DeletePackage", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/packages/%s/dummy/%s/%s?token=%s", user.Name, packageName, packageVersion, token))
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/packages/%s/generic/%s/%s?token=%s", user.Name, packageName, packageVersion, token))
		MakeRequest(t, req, http.StatusNoContent)
	})
}

func TestPackageCleanup(t *testing.T) {
	defer prepareTestEnv(t)()

	time.Sleep(time.Second)

	pbs, err := packages_model.FindExpiredUnreferencedBlobs(db.DefaultContext, time.Duration(0))
	assert.NoError(t, err)
	assert.NotEmpty(t, pbs)

	_, err = packages_model.GetInternalVersionByNameAndVersion(db.DefaultContext, 2, packages_model.TypeContainer, "test", container_model.UploadVersion)
	assert.NoError(t, err)

	err = packages_service.Cleanup(nil, time.Duration(0))
	assert.NoError(t, err)

	pbs, err = packages_model.FindExpiredUnreferencedBlobs(db.DefaultContext, time.Duration(0))
	assert.NoError(t, err)
	assert.Empty(t, pbs)

	_, err = packages_model.GetInternalVersionByNameAndVersion(db.DefaultContext, 2, packages_model.TypeContainer, "test", container_model.UploadVersion)
	assert.ErrorIs(t, err, packages_model.ErrPackageNotExist)
}
