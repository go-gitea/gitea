// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"strconv"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	nuget_module "code.gitea.io/gitea/modules/packages/nuget"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/packages/nuget"
	packageService "code.gitea.io/gitea/services/packages"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func addNuGetAPIKeyHeader(req *RequestWrapper, token string) {
	req.SetHeader("X-NuGet-ApiKey", token)
}

func decodeXML(t testing.TB, resp *httptest.ResponseRecorder, v any) {
	t.Helper()

	assert.NoError(t, xml.NewDecoder(resp.Body).Decode(v))
}

func TestPackageNuGet(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	type FeedEntryProperties struct {
		Version                  string                      `xml:"Version"`
		NormalizedVersion        string                      `xml:"NormalizedVersion"`
		Authors                  string                      `xml:"Authors"`
		Dependencies             string                      `xml:"Dependencies"`
		Description              string                      `xml:"Description"`
		VersionDownloadCount     nuget.TypedValue[int64]     `xml:"VersionDownloadCount"`
		DownloadCount            nuget.TypedValue[int64]     `xml:"DownloadCount"`
		PackageSize              nuget.TypedValue[int64]     `xml:"PackageSize"`
		Created                  nuget.TypedValue[time.Time] `xml:"Created"`
		LastUpdated              nuget.TypedValue[time.Time] `xml:"LastUpdated"`
		Published                nuget.TypedValue[time.Time] `xml:"Published"`
		ProjectURL               string                      `xml:"ProjectUrl,omitempty"`
		ReleaseNotes             string                      `xml:"ReleaseNotes,omitempty"`
		RequireLicenseAcceptance nuget.TypedValue[bool]      `xml:"RequireLicenseAcceptance"`
		Title                    string                      `xml:"Title"`
	}

	type FeedEntry struct {
		XMLName    xml.Name             `xml:"entry"`
		Properties *FeedEntryProperties `xml:"properties"`
		Content    string               `xml:",innerxml"`
	}

	type FeedEntryLink struct {
		Rel  string `xml:"rel,attr"`
		Href string `xml:"href,attr"`
	}

	type FeedResponse struct {
		XMLName xml.Name        `xml:"feed"`
		Links   []FeedEntryLink `xml:"link"`
		Entries []*FeedEntry    `xml:"entry"`
		Count   int64           `xml:"count"`
	}

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	writeToken := getUserToken(t, user.Name, auth_model.AccessTokenScopeWritePackage)
	readToken := getUserToken(t, user.Name, auth_model.AccessTokenScopeReadPackage)
	badToken := getUserToken(t, user.Name, auth_model.AccessTokenScopeReadNotification)

	packageName := "test.package"
	packageVersion := "1.0.3"
	packageAuthors := "KN4CK3R"
	packageDescription := "Gitea Test Package"
	symbolFilename := "test.pdb"
	symbolID := "d910bb6948bd4c6cb40155bcf52c3c94"

	createNuspec := func(id, version string) string {
		return `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://schemas.microsoft.com/packaging/2013/05/nuspec.xsd">
	<metadata>
		<id>` + id + `</id>
		<version>` + version + `</version>
		<authors>` + packageAuthors + `</authors>
		<description>` + packageDescription + `</description>
		<dependencies>
			<group targetFramework=".NETStandard2.0">
				<dependency id="Microsoft.CSharp" version="4.5.0" />
			</group>
		</dependencies>
	</metadata>
</package>`
	}

	createPackage := func(id, version string) *bytes.Buffer {
		var buf bytes.Buffer
		archive := zip.NewWriter(&buf)
		w, _ := archive.Create("package.nuspec")
		w.Write([]byte(createNuspec(id, version)))
		archive.Close()
		return &buf
	}

	content := createPackage(packageName, packageVersion).Bytes()

	url := fmt.Sprintf("/api/packages/%s/nuget", user.Name)

	t.Run("ServiceIndex", func(t *testing.T) {
		t.Run("v2", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			privateUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{Visibility: structs.VisibleTypePrivate})

			cases := []struct {
				Owner          string
				UseBasicAuth   bool
				token          string
				expectedStatus int
			}{
				{privateUser.Name, false, "", http.StatusOK},
				{privateUser.Name, true, "", http.StatusOK},
				{privateUser.Name, false, writeToken, http.StatusOK},
				{privateUser.Name, false, readToken, http.StatusOK},
				{privateUser.Name, false, badToken, http.StatusOK},
				{user.Name, false, "", http.StatusOK},
				{user.Name, true, "", http.StatusOK},
				{user.Name, false, writeToken, http.StatusOK},
				{user.Name, false, readToken, http.StatusOK},
				{user.Name, false, badToken, http.StatusOK},
			}

			for _, c := range cases {
				t.Run(c.Owner, func(t *testing.T) {
					url := fmt.Sprintf("/api/packages/%s/nuget", c.Owner)

					req := NewRequest(t, "GET", url)
					if c.UseBasicAuth {
						req.AddBasicAuth(user.Name)
					} else if c.token != "" {
						addNuGetAPIKeyHeader(req, c.token)
					}
					resp := MakeRequest(t, req, c.expectedStatus)
					if c.expectedStatus != http.StatusOK {
						return
					}

					var result nuget.ServiceIndexResponseV2
					decodeXML(t, resp, &result)

					assert.Equal(t, setting.AppURL+url[1:], result.Base)
					assert.Equal(t, "Packages", result.Workspace.Collection.Href)
				})
			}
		})

		t.Run("v3", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			privateUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{Visibility: structs.VisibleTypePrivate})

			cases := []struct {
				Owner          string
				UseBasicAuth   bool
				token          string
				expectedStatus int
			}{
				{privateUser.Name, false, "", http.StatusOK},
				{privateUser.Name, true, "", http.StatusOK},
				{privateUser.Name, false, writeToken, http.StatusOK},
				{privateUser.Name, false, readToken, http.StatusOK},
				{privateUser.Name, false, badToken, http.StatusOK},
				{user.Name, false, "", http.StatusOK},
				{user.Name, true, "", http.StatusOK},
				{user.Name, false, writeToken, http.StatusOK},
				{user.Name, false, readToken, http.StatusOK},
				{user.Name, false, badToken, http.StatusOK},
			}

			for _, c := range cases {
				t.Run(c.Owner, func(t *testing.T) {
					url := fmt.Sprintf("/api/packages/%s/nuget", c.Owner)

					req := NewRequest(t, "GET", fmt.Sprintf("%s/index.json", url))
					if c.UseBasicAuth {
						req.AddBasicAuth(user.Name)
					} else if c.token != "" {
						addNuGetAPIKeyHeader(req, c.token)
					}
					resp := MakeRequest(t, req, c.expectedStatus)

					if c.expectedStatus != http.StatusOK {
						return
					}

					var result nuget.ServiceIndexResponseV3
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
			}
		})
	})

	t.Run("Upload", func(t *testing.T) {
		t.Run("DependencyPackage", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// create with username/password
			req := NewRequestWithBody(t, "PUT", url, bytes.NewReader(content)).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)

			pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeNuGet)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1, "Should have one version")

			pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
			assert.NoError(t, err)
			assert.NotNil(t, pd.SemVer)
			assert.IsType(t, &nuget_module.Metadata{}, pd.Metadata)
			assert.Equal(t, packageName, pd.Package.Name)
			assert.Equal(t, packageVersion, pd.Version.Version)

			pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
			assert.NoError(t, err)
			assert.Len(t, pfs, 2, "Should have 2 files: nuget and nuspec")
			for _, pf := range pfs {
				switch pf.Name {
				case fmt.Sprintf("%s.%s.nupkg", packageName, packageVersion):
					assert.True(t, pf.IsLead)

					pb, err := packages.GetBlobByID(db.DefaultContext, pf.BlobID)
					assert.NoError(t, err)
					assert.Equal(t, int64(len(content)), pb.Size)
				case fmt.Sprintf("%s.nuspec", packageName):
					assert.False(t, pf.IsLead)
				default:
					assert.Fail(t, "unexpected filename: %v", pf.Name)
				}
			}

			req = NewRequestWithBody(t, "PUT", url, bytes.NewReader(content)).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusConflict)

			// delete the package
			assert.NoError(t, packageService.DeletePackageVersionAndReferences(db.DefaultContext, pvs[0]))

			// create failure with token without write access
			req = NewRequestWithBody(t, "PUT", url, bytes.NewReader(content)).
				AddTokenAuth(readToken)
			MakeRequest(t, req, http.StatusUnauthorized)

			// create with token
			req = NewRequestWithBody(t, "PUT", url, bytes.NewReader(content)).
				AddTokenAuth(writeToken)
			MakeRequest(t, req, http.StatusCreated)

			pvs, err = packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeNuGet)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1, "Should have one version")

			pd, err = packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
			assert.NoError(t, err)
			assert.NotNil(t, pd.SemVer)
			assert.IsType(t, &nuget_module.Metadata{}, pd.Metadata)
			assert.Equal(t, packageName, pd.Package.Name)
			assert.Equal(t, packageVersion, pd.Version.Version)

			pfs, err = packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
			assert.NoError(t, err)
			assert.Len(t, pfs, 2, "Should have 2 files: nuget and nuspec")
			for _, pf := range pfs {
				switch pf.Name {
				case fmt.Sprintf("%s.%s.nupkg", packageName, packageVersion):
					assert.True(t, pf.IsLead)

					pb, err := packages.GetBlobByID(db.DefaultContext, pf.BlobID)
					assert.NoError(t, err)
					assert.Equal(t, int64(len(content)), pb.Size)
				case fmt.Sprintf("%s.nuspec", packageName):
					assert.False(t, pf.IsLead)
				default:
					assert.Fail(t, "unexpected filename: %v", pf.Name)
				}
			}

			req = NewRequestWithBody(t, "PUT", url, bytes.NewReader(content)).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusConflict)
		})

		t.Run("SymbolPackage", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			createSymbolPackage := func(id, packageType string) io.Reader {
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

				w, _ = archive.Create(symbolFilename)
				b, _ := base64.StdEncoding.DecodeString(`QlNKQgEAAQAAAAAADAAAAFBEQiB2MS4wAAAAAAAABgB8AAAAWAAAACNQZGIAAAAA1AAAAAgBAAAj
fgAA3AEAAAQAAAAjU3RyaW5ncwAAAADgAQAABAAAACNVUwDkAQAAMAAAACNHVUlEAAAAFAIAACgB
AAAjQmxvYgAAAGm7ENm9SGxMtAFVvPUsPJTF6PbtAAAAAFcVogEJAAAAAQAAAA==`)
				w.Write(b)

				archive.Close()
				return &buf
			}

			req := NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/symbolpackage", url), createSymbolPackage("unknown-package", "SymbolsPackage")).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNotFound)

			req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/symbolpackage", url), createSymbolPackage(packageName, "DummyPackage")).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusBadRequest)

			req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/symbolpackage", url), createSymbolPackage(packageName, "SymbolsPackage")).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)

			pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeNuGet)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1)

			pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
			assert.NoError(t, err)
			assert.NotNil(t, pd.SemVer)
			assert.IsType(t, &nuget_module.Metadata{}, pd.Metadata)
			assert.Equal(t, packageName, pd.Package.Name)
			assert.Equal(t, packageVersion, pd.Version.Version)

			pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
			assert.NoError(t, err)
			assert.Len(t, pfs, 4, "Should have 4 files: nupkg, snupkg, nuspec and pdb")
			for _, pf := range pfs {
				switch pf.Name {
				case fmt.Sprintf("%s.%s.nupkg", packageName, packageVersion):
					assert.True(t, pf.IsLead)

					pb, err := packages.GetBlobByID(db.DefaultContext, pf.BlobID)
					assert.NoError(t, err)
					assert.Equal(t, int64(412), pb.Size)
				case fmt.Sprintf("%s.%s.snupkg", packageName, packageVersion):
					assert.False(t, pf.IsLead)

					pb, err := packages.GetBlobByID(db.DefaultContext, pf.BlobID)
					assert.NoError(t, err)
					assert.Equal(t, int64(616), pb.Size)
				case fmt.Sprintf("%s.nuspec", packageName):
					assert.False(t, pf.IsLead)

					pb, err := packages.GetBlobByID(db.DefaultContext, pf.BlobID)
					assert.NoError(t, err)
					assert.Equal(t, int64(427), pb.Size)
				case symbolFilename:
					assert.False(t, pf.IsLead)

					pb, err := packages.GetBlobByID(db.DefaultContext, pf.BlobID)
					assert.NoError(t, err)
					assert.Equal(t, int64(160), pb.Size)

					pps, err := packages.GetProperties(db.DefaultContext, packages.PropertyTypeFile, pf.ID)
					assert.NoError(t, err)
					assert.Len(t, pps, 1)
					assert.Equal(t, nuget_module.PropertySymbolID, pps[0].Name)
					assert.Equal(t, symbolID, pps[0].Value)
				default:
					assert.FailNow(t, "unexpected file: %v", pf.Name)
				}
			}

			req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/symbolpackage", url), createSymbolPackage(packageName, "SymbolsPackage")).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusConflict)
		})
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		checkDownloadCount := func(count int64) {
			pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeNuGet)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1)
			assert.Equal(t, count, pvs[0].DownloadCount)
		}

		checkDownloadCount(0)

		req := NewRequest(t, "GET", fmt.Sprintf("%s/package/%s/%s/%s.%s.nupkg", url, packageName, packageVersion, packageName, packageVersion)).
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, content, resp.Body.Bytes())

		req = NewRequest(t, "GET", fmt.Sprintf("%s/package/%s/%s/%s.nuspec", url, packageName, packageVersion, packageName)).
			AddBasicAuth(user.Name)
		resp = MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, createNuspec(packageName, packageVersion), resp.Body.String())

		checkDownloadCount(1)

		req = NewRequest(t, "GET", fmt.Sprintf("%s/package/%s/%s/%s.%s.snupkg", url, packageName, packageVersion, packageName, packageVersion)).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusOK)

		checkDownloadCount(1)

		t.Run("Symbol", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/symbols/%s/%sFFFFFFFF/gitea.pdb", url, symbolFilename, symbolID))
			MakeRequest(t, req, http.StatusBadRequest)

			req = NewRequest(t, "GET", fmt.Sprintf("%s/symbols/%s/%sFFFFFFFF/%s", url, symbolFilename, "00000000000000000000000000000000", symbolFilename)).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNotFound)

			req = NewRequest(t, "GET", fmt.Sprintf("%s/symbols/%s/%sFFFFffff/%s", url, symbolFilename, symbolID, symbolFilename)).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusOK)

			checkDownloadCount(1)
		})
	})

	containsOneNextLink := func(t *testing.T, links []FeedEntryLink) func() bool {
		return func() bool {
			found := 0
			for _, l := range links {
				if l.Rel == "next" {
					found++
					u, err := neturl.Parse(l.Href)
					assert.NoError(t, err)
					q := u.Query()
					assert.Contains(t, q, "$skip")
					assert.Contains(t, q, "$top")
					assert.Equal(t, "1", q.Get("$skip"))
					assert.Equal(t, "1", q.Get("$top"))
				}
			}
			return found == 1
		}
	}

	t.Run("SearchService", func(t *testing.T) {
		cases := []struct {
			Query              string
			Skip               int
			Take               int
			ExpectedTotal      int64
			ExpectedResults    int
			ExpectedExactMatch bool
		}{
			{"", 0, 0, 4, 4, false},
			{"", 0, 10, 4, 4, false},
			{"gitea", 0, 10, 0, 0, false},
			{"test", 0, 10, 1, 1, false},
			{"test", 1, 10, 1, 0, false},
			{"almost.similar", 0, 0, 3, 3, true},
		}

		fakePackages := []string{
			packageName,
			"almost.similar.dependency",
			"almost.similar",
			"almost.similar.dependant",
		}

		for _, fakePackageName := range fakePackages {
			req := NewRequestWithBody(t, "PUT", url, createPackage(fakePackageName, "1.0.99")).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)
		}

		t.Run("v2", func(t *testing.T) {
			t.Run("Search()", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				for i, c := range cases {
					req := NewRequest(t, "GET", fmt.Sprintf("%s/Search()?searchTerm='%s'&$skip=%d&$top=%d", url, c.Query, c.Skip, c.Take)).
						AddBasicAuth(user.Name)
					resp := MakeRequest(t, req, http.StatusOK)

					var result FeedResponse
					decodeXML(t, resp, &result)

					assert.Equal(t, c.ExpectedTotal, result.Count, "case %d: unexpected total hits", i)
					assert.Len(t, result.Entries, c.ExpectedResults, "case %d: unexpected result count", i)

					req = NewRequest(t, "GET", fmt.Sprintf("%s/Search()/$count?searchTerm='%s'&$skip=%d&$top=%d", url, c.Query, c.Skip, c.Take)).
						AddBasicAuth(user.Name)
					resp = MakeRequest(t, req, http.StatusOK)

					assert.Equal(t, strconv.FormatInt(c.ExpectedTotal, 10), resp.Body.String(), "case %d: unexpected total hits", i)
				}
			})

			t.Run("Packages()", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				for i, c := range cases {
					req := NewRequest(t, "GET", fmt.Sprintf("%s/Packages()?$filter=substringof('%s',tolower(Id))&$skip=%d&$top=%d", url, c.Query, c.Skip, c.Take)).
						AddBasicAuth(user.Name)
					resp := MakeRequest(t, req, http.StatusOK)

					var result FeedResponse
					decodeXML(t, resp, &result)

					assert.Equal(t, c.ExpectedTotal, result.Count, "case %d: unexpected total hits", i)
					assert.Len(t, result.Entries, c.ExpectedResults, "case %d: unexpected result count", i)

					req = NewRequest(t, "GET", fmt.Sprintf("%s/Packages()/$count?$filter=substringof('%s',tolower(Id))&$skip=%d&$top=%d", url, c.Query, c.Skip, c.Take)).
						AddBasicAuth(user.Name)
					resp = MakeRequest(t, req, http.StatusOK)

					assert.Equal(t, strconv.FormatInt(c.ExpectedTotal, 10), resp.Body.String(), "case %d: unexpected total hits", i)
				}
			})

			t.Run("Packages()", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				t.Run("substringof", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					for i, c := range cases {
						req := NewRequest(t, "GET", fmt.Sprintf("%s/Packages()?$filter=substringof('%s',tolower(Id))&$skip=%d&$top=%d", url, c.Query, c.Skip, c.Take)).
							AddBasicAuth(user.Name)
						resp := MakeRequest(t, req, http.StatusOK)

						var result FeedResponse
						decodeXML(t, resp, &result)

						assert.Equal(t, c.ExpectedTotal, result.Count, "case %d: unexpected total hits", i)
						assert.Len(t, result.Entries, c.ExpectedResults, "case %d: unexpected result count", i)

						req = NewRequest(t, "GET", fmt.Sprintf("%s/Packages()/$count?$filter=substringof('%s',tolower(Id))&$skip=%d&$top=%d", url, c.Query, c.Skip, c.Take)).
							AddBasicAuth(user.Name)
						resp = MakeRequest(t, req, http.StatusOK)

						assert.Equal(t, strconv.FormatInt(c.ExpectedTotal, 10), resp.Body.String(), "case %d: unexpected total hits", i)
					}
				})

				t.Run("IdEq", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					for i, c := range cases {
						if c.Query == "" {
							// Ignore the `tolower(Id) eq ''` as it's unlikely to happen
							continue
						}
						req := NewRequest(t, "GET", fmt.Sprintf("%s/Packages()?$filter=(tolower(Id) eq '%s')&$skip=%d&$top=%d", url, c.Query, c.Skip, c.Take)).
							AddBasicAuth(user.Name)
						resp := MakeRequest(t, req, http.StatusOK)

						var result FeedResponse
						decodeXML(t, resp, &result)

						expectedCount := 0
						if c.ExpectedExactMatch {
							expectedCount = 1
						}

						assert.Equal(t, int64(expectedCount), result.Count, "case %d: unexpected total hits", i)
						assert.Len(t, result.Entries, expectedCount, "case %d: unexpected result count", i)

						req = NewRequest(t, "GET", fmt.Sprintf("%s/Packages()/$count?$filter=(tolower(Id) eq '%s')&$skip=%d&$top=%d", url, c.Query, c.Skip, c.Take)).
							AddBasicAuth(user.Name)
						resp = MakeRequest(t, req, http.StatusOK)

						assert.Equal(t, strconv.FormatInt(int64(expectedCount), 10), resp.Body.String(), "case %d: unexpected total hits", i)
					}
				})
			})

			t.Run("Next", func(t *testing.T) {
				req := NewRequest(t, "GET", fmt.Sprintf("%s/Search()?searchTerm='test'&$skip=0&$top=1", url)).
					AddBasicAuth(user.Name)
				resp := MakeRequest(t, req, http.StatusOK)

				var result FeedResponse
				decodeXML(t, resp, &result)

				assert.Condition(t, containsOneNextLink(t, result.Links))
			})
		})

		t.Run("v3", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			for i, c := range cases {
				req := NewRequest(t, "GET", fmt.Sprintf("%s/query?q=%s&skip=%d&take=%d", url, c.Query, c.Skip, c.Take)).
					AddBasicAuth(user.Name)
				resp := MakeRequest(t, req, http.StatusOK)

				var result nuget.SearchResultResponse
				DecodeJSON(t, resp, &result)

				assert.Equal(t, c.ExpectedTotal, result.TotalHits, "case %d: unexpected total hits", i)
				assert.Len(t, result.Data, c.ExpectedResults, "case %d: unexpected result count", i)
			}

			t.Run("EnforceGrouped", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequestWithBody(t, "PUT", url, createPackage(packageName+".dummy", "1.0.0")).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusCreated)

				req = NewRequest(t, "GET", fmt.Sprintf("%s/query?q=%s", url, packageName)).
					AddBasicAuth(user.Name)
				resp := MakeRequest(t, req, http.StatusOK)

				var result nuget.SearchResultResponse
				DecodeJSON(t, resp, &result)

				assert.EqualValues(t, 2, result.TotalHits)
				assert.Len(t, result.Data, 2)
				for _, sr := range result.Data {
					if sr.ID == packageName {
						assert.Len(t, sr.Versions, 2)
					} else {
						assert.Len(t, sr.Versions, 1)
					}
				}

				req = NewRequest(t, "DELETE", fmt.Sprintf("%s/%s/%s", url, packageName+".dummy", "1.0.0")).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusNoContent)
			})
		})

		for _, fakePackageName := range fakePackages {
			req := NewRequest(t, "DELETE", fmt.Sprintf("%s/%s/%s", url, fakePackageName, "1.0.99")).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNoContent)
		}
	})

	t.Run("RegistrationService", func(t *testing.T) {
		indexURL := fmt.Sprintf("%s%s/registration/%s/index.json", setting.AppURL, url[1:], packageName)
		leafURL := fmt.Sprintf("%s%s/registration/%s/%s.json", setting.AppURL, url[1:], packageName, packageVersion)
		contentURL := fmt.Sprintf("%s%s/package/%s/%s/%s.%s.nupkg", setting.AppURL, url[1:], packageName, packageVersion, packageName, packageVersion)

		t.Run("RegistrationIndex", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/registration/%s/index.json", url, packageName)).
				AddBasicAuth(user.Name)
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
			t.Run("v2", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "GET", fmt.Sprintf("%s/Packages(Id='%s',Version='%s')", url, packageName, packageVersion)).
					AddBasicAuth(user.Name)
				resp := MakeRequest(t, req, http.StatusOK)

				var result FeedEntry
				decodeXML(t, resp, &result)

				assert.Equal(t, packageName, result.Properties.Title)
				assert.Equal(t, packageVersion, result.Properties.Version)
				assert.Equal(t, packageAuthors, result.Properties.Authors)
				assert.Equal(t, packageDescription, result.Properties.Description)
				assert.Equal(t, "Microsoft.CSharp:4.5.0:.NETStandard2.0", result.Properties.Dependencies)
			})

			t.Run("v3", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "GET", fmt.Sprintf("%s/registration/%s/%s.json", url, packageName, packageVersion)).
					AddBasicAuth(user.Name)
				resp := MakeRequest(t, req, http.StatusOK)

				var result nuget.RegistrationLeafResponse
				DecodeJSON(t, resp, &result)

				assert.Equal(t, leafURL, result.RegistrationLeafURL)
				assert.Equal(t, contentURL, result.PackageContentURL)
				assert.Equal(t, indexURL, result.RegistrationIndexURL)
			})
		})
	})

	t.Run("PackageService", func(t *testing.T) {
		t.Run("v2", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/FindPackagesById()?id='%s'&$top=1", url, packageName)).
				AddBasicAuth(user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			var result FeedResponse
			decodeXML(t, resp, &result)

			assert.Len(t, result.Entries, 1)
			assert.Equal(t, packageVersion, result.Entries[0].Properties.Version)
			assert.Condition(t, containsOneNextLink(t, result.Links))

			req = NewRequest(t, "GET", fmt.Sprintf("%s/FindPackagesById()/$count?id='%s'", url, packageName)).
				AddBasicAuth(user.Name)
			resp = MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, "1", resp.Body.String())
		})

		t.Run("v3", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/package/%s/index.json", url, packageName)).
				AddBasicAuth(user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			var result nuget.PackageVersionsResponse
			DecodeJSON(t, resp, &result)

			assert.Len(t, result.Versions, 1)
			assert.Equal(t, packageVersion, result.Versions[0])
		})
	})

	t.Run("Delete", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "DELETE", fmt.Sprintf("%s/%s/%s", url, packageName, packageVersion)).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNoContent)

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeNuGet)
		assert.NoError(t, err)
		assert.Empty(t, pvs)
	})

	t.Run("DownloadNotExists", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/package/%s/%s/%s.%s.nupkg", url, packageName, packageVersion, packageName, packageVersion)).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "GET", fmt.Sprintf("%s/package/%s/%s/%s.%s.snupkg", url, packageName, packageVersion, packageName, packageVersion)).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("DeleteNotExists", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "DELETE", fmt.Sprintf("%s/package/%s/%s", url, packageName, packageVersion)).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)
	})
}
