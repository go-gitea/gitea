// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"net/http"
	neturl "net/url"
	"testing"

	"code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	composer_module "code.gitea.io/gitea/modules/packages/composer"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/routers/api/packages/composer"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPackageComposer(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	otherUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	privateUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 31})

	vendorName := "gitea"
	projectName := "composer-package"
	packageName := vendorName + "/" + projectName
	packageVersion := "1.0.3"
	packageDescription := "Package Description"
	packageType := "composer-plugin"
	packageAuthor := "Gitea Authors"
	packageLicense := "MIT"
	packageBin := "./bin/script"

	content := test.WriteZipArchive(map[string]string{
		"composer.json": `{
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
	}`,
	}).Bytes()

	url := fmt.Sprintf("%sapi/packages/%s/composer", setting.AppURL, user.Name)

	t.Run("ServiceIndex", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", url+"/packages.json").
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		result := DecodeJSON(t, resp, &composer.ServiceIndexResponse{})

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

			pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeComposer)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1)

			pd, err := packages.GetPackageDescriptor(t.Context(), pvs[0])
			assert.NoError(t, err)
			assert.NotNil(t, pd.SemVer)
			assert.IsType(t, &composer_module.Metadata{}, pd.Metadata)
			assert.Equal(t, packageName, pd.Package.Name)
			assert.Equal(t, packageVersion, pd.Version.Version)

			pfs, err := packages.GetFilesByVersionID(t.Context(), pvs[0].ID)
			assert.NoError(t, err)
			assert.Len(t, pfs, 1)
			assert.Equal(t, fmt.Sprintf("%s-%s.%s.zip", vendorName, projectName, packageVersion), pfs[0].Name)
			assert.True(t, pfs[0].IsLead)

			pb, err := packages.GetBlobByID(t.Context(), pfs[0].BlobID)
			assert.NoError(t, err)
			assert.Equal(t, int64(len(content)), pb.Size)

			req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(content)).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusConflict)
		})
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeComposer)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)
		assert.Equal(t, int64(0), pvs[0].DownloadCount)

		pfs, err := packages.GetFilesByVersionID(t.Context(), pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)

		req := NewRequest(t, "GET", fmt.Sprintf("%s/files/%s/%s/%s", url, neturl.PathEscape(packageName), neturl.PathEscape(pvs[0].LowerVersion), neturl.PathEscape(pfs[0].LowerName))).
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, content, resp.Body.Bytes())

		pvs, err = packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeComposer)
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

			result := DecodeJSON(t, resp, &composer.SearchResultResponse{})

			assert.Equal(t, c.ExpectedTotal, result.Total, "case %d: unexpected total hits", i)
			assert.Len(t, result.Results, c.ExpectedResults, "case %d: unexpected result count", i)
		}
	})

	t.Run("EnumeratePackages", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", url+"/list.json").
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		result := DecodeJSON(t, resp, map[string][]string{})

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

		result := DecodeJSON(t, resp, &composer.PackageMetadataResponse{})

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
		userPkgs, err := packages.GetPackagesByType(t.Context(), user.ID, packages.TypeComposer)
		assert.NoError(t, err)
		assert.Len(t, userPkgs, 1)
		assert.EqualValues(t, 0, userPkgs[0].RepoID)

		err = packages.SetRepositoryLink(t.Context(), userPkgs[0].ID, repo1.ID)
		assert.NoError(t, err)

		req = NewRequest(t, "GET", fmt.Sprintf("%s/p2/%s/%s.json", url, vendorName, projectName)).
			AddBasicAuth(user.Name)
		resp = MakeRequest(t, req, http.StatusOK)

		result = DecodeJSON(t, resp, &composer.PackageMetadataResponse{})

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

		// Private repository links remain visible to callers who can access the repository.
		repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
		err = packages.SetRepositoryLink(t.Context(), userPkgs[0].ID, repo2.ID)
		assert.NoError(t, err)

		req = NewRequest(t, "GET", fmt.Sprintf("%s/p2/%s/%s.json", url, vendorName, projectName)).
			AddBasicAuth(user.Name)
		resp = MakeRequest(t, req, http.StatusOK)

		result = DecodeJSON(t, resp, &composer.PackageMetadataResponse{})
		pkgs = result.Packages[packageName]
		assert.Len(t, pkgs, 1)
		assert.Equal(t, repo2.HTMLURL(), pkgs[0].Source.URL)
		assert.Equal(t, "git", pkgs[0].Source.Type)
		assert.Equal(t, packageVersion, pkgs[0].Source.Reference)

		// Callers without repository access still get the package metadata, but not the private source URL.
		req = NewRequest(t, "GET", fmt.Sprintf("%s/p2/%s/%s.json", url, vendorName, projectName)).
			AddBasicAuth(otherUser.Name)
		resp = MakeRequest(t, req, http.StatusOK)

		result = DecodeJSON(t, resp, &composer.PackageMetadataResponse{})
		pkgs = result.Packages[packageName]
		assert.Len(t, pkgs, 1)
		assert.Empty(t, pkgs[0].Source.URL)
		assert.Empty(t, pkgs[0].Source.Type)
		assert.Empty(t, pkgs[0].Source.Reference)
	})

	t.Run("WebVisibilityBadge", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		listReq := NewRequest(t, "GET", fmt.Sprintf("/%s/-/packages", user.Name)).
			AddBasicAuth(user.Name)
		listResp := MakeRequest(t, listReq, http.StatusOK)
		listDoc := NewHTMLParser(t, bytes.NewReader(listResp.Body.Bytes()))
		assert.Equal(t, 0, listDoc.Find(".item-title .ui.basic.label").Length())

		viewReq := NewRequest(t, "GET", fmt.Sprintf("/%s/-/packages/composer/%s/%s", user.Name, neturl.PathEscape(packageName), neturl.PathEscape(packageVersion))).
			AddBasicAuth(user.Name)
		viewResp := MakeRequest(t, viewReq, http.StatusOK)
		viewDoc := NewHTMLParser(t, bytes.NewReader(viewResp.Body.Bytes()))
		assert.Equal(t, 0, viewDoc.Find(".issue-title-header .ui.basic.label").Length())

		privatePackageName := privateUser.Name + "/private-composer-package"
		privatePackageVersion := "1.0.0"
		privateContent := test.WriteZipArchive(map[string]string{
			"composer.json": `{
				"name": "` + privatePackageName + `",
				"description": "Private Package",
				"type": "` + packageType + `",
				"license": "` + packageLicense + `",
				"authors": [
					{
						"name": "` + packageAuthor + `"
					}
				]
			}`,
		}).Bytes()
		privateUploadURL := fmt.Sprintf("%sapi/packages/%s/composer?version=%s", setting.AppURL, privateUser.Name, privatePackageVersion)

		uploadReq := NewRequestWithBody(t, "PUT", privateUploadURL, bytes.NewReader(privateContent)).
			AddBasicAuth(privateUser.Name)
		MakeRequest(t, uploadReq, http.StatusCreated)
		privateSession := loginUser(t, privateUser.Name)

		privateListReq := NewRequest(t, "GET", fmt.Sprintf("/%s/-/packages", privateUser.Name))
		privateListResp := privateSession.MakeRequest(t, privateListReq, http.StatusOK)
		privateListDoc := NewHTMLParser(t, bytes.NewReader(privateListResp.Body.Bytes()))
		assert.Equal(t, 1, privateListDoc.Find(".item-title .ui.basic.label").Length())
		assert.Equal(t, "Private", privateListDoc.Find(".item-title .ui.basic.label").First().Text())

		privateViewReq := NewRequest(t, "GET", fmt.Sprintf("/%s/-/packages/composer/%s/%s", privateUser.Name, neturl.PathEscape(privatePackageName), neturl.PathEscape(privatePackageVersion)))
		privateViewResp := privateSession.MakeRequest(t, privateViewReq, http.StatusOK)
		privateViewDoc := NewHTMLParser(t, bytes.NewReader(privateViewResp.Body.Bytes()))
		assert.Equal(t, 1, privateViewDoc.Find(".issue-title-header .ui.basic.label").Length())
		assert.Equal(t, "Private", privateViewDoc.Find(".issue-title-header .ui.basic.label").First().Text())
	})
}
