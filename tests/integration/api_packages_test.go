// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	container_model "code.gitea.io/gitea/models/packages/container"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	packages_service "code.gitea.io/gitea/services/packages"
	packages_cleanup_service "code.gitea.io/gitea/services/packages/cleanup"
	"code.gitea.io/gitea/tests"

	"github.com/minio/sha256-simd"
	"github.com/stretchr/testify/assert"
)

func TestPackageAPI(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	session := loginUser(t, user.Name)
	tokenReadPackage := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadPackage)
	tokenDeletePackage := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeDeletePackage)

	packageName := "test-package"
	packageVersion := "1.0.3"
	filename := "file.bin"

	url := fmt.Sprintf("/api/packages/%s/generic/%s/%s/%s", user.Name, packageName, packageVersion, filename)
	req := NewRequestWithBody(t, "PUT", url, bytes.NewReader([]byte{}))
	AddBasicAuthHeader(req, user.Name)
	MakeRequest(t, req, http.StatusCreated)

	t.Run("ListPackages", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s?token=%s", user.Name, tokenReadPackage))
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
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s/dummy/%s/%s?token=%s", user.Name, packageName, packageVersion, tokenReadPackage))
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s/generic/%s/%s?token=%s", user.Name, packageName, packageVersion, tokenReadPackage))
		resp := MakeRequest(t, req, http.StatusOK)

		var p *api.Package
		DecodeJSON(t, resp, &p)

		assert.Equal(t, string(packages_model.TypeGeneric), p.Type)
		assert.Equal(t, packageName, p.Name)
		assert.Equal(t, packageVersion, p.Version)
		assert.NotNil(t, p.Creator)
		assert.Equal(t, user.Name, p.Creator.UserName)

		t.Run("RepositoryLink", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			p, err := packages_model.GetPackageByName(db.DefaultContext, user.ID, packages_model.TypeGeneric, packageName)
			assert.NoError(t, err)

			// no repository link
			req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s/generic/%s/%s?token=%s", user.Name, packageName, packageVersion, tokenReadPackage))
			resp := MakeRequest(t, req, http.StatusOK)

			var ap1 *api.Package
			DecodeJSON(t, resp, &ap1)
			assert.Nil(t, ap1.Repository)

			// link to public repository
			assert.NoError(t, packages_model.SetRepositoryLink(db.DefaultContext, p.ID, 1))

			req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s/generic/%s/%s?token=%s", user.Name, packageName, packageVersion, tokenReadPackage))
			resp = MakeRequest(t, req, http.StatusOK)

			var ap2 *api.Package
			DecodeJSON(t, resp, &ap2)
			assert.NotNil(t, ap2.Repository)
			assert.EqualValues(t, 1, ap2.Repository.ID)

			// link to private repository
			assert.NoError(t, packages_model.SetRepositoryLink(db.DefaultContext, p.ID, 2))

			req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s/generic/%s/%s?token=%s", user.Name, packageName, packageVersion, tokenReadPackage))
			resp = MakeRequest(t, req, http.StatusOK)

			var ap3 *api.Package
			DecodeJSON(t, resp, &ap3)
			assert.Nil(t, ap3.Repository)

			assert.NoError(t, packages_model.UnlinkRepositoryFromAllPackages(db.DefaultContext, 2))
		})
	})

	t.Run("ListPackageFiles", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s/dummy/%s/%s/files?token=%s", user.Name, packageName, packageVersion, tokenReadPackage))
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s/generic/%s/%s/files?token=%s", user.Name, packageName, packageVersion, tokenReadPackage))
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
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/packages/%s/dummy/%s/%s?token=%s", user.Name, packageName, packageVersion, tokenDeletePackage))
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/packages/%s/generic/%s/%s?token=%s", user.Name, packageName, packageVersion, tokenDeletePackage))
		MakeRequest(t, req, http.StatusNoContent)
	})
}

func TestPackageAccess(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	inactive := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 9})
	privatedOrg := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 23})

	uploadPackage := func(doer, owner *user_model.User, expectedStatus int) {
		url := fmt.Sprintf("/api/packages/%s/generic/test-package/1.0/file.bin", owner.Name)
		req := NewRequestWithBody(t, "PUT", url, bytes.NewReader([]byte{1}))
		AddBasicAuthHeader(req, doer.Name)
		MakeRequest(t, req, expectedStatus)
	}

	uploadPackage(user, inactive, http.StatusUnauthorized)
	uploadPackage(inactive, inactive, http.StatusUnauthorized)
	uploadPackage(inactive, user, http.StatusUnauthorized)
	uploadPackage(admin, inactive, http.StatusCreated)
	uploadPackage(admin, user, http.StatusCreated)

	// team.authorize is write, but team_unit.access_mode is none
	// so the user can not upload packages or get package list
	uploadPackage(user, privatedOrg, http.StatusUnauthorized)

	session := loginUser(t, user.Name)
	tokenReadPackage := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadPackage)
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s?token=%s", privatedOrg.Name, tokenReadPackage))
	MakeRequest(t, req, http.StatusForbidden)
}

func TestPackageQuota(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	limitTotalOwnerCount, limitTotalOwnerSize := setting.Packages.LimitTotalOwnerCount, setting.Packages.LimitTotalOwnerSize

	// Exceeded quota result in StatusForbidden for normal users but admins are always allowed to upload.
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})

	t.Run("Common", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		limitSizeGeneric := setting.Packages.LimitSizeGeneric

		uploadPackage := func(doer *user_model.User, version string, expectedStatus int) {
			url := fmt.Sprintf("/api/packages/%s/generic/test-package/%s/file.bin", user.Name, version)
			req := NewRequestWithBody(t, "PUT", url, bytes.NewReader([]byte{1}))
			AddBasicAuthHeader(req, doer.Name)
			MakeRequest(t, req, expectedStatus)
		}

		setting.Packages.LimitTotalOwnerCount = 0
		uploadPackage(user, "1.0", http.StatusForbidden)
		uploadPackage(admin, "1.0", http.StatusCreated)
		setting.Packages.LimitTotalOwnerCount = limitTotalOwnerCount

		setting.Packages.LimitTotalOwnerSize = 0
		uploadPackage(user, "1.1", http.StatusForbidden)
		uploadPackage(admin, "1.1", http.StatusCreated)
		setting.Packages.LimitTotalOwnerSize = limitTotalOwnerSize

		setting.Packages.LimitSizeGeneric = 0
		uploadPackage(user, "1.2", http.StatusForbidden)
		uploadPackage(admin, "1.2", http.StatusCreated)
		setting.Packages.LimitSizeGeneric = limitSizeGeneric
	})

	t.Run("Container", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		limitSizeContainer := setting.Packages.LimitSizeContainer

		uploadBlob := func(doer *user_model.User, data string, expectedStatus int) {
			url := fmt.Sprintf("/v2/%s/quota-test/blobs/uploads?digest=sha256:%x", user.Name, sha256.Sum256([]byte(data)))
			req := NewRequestWithBody(t, "POST", url, strings.NewReader(data))
			AddBasicAuthHeader(req, doer.Name)
			MakeRequest(t, req, expectedStatus)
		}

		setting.Packages.LimitTotalOwnerSize = 0
		uploadBlob(user, "2", http.StatusForbidden)
		uploadBlob(admin, "2", http.StatusCreated)
		setting.Packages.LimitTotalOwnerSize = limitTotalOwnerSize

		setting.Packages.LimitSizeContainer = 0
		uploadBlob(user, "3", http.StatusForbidden)
		uploadBlob(admin, "3", http.StatusCreated)
		setting.Packages.LimitSizeContainer = limitSizeContainer
	})
}

func TestPackageCleanup(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	duration, _ := time.ParseDuration("-1h")

	t.Run("Common", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// Upload and delete a generic package and upload a container blob
		data, _ := util.CryptoRandomBytes(5)
		url := fmt.Sprintf("/api/packages/%s/generic/cleanup-test/1.1.1/file.bin", user.Name)
		req := NewRequestWithBody(t, "PUT", url, bytes.NewReader(data))
		AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "DELETE", url)
		AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, http.StatusNoContent)

		data, _ = util.CryptoRandomBytes(5)
		url = fmt.Sprintf("/v2/%s/cleanup-test/blobs/uploads?digest=sha256:%x", user.Name, sha256.Sum256(data))
		req = NewRequestWithBody(t, "POST", url, bytes.NewReader(data))
		AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, http.StatusCreated)

		pbs, err := packages_model.FindExpiredUnreferencedBlobs(db.DefaultContext, duration)
		assert.NoError(t, err)
		assert.NotEmpty(t, pbs)

		_, err = packages_model.GetInternalVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeContainer, "cleanup-test", container_model.UploadVersion)
		assert.NoError(t, err)

		err = packages_cleanup_service.Cleanup(db.DefaultContext, duration)
		assert.NoError(t, err)

		pbs, err = packages_model.FindExpiredUnreferencedBlobs(db.DefaultContext, duration)
		assert.NoError(t, err)
		assert.Empty(t, pbs)

		_, err = packages_model.GetInternalVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeContainer, "cleanup-test", container_model.UploadVersion)
		assert.ErrorIs(t, err, packages_model.ErrPackageNotExist)
	})

	t.Run("CleanupRules", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		type version struct {
			Version     string
			ShouldExist bool
			Created     int64
		}

		cases := []struct {
			Name     string
			Versions []version
			Rule     *packages_model.PackageCleanupRule
		}{
			{
				Name: "Disabled",
				Versions: []version{
					{Version: "keep", ShouldExist: true},
				},
				Rule: &packages_model.PackageCleanupRule{
					Enabled: false,
				},
			},
			{
				Name: "KeepCount",
				Versions: []version{
					{Version: "keep", ShouldExist: true},
					{Version: "v1.0", ShouldExist: true},
					{Version: "test-3", ShouldExist: false, Created: 1},
					{Version: "test-4", ShouldExist: false, Created: 1},
				},
				Rule: &packages_model.PackageCleanupRule{
					Enabled:   true,
					KeepCount: 2,
				},
			},
			{
				Name: "KeepPattern",
				Versions: []version{
					{Version: "keep", ShouldExist: true},
					{Version: "v1.0", ShouldExist: false},
				},
				Rule: &packages_model.PackageCleanupRule{
					Enabled:     true,
					KeepPattern: "k.+p",
				},
			},
			{
				Name: "RemoveDays",
				Versions: []version{
					{Version: "keep", ShouldExist: true},
					{Version: "v1.0", ShouldExist: false, Created: 1},
				},
				Rule: &packages_model.PackageCleanupRule{
					Enabled:    true,
					RemoveDays: 60,
				},
			},
			{
				Name: "RemovePattern",
				Versions: []version{
					{Version: "test", ShouldExist: true},
					{Version: "test-3", ShouldExist: false},
					{Version: "test-4", ShouldExist: false},
				},
				Rule: &packages_model.PackageCleanupRule{
					Enabled:       true,
					RemovePattern: `t[e]+st-\d+`,
				},
			},
			{
				Name: "MatchFullName",
				Versions: []version{
					{Version: "keep", ShouldExist: true},
					{Version: "test", ShouldExist: false},
				},
				Rule: &packages_model.PackageCleanupRule{
					Enabled:       true,
					RemovePattern: `package/test|different/keep`,
					MatchFullName: true,
				},
			},
			{
				Name: "Mixed",
				Versions: []version{
					{Version: "keep", ShouldExist: true, Created: time.Now().Add(time.Duration(10000)).Unix()},
					{Version: "dummy", ShouldExist: true, Created: 1},
					{Version: "test-3", ShouldExist: true},
					{Version: "test-4", ShouldExist: false, Created: 1},
				},
				Rule: &packages_model.PackageCleanupRule{
					Enabled:       true,
					KeepCount:     1,
					KeepPattern:   `dummy`,
					RemoveDays:    7,
					RemovePattern: `t[e]+st-\d+`,
				},
			},
		}

		for _, c := range cases {
			t.Run(c.Name, func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				for _, v := range c.Versions {
					url := fmt.Sprintf("/api/packages/%s/generic/package/%s/file.bin", user.Name, v.Version)
					req := NewRequestWithBody(t, "PUT", url, bytes.NewReader([]byte{1}))
					AddBasicAuthHeader(req, user.Name)
					MakeRequest(t, req, http.StatusCreated)

					if v.Created != 0 {
						pv, err := packages_model.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeGeneric, "package", v.Version)
						assert.NoError(t, err)
						_, err = db.GetEngine(db.DefaultContext).Exec("UPDATE package_version SET created_unix = ? WHERE id = ?", v.Created, pv.ID)
						assert.NoError(t, err)
					}
				}

				c.Rule.OwnerID = user.ID
				c.Rule.Type = packages_model.TypeGeneric

				pcr, err := packages_model.InsertCleanupRule(db.DefaultContext, c.Rule)
				assert.NoError(t, err)

				err = packages_cleanup_service.Cleanup(db.DefaultContext, duration)
				assert.NoError(t, err)

				for _, v := range c.Versions {
					pv, err := packages_model.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeGeneric, "package", v.Version)
					if v.ShouldExist {
						assert.NoError(t, err)
						err = packages_service.DeletePackageVersionAndReferences(db.DefaultContext, pv)
						assert.NoError(t, err)
					} else {
						assert.ErrorIs(t, err, packages_model.ErrPackageNotExist)
					}
				}

				assert.NoError(t, packages_model.DeleteCleanupRuleByID(db.DefaultContext, pcr.ID))
			})
		}
	})
}
