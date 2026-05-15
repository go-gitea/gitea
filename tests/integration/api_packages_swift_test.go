// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	swift_module "code.gitea.io/gitea/modules/packages/swift"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	swift_router "code.gitea.io/gitea/routers/api/packages/swift"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageSwift(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	packageScope := "test-scope"
	packageName := "test_package"
	packageID := packageScope + "." + packageName
	packageVersion := "1.0.3"
	packageVersion2 := "1.0.4"
	packageVersion3 := "1.0.5"
	packageAuthor := "KN4CK3R"
	packageDescription := "Gitea Test Package"
	packageCodeRepositoryURL := "https://gitea.io/gitea/gitea" // this one is not used as a property, it is meta
	packageLicenseURL := "https://opensource.org/license/mit"
	packageRepositoryURL1 := "https://gitea.io/gitea/repo"
	packageRepositoryURLs := []string{packageRepositoryURL1, "https://gitea.io/gitea/repo.git", "ssh://git@gitea.io/gitea/repo.git"}
	makePackageMetadataJSON := func(ver string) string {
		tmpl := `{
	"name":"` + packageName + `",
	"version":"%s",
	"description":"` + packageDescription + `",
	"codeRepository":"` + packageCodeRepositoryURL + `",
	"licenseURL":"` + packageLicenseURL + `",
	"author":{"givenName":"` + packageAuthor + `"},
	"repositoryURLs":["` + strings.Join(packageRepositoryURLs, `","`) + `"]
}`
		return fmt.Sprintf(tmpl, ver)
	}

	contentManifest1 := "// swift-tools-version:5.7\n//\n//  Package.swift"
	contentManifest2 := "// swift-tools-version:5.6\n//\n//  Package@swift-5.6.swift"

	url := fmt.Sprintf("/api/packages/%s/swift", user.Name)

	t.Run("CheckLogin", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequestWithBody(t, "POST", url, strings.NewReader(""))
		MakeRequest(t, req, http.StatusUnauthorized)

		req = NewRequestWithBody(t, "POST", url, strings.NewReader("")).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusOK)

		req = NewRequestWithBody(t, "POST", url+"/login", strings.NewReader(""))
		MakeRequest(t, req, http.StatusUnauthorized)

		req = NewRequestWithBody(t, "POST", url+"/login", strings.NewReader("")).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusOK)
	})

	t.Run("CheckAcceptMediaType", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		for _, sub := range []string{
			"/scope/package",
			"/scope/package.json",
			"/scope/package/1.0.0",
			"/scope/package/1.0.0.json",
			"/scope/package/1.0.0.zip",
			"/scope/package/1.0.0/Package.swift",
			"/identifiers",
		} {
			req := NewRequest(t, "GET", url+sub)
			req.Header.Add("Accept", "application/unknown")
			resp := MakeRequest(t, req, http.StatusBadRequest)

			assert.Equal(t, "1", resp.Header().Get("Content-Version"))
			assert.Equal(t, "application/problem+json", resp.Header().Get("Content-Type"))
		}

		req := NewRequestWithBody(t, "PUT", url+"/scope/package/1.0.0", strings.NewReader("")).
			AddBasicAuth(user.Name).
			SetHeader("Accept", "application/unknown")
		resp := MakeRequest(t, req, http.StatusBadRequest)

		assert.Equal(t, "1", resp.Header().Get("Content-Version"))
		assert.Equal(t, "application/problem+json", resp.Header().Get("Content-Type"))
	})

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		uploadPackage := func(t *testing.T, url string, expectedStatus int, sr io.Reader, metadata string) {
			var body bytes.Buffer
			mpw := multipart.NewWriter(&body)

			part, _ := mpw.CreateFormFile("source-archive", "source-archive.zip")
			io.Copy(part, sr)

			if metadata != "" {
				mpw.WriteField("metadata", metadata)
			}

			mpw.Close()

			req := NewRequestWithBody(t, "PUT", url, &body).
				SetHeader("Content-Type", mpw.FormDataContentType()).
				SetHeader("Accept", swift_router.AcceptJSON).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, expectedStatus)
		}

		for _, triple := range []string{"/sc_ope/package/1.0.0", "/scope/pack~age/1.0.0", "/scope/package/1_0.0"} {
			req := NewRequestWithBody(t, "PUT", url+triple, bytes.NewReader([]byte{})).
				AddBasicAuth(user.Name)
			resp := MakeRequest(t, req, http.StatusBadRequest)

			assert.Equal(t, "1", resp.Header().Get("Content-Version"))
			assert.Equal(t, "application/problem+json", resp.Header().Get("Content-Type"))
		}

		uploadURL := fmt.Sprintf("%s/%s/%s/%s", url, packageScope, packageName, packageVersion)

		req := NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader([]byte{}))
		MakeRequest(t, req, http.StatusUnauthorized)

		uploadPackage(
			t,
			uploadURL,
			http.StatusCreated,
			test.WriteZipArchive(map[string]string{
				"Package.swift":           contentManifest1,
				"Package@swift-5.6.swift": contentManifest2,
			}),
			makePackageMetadataJSON(packageVersion),
		)

		pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeSwift)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(t.Context(), pvs[0])
		assert.NoError(t, err)
		assert.NotNil(t, pd.SemVer)
		assert.Equal(t, packageID, pd.Package.Name)
		assert.Equal(t, packageVersion, pd.Version.Version)
		assert.IsType(t, &swift_module.Metadata{}, pd.Metadata)
		metadata := pd.Metadata.(*swift_module.Metadata)
		assert.Equal(t, packageDescription, metadata.Description)
		assert.Len(t, metadata.Manifests, 2)
		assert.Equal(t, contentManifest1, metadata.Manifests[""].Content)
		assert.Equal(t, contentManifest2, metadata.Manifests["5.6"].Content)
		assert.Len(t, pd.VersionProperties, 3)
		assert.Equal(t, packageRepositoryURL1, pd.VersionProperties.GetByName(swift_module.PropertyRepositoryURL))

		pfs, err := packages.GetFilesByVersionID(t.Context(), pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, fmt.Sprintf("%s-%s.zip", packageName, packageVersion), pfs[0].Name)
		assert.True(t, pfs[0].IsLead)

		uploadPackage(
			t,
			uploadURL,
			http.StatusConflict,
			test.WriteZipArchive(map[string]string{
				"Package.swift": contentManifest1,
			}),
			"",
		)
	})

	t.Run("UploadMultipart", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		uploadPackage := func(t *testing.T, url string, expectedStatus int, sr io.Reader, metadata string) {
			var body bytes.Buffer
			mpw := multipart.NewWriter(&body)

			// Read the source archive content
			sourceContent, err := io.ReadAll(sr)
			assert.NoError(t, err)
			mpw.WriteField("source-archive", string(sourceContent))

			if metadata != "" {
				mpw.WriteField("metadata", metadata)
			}

			mpw.Close()

			req := NewRequestWithBody(t, "PUT", url, &body).
				SetHeader("Content-Type", mpw.FormDataContentType()).
				SetHeader("Accept", swift_router.AcceptJSON).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, expectedStatus)
		}

		uploadURL := fmt.Sprintf("%s/%s/%s/%s", url, packageScope, packageName, packageVersion2)

		req := NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader([]byte{}))
		MakeRequest(t, req, http.StatusUnauthorized)

		// Test with metadata as form field
		uploadPackage(
			t,
			uploadURL,
			http.StatusCreated,
			test.WriteZipArchive(map[string]string{
				"Package.swift":           contentManifest1,
				"Package@swift-5.6.swift": contentManifest2,
			}),
			makePackageMetadataJSON(packageVersion2),
		)

		pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeSwift)
		assert.NoError(t, err)
		require.Len(t, pvs, 2) // ATTENTION: many subtests are unable to run separately, they depend on the results of previous tests
		thisPackageVersion := pvs[0]
		pd, err := packages.GetPackageDescriptor(t.Context(), thisPackageVersion)
		assert.NoError(t, err)
		assert.NotNil(t, pd.SemVer)
		assert.Equal(t, packageID, pd.Package.Name)
		assert.Equal(t, packageVersion2, pd.Version.Version)
		assert.IsType(t, &swift_module.Metadata{}, pd.Metadata)
		metadata := pd.Metadata.(*swift_module.Metadata)
		assert.Equal(t, packageDescription, metadata.Description)
		assert.Len(t, metadata.Manifests, 2)
		assert.Equal(t, contentManifest1, metadata.Manifests[""].Content)
		assert.Equal(t, contentManifest2, metadata.Manifests["5.6"].Content)
		assert.Len(t, pd.VersionProperties, 3)
		assert.Equal(t, packageRepositoryURL1, pd.VersionProperties.GetByName(swift_module.PropertyRepositoryURL))

		pfs, err := packages.GetFilesByVersionID(t.Context(), thisPackageVersion.ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, fmt.Sprintf("%s-%s.zip", packageName, packageVersion2), pfs[0].Name)
		assert.True(t, pfs[0].IsLead)

		uploadPackage(
			t,
			uploadURL,
			http.StatusConflict,
			test.WriteZipArchive(map[string]string{
				"Package.swift": contentManifest1,
			}),
			"",
		)
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s/%s.zip", url, packageScope, packageName, packageVersion)).
			AddBasicAuth(user.Name).
			SetHeader("Accept", swift_router.AcceptZip)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "1", resp.Header().Get("Content-Version"))
		assert.Equal(t, "application/zip", resp.Header().Get("Content-Type"))

		pv, err := packages.GetVersionByNameAndVersion(t.Context(), user.ID, packages.TypeSwift, packageID, packageVersion)
		assert.NotNil(t, pv)
		assert.NoError(t, err)

		pd, err := packages.GetPackageDescriptor(t.Context(), pv)
		assert.NoError(t, err)
		assert.Equal(t, "sha256="+pd.Files[0].Blob.HashSHA256, resp.Header().Get("Digest"))
	})

	t.Run("EnumeratePackageVersions", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s", url, packageScope, packageName)).
			AddBasicAuth(user.Name).
			SetHeader("Accept", swift_router.AcceptJSON)
		resp := MakeRequest(t, req, http.StatusOK)

		versionURL := setting.AppURL + url[1:] + fmt.Sprintf("/%s/%s/%s", packageScope, packageName, packageVersion2)

		assert.Equal(t, "1", resp.Header().Get("Content-Version"))
		assert.Equal(t, fmt.Sprintf(`<%s>; rel="latest-version"`, versionURL), resp.Header().Get("Link"))

		body := resp.Body.String()

		result := DecodeJSON(t, resp, &swift_router.EnumeratePackageVersionsResponse{})

		assert.Len(t, result.Releases, 2)
		assert.Contains(t, result.Releases, packageVersion2)
		assert.Equal(t, versionURL, result.Releases[packageVersion2].URL)

		req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s.json", url, packageScope, packageName)).
			AddBasicAuth(user.Name)
		resp = MakeRequest(t, req, http.StatusOK)

		assert.JSONEq(t, body, resp.Body.String())
	})

	t.Run("PackageVersionMetadata", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s/%s", url, packageScope, packageName, packageVersion)).
			AddBasicAuth(user.Name).
			SetHeader("Accept", swift_router.AcceptJSON)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "1", resp.Header().Get("Content-Version"))

		body := resp.Body.String()

		result := DecodeJSON(t, resp, &swift_router.PackageVersionMetadataResponse{})

		pv, err := packages.GetVersionByNameAndVersion(t.Context(), user.ID, packages.TypeSwift, packageID, packageVersion)
		assert.NotNil(t, pv)
		assert.NoError(t, err)

		pd, err := packages.GetPackageDescriptor(t.Context(), pv)
		assert.NoError(t, err)

		assert.Equal(t, packageID, result.ID)
		assert.Equal(t, packageVersion, result.Version)
		assert.Len(t, result.Resources, 1)
		assert.Equal(t, "source-archive", result.Resources[0].Name)
		assert.Equal(t, "application/zip", result.Resources[0].Type)
		assert.Equal(t, pd.Files[0].Blob.HashSHA256, result.Resources[0].Checksum)
		assert.Equal(t, "SoftwareSourceCode", result.Metadata.Type)
		assert.Equal(t, packageName, result.Metadata.Name)
		assert.Equal(t, packageVersion, result.Metadata.Version)
		assert.Equal(t, packageDescription, result.Metadata.Description)
		assert.Equal(t, "Swift", result.Metadata.ProgrammingLanguage.Name)
		assert.Equal(t, packageLicenseURL, result.Metadata.LicenseURL)
		require.NotNil(t, result.Metadata.Author)
		assert.Equal(t, packageAuthor, result.Metadata.Author.Name)
		assert.Equal(t, packageAuthor, result.Metadata.Author.GivenName)
		assert.ElementsMatch(t, packageRepositoryURLs, result.Metadata.RepositoryURLs)

		req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s/%s.json", url, packageScope, packageName, packageVersion)).
			AddBasicAuth(user.Name)
		resp = MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, body, resp.Body.String())
	})

	t.Run("UploadEmptyJSONMetadata", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		uploadURL := fmt.Sprintf("%s/%s/%s/%s", url, packageScope, packageName, packageVersion3)
		var body bytes.Buffer
		mpw := multipart.NewWriter(&body)

		part, err := mpw.CreateFormFile("source-archive", "source-archive.zip")
		require.NoError(t, err)
		_, err = io.Copy(part, test.WriteZipArchive(map[string]string{
			"Package.swift":           contentManifest1,
			"Package@swift-5.6.swift": contentManifest2,
		}))
		require.NoError(t, err)
		require.NoError(t, mpw.WriteField("metadata", "{}"))
		require.NoError(t, mpw.Close())

		req := NewRequestWithBody(t, "PUT", uploadURL, &body).
			SetHeader("Content-Type", mpw.FormDataContentType()).
			SetHeader("Accept", swift_router.AcceptJSON).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s/%s", url, packageScope, packageName, packageVersion3)).
			AddBasicAuth(user.Name).
			SetHeader("Accept", swift_router.AcceptJSON)
		resp := MakeRequest(t, req, http.StatusOK)
		result := DecodeJSON(t, resp, &swift_router.PackageVersionMetadataResponse{})

		assert.Nil(t, result.Metadata.Author)
		assert.Empty(t, result.Metadata.RepositoryURLs)
		assert.Empty(t, result.Metadata.CodeRepository)
		assert.Empty(t, result.Metadata.LicenseURL)
	})

	t.Run("DownloadManifest", func(t *testing.T) {
		manifestURL := fmt.Sprintf("%s/%s/%s/%s/Package.swift", url, packageScope, packageName, packageVersion)

		t.Run("Default", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", manifestURL).
				AddBasicAuth(user.Name).
				SetHeader("Accept", swift_router.AcceptSwift)
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, "1", resp.Header().Get("Content-Version"))
			assert.Equal(t, "text/x-swift", resp.Header().Get("Content-Type"))
			assert.Equal(t, contentManifest1, resp.Body.String())
		})

		t.Run("DifferentVersion", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", manifestURL+"?swift-version=5.6").
				AddBasicAuth(user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, "1", resp.Header().Get("Content-Version"))
			assert.Equal(t, "text/x-swift", resp.Header().Get("Content-Type"))
			assert.Equal(t, contentManifest2, resp.Body.String())

			req = NewRequest(t, "GET", manifestURL+"?swift-version=5.6.0").
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusOK)
		})

		t.Run("Redirect", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", manifestURL+"?swift-version=1.0").
				AddBasicAuth(user.Name)
			resp := MakeRequest(t, req, http.StatusSeeOther)

			assert.Equal(t, "1", resp.Header().Get("Content-Version"))
			assert.Equal(t, setting.AppURL+url[1:]+fmt.Sprintf("/%s/%s/%s/Package.swift", packageScope, packageName, packageVersion), resp.Header().Get("Location"))
		})
	})

	t.Run("LookupPackageIdentifiers", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", url+"/identifiers").
			SetHeader("Accept", swift_router.AcceptJSON)
		resp := MakeRequest(t, req, http.StatusBadRequest)

		assert.Equal(t, "1", resp.Header().Get("Content-Version"))
		assert.Equal(t, "application/problem+json", resp.Header().Get("Content-Type"))

		req = NewRequest(t, "GET", url+"/identifiers?url=https://unknown.host/")
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "GET", url+"/identifiers?url="+packageRepositoryURL1).
			SetHeader("Accept", swift_router.AcceptJSON)
		resp = MakeRequest(t, req, http.StatusOK)

		result := DecodeJSON(t, resp, &swift_router.LookupPackageIdentifiersResponse{})

		assert.Len(t, result.Identifiers, 1)
		assert.Equal(t, packageID, result.Identifiers[0])
	})
}
