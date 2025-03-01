// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	neturl "net/url"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	composer_module "code.gitea.io/gitea/modules/packages/composer"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/packages/composer"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPackageComposer(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	vendorName := "gitea"
	projectName := "composer-package"
	packageName := vendorName + "/" + projectName
	packageVersion := "1.0.3"
	packageDescription := "Package Description"
	packageType := "composer-plugin"
	packageAuthor := "Gitea Authors"
	packageLicense := "MIT"
	packageBin := "./bin/script"

	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	w, _ := archive.Create("composer.json")
	w.Write([]byte(`{
		"name": "` + packageName + `",
		"description": "` + packageDescription + `",
		"type": "` + packageType + `",
		"license": "` + packageLicense + `",
		"authors": [
			{
				"name": "` + packageAuthor + `"
			}
		],
		"bin": [
			"` + packageBin + `"
		]
	}`))
	archive.Close()
	content := buf.Bytes()

	url := fmt.Sprintf("%sapi/packages/%s/composer", setting.AppURL, user.Name)

	t.Run("ServiceIndex", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/packages.json", url)).
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		var result composer.ServiceIndexResponse
		DecodeJSON(t, resp, &result)

		assert.Equal(t, url+"/search.json?q=%query%&type=%type%", result.SearchTemplate)
		assert.Equal(t, url+"/p2/%package%.json", result.MetadataTemplate)
		assert.Equal(t, url+"/list.json", result.PackageList)
	})

	t.Run("Upload", func(t *testing.T) {
		t.Run("MissingVersion", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequestWithBody(t, "PUT", url, bytes.NewReader(content)).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusBadRequest)
		})

		t.Run("Valid", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			uploadURL := url + "?version=" + packageVersion

			req := NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(content)).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)

			pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeComposer)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1)

			pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
			assert.NoError(t, err)
			assert.NotNil(t, pd.SemVer)
			assert.IsType(t, &composer_module.Metadata{}, pd.Metadata)
			assert.Equal(t, packageName, pd.Package.Name)
			assert.Equal(t, packageVersion, pd.Version.Version)

			pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
			assert.NoError(t, err)
			assert.Len(t, pfs, 1)
			assert.Equal(t, fmt.Sprintf("%s-%s.%s.zip", vendorName, projectName, packageVersion), pfs[0].Name)
			assert.True(t, pfs[0].IsLead)

			pb, err := packages.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
			assert.NoError(t, err)
			assert.Equal(t, int64(len(content)), pb.Size)

			req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(content)).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusConflict)
		})
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeComposer)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)
		assert.Equal(t, int64(0), pvs[0].DownloadCount)

		pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)

		req := NewRequest(t, "GET", fmt.Sprintf("%s/files/%s/%s/%s", url, neturl.PathEscape(packageName), neturl.PathEscape(pvs[0].LowerVersion), neturl.PathEscape(pfs[0].LowerName))).
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, content, resp.Body.Bytes())

		pvs, err = packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeComposer)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)
		assert.Equal(t, int64(1), pvs[0].DownloadCount)
	})

	t.Run("SearchService", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		cases := []struct {
			Query           string
			Type            string
			Page            int
			PerPage         int
			ExpectedTotal   int64
			ExpectedResults int
		}{
			{"", "", 0, 0, 1, 1},
			{"", "", 1, 1, 1, 1},
			{"test", "", 1, 0, 0, 0},
			{"gitea", "", 1, 1, 1, 1},
			{"gitea", "", 2, 1, 1, 0},
			{"", packageType, 1, 1, 1, 1},
			{"gitea", packageType, 1, 1, 1, 1},
			{"gitea", "dummy", 1, 1, 0, 0},
		}

		for i, c := range cases {
			req := NewRequest(t, "GET", fmt.Sprintf("%s/search.json?q=%s&type=%s&page=%d&per_page=%d", url, c.Query, c.Type, c.Page, c.PerPage)).
				AddBasicAuth(user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			var result composer.SearchResultResponse
			DecodeJSON(t, resp, &result)

			assert.Equal(t, c.ExpectedTotal, result.Total, "case %d: unexpected total hits", i)
			assert.Len(t, result.Results, c.ExpectedResults, "case %d: unexpected result count", i)
		}
	})

	t.Run("EnumeratePackages", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", url+"/list.json").
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		var result map[string][]string
		DecodeJSON(t, resp, &result)

		assert.Contains(t, result, "packageNames")
		names := result["packageNames"]
		assert.Len(t, names, 1)
		assert.Equal(t, packageName, names[0])
	})

	t.Run("PackageMetadata", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/p2/%s/%s.json", url, vendorName, projectName)).
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		var result composer.PackageMetadataResponse
		DecodeJSON(t, resp, &result)

		assert.Contains(t, result.Packages, packageName)
		pkgs := result.Packages[packageName]
		assert.Len(t, pkgs, 1)
		assert.Equal(t, packageName, pkgs[0].Name)
		assert.Equal(t, packageVersion, pkgs[0].Version)
		assert.Equal(t, packageType, pkgs[0].Type)
		assert.Equal(t, packageDescription, pkgs[0].Description)
		assert.Len(t, pkgs[0].Authors, 1)
		assert.Equal(t, packageAuthor, pkgs[0].Authors[0].Name)
		assert.Equal(t, "zip", pkgs[0].Dist.Type)
		assert.Equal(t, "4f5fa464c3cb808a1df191dbf6cb75363f8b7072", pkgs[0].Dist.Checksum)
		assert.Len(t, pkgs[0].Bin, 1)
		assert.Equal(t, packageBin, pkgs[0].Bin[0])

		// Test package linked to repository
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		userPkgs, err := packages.GetPackagesByType(db.DefaultContext, user.ID, packages.TypeComposer)
		assert.NoError(t, err)
		assert.Len(t, userPkgs, 1)
		assert.EqualValues(t, 0, userPkgs[0].RepoID)

		err = packages.SetRepositoryLink(db.DefaultContext, userPkgs[0].ID, repo1.ID)
		assert.NoError(t, err)

		req = NewRequest(t, "GET", fmt.Sprintf("%s/p2/%s/%s.json", url, vendorName, projectName)).
			AddBasicAuth(user.Name)
		resp = MakeRequest(t, req, http.StatusOK)

		result = composer.PackageMetadataResponse{}
		DecodeJSON(t, resp, &result)

		assert.Contains(t, result.Packages, packageName)
		pkgs = result.Packages[packageName]
		assert.Len(t, pkgs, 1)
		assert.Equal(t, packageName, pkgs[0].Name)
		assert.Equal(t, packageVersion, pkgs[0].Version)
		assert.Equal(t, packageType, pkgs[0].Type)
		assert.Equal(t, packageDescription, pkgs[0].Description)
		assert.Len(t, pkgs[0].Authors, 1)
		assert.Equal(t, packageAuthor, pkgs[0].Authors[0].Name)
		assert.Equal(t, "zip", pkgs[0].Dist.Type)
		assert.Equal(t, "4f5fa464c3cb808a1df191dbf6cb75363f8b7072", pkgs[0].Dist.Checksum)
		assert.Len(t, pkgs[0].Bin, 1)
		assert.Equal(t, packageBin, pkgs[0].Bin[0])
		assert.Equal(t, repo1.HTMLURL(), pkgs[0].Source.URL)
		assert.Equal(t, "git", pkgs[0].Source.Type)
		assert.Equal(t, packageVersion, pkgs[0].Source.Reference)
	})
}
