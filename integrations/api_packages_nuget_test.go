// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	nuget_module "code.gitea.io/gitea/modules/packages/nuget"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/packages/nuget"

	"github.com/stretchr/testify/assert"
)

func TestPackageNuGet(t *testing.T) {
	defer prepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)

	packageName := "test.package"
	packageVersion := "1.0.3"
	packageAuthors := "KN4CK3R"
	packageDescription := "Gitea Test Package"

	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	w, _ := archive.Create("package.nuspec")
	w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
	<package xmlns="http://schemas.microsoft.com/packaging/2013/05/nuspec.xsd">
	  <metadata>
		<id>` + packageName + `</id>
		<version>` + packageVersion + `</version>
		<authors>` + packageAuthors + `</authors>
		<description>` + packageDescription + `</description>
		<group targetFramework=".NETStandard2.0">
			<dependency id="Microsoft.CSharp" version="4.5.0" />
		</group>
	  </metadata>
	</package>`))
	archive.Close()
	content := buf.Bytes()

	url := fmt.Sprintf("/api/v1/packages/%s/nuget", user.Name)

	t.Run("ServiceIndex", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/index.json", url))
		req = AddBasicAuthHeader(req, user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		var result nuget.ServiceIndexResponse
		DecodeJSON(t, resp, &result)

		assert.Equal(t, "3.0.0", result.Version)
		assert.NotEmpty(t, result.Resources)

		root := setting.AppURL + url[1:]
		for _, r := range result.Resources {
			switch r.Type {
			case "SearchQueryService":
				fallthrough
			case "SearchQueryService/3.0.0-beta":
				fallthrough
			case "SearchQueryService/3.0.0-rc":
				assert.Equal(t, root+"/query", r.ID)
			case "RegistrationsBaseUrl":
				fallthrough
			case "RegistrationsBaseUrl/3.0.0-beta":
				fallthrough
			case "RegistrationsBaseUrl/3.0.0-rc":
				assert.Equal(t, root+"/registration", r.ID)
			case "PackageBaseAddress/3.0.0":
				assert.Equal(t, root+"/package", r.ID)
			case "PackagePublish/2.0.0":
				assert.Equal(t, root, r.ID)
			}
		}
	})

	t.Run("Upload", func(t *testing.T) {
		t.Run("DependencyPackage", func(t *testing.T) {
			defer PrintCurrentTest(t)()

			req := NewRequestWithBody(t, "PUT", url, bytes.NewReader(content))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusCreated)

			pvs, err := packages.GetVersionsByPackageType(user.ID, packages.TypeNuGet)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1)

			pd, err := packages.GetPackageDescriptor(pvs[0])
			assert.NoError(t, err)
			assert.NotNil(t, pd.SemVer)
			assert.IsType(t, &nuget_module.Metadata{}, pd.Metadata)
			assert.Equal(t, packageName, pd.Package.Name)
			assert.Equal(t, packageVersion, pd.Version.Version)

			pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
			assert.NoError(t, err)
			assert.Len(t, pfs, 1)
			assert.Equal(t, fmt.Sprintf("%s.%s.nupkg", packageName, packageVersion), pfs[0].Name)
			assert.True(t, pfs[0].IsLead)

			pb, err := packages.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
			assert.NoError(t, err)
			assert.Equal(t, int64(len(content)), pb.Size)

			req = NewRequestWithBody(t, "PUT", url, bytes.NewReader(content))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusBadRequest)
		})

		t.Run("SymbolPackage", func(t *testing.T) {
			defer PrintCurrentTest(t)()

			createPackage := func(id, packageType string) io.Reader {
				var buf bytes.Buffer
				archive := zip.NewWriter(&buf)
				w, _ := archive.Create("package.nuspec")
				w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
				<package xmlns="http://schemas.microsoft.com/packaging/2013/05/nuspec.xsd">
				<metadata>
					<id>` + id + `</id>
					<version>` + packageVersion + `</version>
					<authors>` + packageAuthors + `</authors>
					<description>` + packageDescription + `</description>
					<packageTypes><packageType name="` + packageType + `" /></packageTypes>
				</metadata>
				</package>`))
				archive.Close()
				return &buf
			}

			req := NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/symbolpackage", url), createPackage("unknown-package", "SymbolsPackage"))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusNotFound)

			req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/symbolpackage", url), createPackage(packageName, "DummyPackage"))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusBadRequest)

			req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/symbolpackage", url), createPackage(packageName, "SymbolsPackage"))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusCreated)

			pvs, err := packages.GetVersionsByPackageType(user.ID, packages.TypeNuGet)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1)

			pd, err := packages.GetPackageDescriptor(pvs[0])
			assert.NoError(t, err)
			assert.NotNil(t, pd.SemVer)
			assert.IsType(t, &nuget_module.Metadata{}, pd.Metadata)
			assert.Equal(t, packageName, pd.Package.Name)
			assert.Equal(t, packageVersion, pd.Version.Version)

			pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
			assert.NoError(t, err)
			assert.Len(t, pfs, 2)
			i := 0
			if strings.HasSuffix(pfs[1].Name, ".snupkg") {
				i = 1
			}
			assert.Equal(t, fmt.Sprintf("%s.%s.snupkg", packageName, packageVersion), pfs[i].Name)
			assert.False(t, pfs[i].IsLead)

			pb, err := packages.GetBlobByID(db.DefaultContext, pfs[i].BlobID)
			assert.NoError(t, err)
			assert.Equal(t, int64(368), pb.Size)

			req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/symbolpackage", url), createPackage(packageName, "SymbolsPackage"))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusBadRequest)
		})
	})

	t.Run("Download", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/package/%s/%s/%s.%s.nupkg", url, packageName, packageVersion, packageName, packageVersion))
		req = AddBasicAuthHeader(req, user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, content, resp.Body.Bytes())

		pvs, err := packages.GetVersionsByPackageType(user.ID, packages.TypeNuGet)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)
		assert.Equal(t, int64(1), pvs[0].DownloadCount)

		req = NewRequest(t, "GET", fmt.Sprintf("%s/package/%s/%s/%s.%s.snupkg", url, packageName, packageVersion, packageName, packageVersion))
		req = AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, http.StatusOK)

		pvs, err = packages.GetVersionsByPackageType(user.ID, packages.TypeNuGet)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)
		assert.Equal(t, int64(1), pvs[0].DownloadCount)
	})

	t.Run("SearchService", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		cases := []struct {
			Query           string
			Skip            int
			Take            int
			ExpectedTotal   int64
			ExpectedResults int
		}{
			{"", 0, 0, 1, 1},
			{"", 0, 10, 1, 1},
			{"gitea", 0, 10, 0, 0},
			{"test", 0, 10, 1, 1},
			{"test", 1, 10, 1, 0},
		}

		for i, c := range cases {
			req := NewRequest(t, "GET", fmt.Sprintf("%s/query?q=%s&skip=%d&take=%d", url, c.Query, c.Skip, c.Take))
			req = AddBasicAuthHeader(req, user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			var result nuget.SearchResultResponse
			DecodeJSON(t, resp, &result)

			assert.Equal(t, c.ExpectedTotal, result.TotalHits, "case %d: unexpected total hits", i)
			assert.Len(t, result.Data, c.ExpectedResults, "case %d: unexpected result count", i)
		}
	})

	t.Run("RegistrationService", func(t *testing.T) {
		indexURL := fmt.Sprintf("%s%s/registration/%s/index.json", setting.AppURL, url[1:], packageName)
		leafURL := fmt.Sprintf("%s%s/registration/%s/%s.json", setting.AppURL, url[1:], packageName, packageVersion)
		contentURL := fmt.Sprintf("%s%s/package/%s/%s/%s.%s.nupkg", setting.AppURL, url[1:], packageName, packageVersion, packageName, packageVersion)

		t.Run("RegistrationIndex", func(t *testing.T) {
			defer PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/registration/%s/index.json", url, packageName))
			req = AddBasicAuthHeader(req, user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			var result nuget.RegistrationIndexResponse
			DecodeJSON(t, resp, &result)

			assert.Equal(t, indexURL, result.RegistrationIndexURL)
			assert.Equal(t, 1, result.Count)
			assert.Len(t, result.Pages, 1)
			assert.Equal(t, indexURL, result.Pages[0].RegistrationPageURL)
			assert.Equal(t, packageVersion, result.Pages[0].Lower)
			assert.Equal(t, packageVersion, result.Pages[0].Upper)
			assert.Equal(t, 1, result.Pages[0].Count)
			assert.Len(t, result.Pages[0].Items, 1)
			assert.Equal(t, packageName, result.Pages[0].Items[0].CatalogEntry.ID)
			assert.Equal(t, packageVersion, result.Pages[0].Items[0].CatalogEntry.Version)
			assert.Equal(t, packageAuthors, result.Pages[0].Items[0].CatalogEntry.Authors)
			assert.Equal(t, packageDescription, result.Pages[0].Items[0].CatalogEntry.Description)
			assert.Equal(t, leafURL, result.Pages[0].Items[0].CatalogEntry.CatalogLeafURL)
			assert.Equal(t, contentURL, result.Pages[0].Items[0].CatalogEntry.PackageContentURL)
		})

		t.Run("RegistrationLeaf", func(t *testing.T) {
			defer PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/registration/%s/%s.json", url, packageName, packageVersion))
			req = AddBasicAuthHeader(req, user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			var result nuget.RegistrationLeafResponse
			DecodeJSON(t, resp, &result)

			assert.Equal(t, leafURL, result.RegistrationLeafURL)
			assert.Equal(t, contentURL, result.PackageContentURL)
			assert.Equal(t, indexURL, result.RegistrationIndexURL)
		})
	})

	t.Run("PackageService", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/package/%s/index.json", url, packageName))
		req = AddBasicAuthHeader(req, user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		var result nuget.PackageVersionsResponse
		DecodeJSON(t, resp, &result)

		assert.Len(t, result.Versions, 1)
		assert.Equal(t, packageVersion, result.Versions[0])
	})

	t.Run("Delete", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "DELETE", fmt.Sprintf("%s/%s/%s", url, packageName, packageVersion))
		req = AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, http.StatusOK)

		pvs, err := packages.GetVersionsByPackageType(user.ID, packages.TypeNuGet)
		assert.NoError(t, err)
		assert.Empty(t, pvs)
	})

	t.Run("DownloadNotExists", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/package/%s/%s/%s.%s.nupkg", url, packageName, packageVersion, packageName, packageVersion))
		req = AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "GET", fmt.Sprintf("%s/package/%s/%s/%s.%s.snupkg", url, packageName, packageVersion, packageName, packageVersion))
		req = AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("DeleteNotExists", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "DELETE", fmt.Sprintf("%s/package/%s/%s", url, packageName, packageVersion))
		req = AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, http.StatusNotFound)
	})
}
