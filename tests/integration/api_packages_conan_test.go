// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	stdurl "net/url"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	conan_model "code.gitea.io/gitea/models/packages/conan"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	conan_module "code.gitea.io/gitea/modules/packages/conan"
	"code.gitea.io/gitea/modules/setting"
	conan_router "code.gitea.io/gitea/routers/api/packages/conan"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

const (
	conanfileName = "conanfile.py"
	conaninfoName = "conaninfo.txt"

	conanLicense     = "MIT"
	conanAuthor      = "Gitea <info@gitea.io>"
	conanHomepage    = "https://gitea.io/"
	conanURL         = "https://gitea.com/"
	conanDescription = "Description of ConanPackage"
	conanTopic       = "gitea"

	conanPackageReference = "dummyreference"

	contentConaninfo = `[settings]
    arch=x84_64

[requires]
    fmt/7.1.3

[options]
    shared=False

[full_settings]
    arch=x84_64

[full_requires]
    fmt/7.1.3

[full_options]
    shared=False

[recipe_hash]
    74714915a51073acb548ca1ce29afbac

[env]
CC=gcc-10`
)

func addTokenAuthHeader(request *http.Request, token string) *http.Request {
	request.Header.Set("Authorization", token)
	return request
}

func buildConanfileContent(name, version string) string {
	return `from conans import ConanFile, CMake, tools

class ConanPackageConan(ConanFile):
	name = "` + name + `"
	version = "` + version + `"
	license = "` + conanLicense + `"
	author = "` + conanAuthor + `"
	homepage = "` + conanHomepage + `"
	url = "` + conanURL + `"
	description = "` + conanDescription + `"
	topics = ("` + conanTopic + `")
	settings = "os", "compiler", "build_type", "arch"
	options = {"shared": [True, False], "fPIC": [True, False]}
	default_options = {"shared": False, "fPIC": True}
	generators = "cmake"`
}

func uploadConanPackageV1(t *testing.T, baseURL, token, name, version, user, channel string) {
	contentConanfile := buildConanfileContent(name, version)

	recipeURL := fmt.Sprintf("%s/v1/conans/%s/%s/%s/%s", baseURL, name, version, user, channel)

	req := NewRequest(t, "GET", recipeURL)
	req = addTokenAuthHeader(req, token)
	MakeRequest(t, req, http.StatusNotFound)

	req = NewRequest(t, "GET", fmt.Sprintf("%s/digest", recipeURL))
	req = addTokenAuthHeader(req, token)
	MakeRequest(t, req, http.StatusNotFound)

	req = NewRequest(t, "GET", fmt.Sprintf("%s/download_urls", recipeURL))
	req = addTokenAuthHeader(req, token)
	MakeRequest(t, req, http.StatusNotFound)

	req = NewRequest(t, "POST", fmt.Sprintf("%s/upload_urls", recipeURL))
	MakeRequest(t, req, http.StatusUnauthorized)

	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("%s/upload_urls", recipeURL), map[string]int64{
		conanfileName: int64(len(contentConanfile)),
		"removed.txt": 0,
	})
	req = addTokenAuthHeader(req, token)
	resp := MakeRequest(t, req, http.StatusOK)

	uploadURLs := make(map[string]string)
	DecodeJSON(t, resp, &uploadURLs)

	assert.Contains(t, uploadURLs, conanfileName)
	assert.NotContains(t, uploadURLs, "removed.txt")

	uploadURL := uploadURLs[conanfileName]
	assert.NotEmpty(t, uploadURL)

	req = NewRequestWithBody(t, "PUT", uploadURL, strings.NewReader(contentConanfile))
	req = addTokenAuthHeader(req, token)
	MakeRequest(t, req, http.StatusCreated)

	packageURL := fmt.Sprintf("%s/packages/%s", recipeURL, conanPackageReference)

	req = NewRequest(t, "GET", packageURL)
	req = addTokenAuthHeader(req, token)
	MakeRequest(t, req, http.StatusNotFound)

	req = NewRequest(t, "GET", fmt.Sprintf("%s/digest", packageURL))
	req = addTokenAuthHeader(req, token)
	MakeRequest(t, req, http.StatusNotFound)

	req = NewRequest(t, "GET", fmt.Sprintf("%s/download_urls", packageURL))
	req = addTokenAuthHeader(req, token)
	MakeRequest(t, req, http.StatusNotFound)

	req = NewRequest(t, "POST", fmt.Sprintf("%s/upload_urls", packageURL))
	MakeRequest(t, req, http.StatusUnauthorized)

	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("%s/upload_urls", packageURL), map[string]int64{
		conaninfoName: int64(len(contentConaninfo)),
		"removed.txt": 0,
	})
	req = addTokenAuthHeader(req, token)
	resp = MakeRequest(t, req, http.StatusOK)

	uploadURLs = make(map[string]string)
	DecodeJSON(t, resp, &uploadURLs)

	assert.Contains(t, uploadURLs, conaninfoName)
	assert.NotContains(t, uploadURLs, "removed.txt")

	uploadURL = uploadURLs[conaninfoName]
	assert.NotEmpty(t, uploadURL)

	req = NewRequestWithBody(t, "PUT", uploadURL, strings.NewReader(contentConaninfo))
	req = addTokenAuthHeader(req, token)
	MakeRequest(t, req, http.StatusCreated)
}

func uploadConanPackageV2(t *testing.T, baseURL, token, name, version, user, channel, recipeRevision, packageRevision string) {
	contentConanfile := buildConanfileContent(name, version)

	recipeURL := fmt.Sprintf("%s/v2/conans/%s/%s/%s/%s/revisions/%s", baseURL, name, version, user, channel, recipeRevision)

	req := NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/files/%s", recipeURL, conanfileName), strings.NewReader(contentConanfile))
	req = addTokenAuthHeader(req, token)
	MakeRequest(t, req, http.StatusCreated)

	req = NewRequest(t, "GET", fmt.Sprintf("%s/files", recipeURL))
	req = addTokenAuthHeader(req, token)
	resp := MakeRequest(t, req, http.StatusOK)

	var list *struct {
		Files map[string]any `json:"files"`
	}
	DecodeJSON(t, resp, &list)
	assert.Len(t, list.Files, 1)
	assert.Contains(t, list.Files, conanfileName)

	packageURL := fmt.Sprintf("%s/packages/%s/revisions/%s", recipeURL, conanPackageReference, packageRevision)

	req = NewRequest(t, "GET", fmt.Sprintf("%s/files", packageURL))
	req = addTokenAuthHeader(req, token)
	MakeRequest(t, req, http.StatusNotFound)

	req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/files/%s", packageURL, conaninfoName), strings.NewReader(contentConaninfo))
	req = addTokenAuthHeader(req, token)
	MakeRequest(t, req, http.StatusCreated)

	req = NewRequest(t, "GET", fmt.Sprintf("%s/files", packageURL))
	req = addTokenAuthHeader(req, token)
	resp = MakeRequest(t, req, http.StatusOK)

	list = nil
	DecodeJSON(t, resp, &list)
	assert.Len(t, list.Files, 1)
	assert.Contains(t, list.Files, conaninfoName)
}

func TestPackageConan(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	name := "ConanPackage"
	version1 := "1.2"
	version2 := "1.3"
	user1 := "dummy"
	user2 := "gitea"
	channel1 := "test"
	channel2 := "final"
	revision1 := "rev1"
	revision2 := "rev2"

	url := fmt.Sprintf("%sapi/packages/%s/conan", setting.AppURL, user.Name)

	t.Run("v1", func(t *testing.T) {
		t.Run("Ping", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/v1/ping", url))
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, "revisions", resp.Header().Get("X-Conan-Server-Capabilities"))
		})

		token := ""

		t.Run("Authenticate", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/v1/users/authenticate", url))
			req = AddBasicAuthHeader(req, user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			body := resp.Body.String()
			assert.NotEmpty(t, body)

			token = fmt.Sprintf("Bearer %s", body)
		})

		t.Run("CheckCredentials", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/v1/users/check_credentials", url))
			req = addTokenAuthHeader(req, token)
			MakeRequest(t, req, http.StatusOK)
		})

		t.Run("Upload", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			uploadConanPackageV1(t, url, token, name, version1, user1, channel1)

			t.Run("Validate", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeConan)
				assert.NoError(t, err)
				assert.Len(t, pvs, 1)

				pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
				assert.NoError(t, err)
				assert.Nil(t, pd.SemVer)
				assert.Equal(t, name, pd.Package.Name)
				assert.Equal(t, version1, pd.Version.Version)
				assert.IsType(t, &conan_module.Metadata{}, pd.Metadata)
				metadata := pd.Metadata.(*conan_module.Metadata)
				assert.Equal(t, conanLicense, metadata.License)
				assert.Equal(t, conanAuthor, metadata.Author)
				assert.Equal(t, conanHomepage, metadata.ProjectURL)
				assert.Equal(t, conanURL, metadata.RepositoryURL)
				assert.Equal(t, conanDescription, metadata.Description)
				assert.Equal(t, []string{conanTopic}, metadata.Keywords)

				pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
				assert.NoError(t, err)
				assert.Len(t, pfs, 2)

				for _, pf := range pfs {
					pb, err := packages.GetBlobByID(db.DefaultContext, pf.BlobID)
					assert.NoError(t, err)

					if pf.Name == conanfileName {
						assert.True(t, pf.IsLead)

						assert.Equal(t, int64(len(buildConanfileContent(name, version1))), pb.Size)
					} else if pf.Name == conaninfoName {
						assert.False(t, pf.IsLead)

						assert.Equal(t, int64(len(contentConaninfo)), pb.Size)
					} else {
						assert.FailNow(t, "unknown file: %s", pf.Name)
					}
				}
			})
		})

		t.Run("Download", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			recipeURL := fmt.Sprintf("%s/v1/conans/%s/%s/%s/%s", url, name, version1, user1, channel1)

			req := NewRequest(t, "GET", recipeURL)
			resp := MakeRequest(t, req, http.StatusOK)

			fileHashes := make(map[string]string)
			DecodeJSON(t, resp, &fileHashes)
			assert.Len(t, fileHashes, 1)
			assert.Contains(t, fileHashes, conanfileName)
			assert.Equal(t, "7abc52241c22090782c54731371847a8", fileHashes[conanfileName])

			req = NewRequest(t, "GET", fmt.Sprintf("%s/digest", recipeURL))
			resp = MakeRequest(t, req, http.StatusOK)

			downloadURLs := make(map[string]string)
			DecodeJSON(t, resp, &downloadURLs)
			assert.Contains(t, downloadURLs, conanfileName)

			req = NewRequest(t, "GET", fmt.Sprintf("%s/download_urls", recipeURL))
			resp = MakeRequest(t, req, http.StatusOK)

			DecodeJSON(t, resp, &downloadURLs)
			assert.Contains(t, downloadURLs, conanfileName)

			req = NewRequest(t, "GET", downloadURLs[conanfileName])
			resp = MakeRequest(t, req, http.StatusOK)
			assert.Equal(t, buildConanfileContent(name, version1), resp.Body.String())

			packageURL := fmt.Sprintf("%s/packages/%s", recipeURL, conanPackageReference)

			req = NewRequest(t, "GET", packageURL)
			resp = MakeRequest(t, req, http.StatusOK)

			fileHashes = make(map[string]string)
			DecodeJSON(t, resp, &fileHashes)
			assert.Len(t, fileHashes, 1)
			assert.Contains(t, fileHashes, conaninfoName)
			assert.Equal(t, "7628bfcc5b17f1470c468621a78df394", fileHashes[conaninfoName])

			req = NewRequest(t, "GET", fmt.Sprintf("%s/digest", packageURL))
			resp = MakeRequest(t, req, http.StatusOK)

			downloadURLs = make(map[string]string)
			DecodeJSON(t, resp, &downloadURLs)
			assert.Contains(t, downloadURLs, conaninfoName)

			req = NewRequest(t, "GET", fmt.Sprintf("%s/download_urls", packageURL))
			resp = MakeRequest(t, req, http.StatusOK)

			DecodeJSON(t, resp, &downloadURLs)
			assert.Contains(t, downloadURLs, conaninfoName)

			req = NewRequest(t, "GET", downloadURLs[conaninfoName])
			resp = MakeRequest(t, req, http.StatusOK)
			assert.Equal(t, contentConaninfo, resp.Body.String())
		})

		t.Run("Search", func(t *testing.T) {
			uploadConanPackageV1(t, url, token, name, version2, user1, channel1)
			uploadConanPackageV1(t, url, token, name, version1, user1, channel2)
			uploadConanPackageV1(t, url, token, name, version1, user2, channel1)
			uploadConanPackageV1(t, url, token, name, version1, user2, channel2)

			t.Run("Recipe", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				cases := []struct {
					Query    string
					Expected []string
				}{
					{"ConanPackage", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.3@dummy/test", "ConanPackage/1.2@dummy/final", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/1.2", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.2@dummy/final", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/1.1", []string{}},
					{"Conan*", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.3@dummy/test", "ConanPackage/1.2@dummy/final", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.3@dummy/test", "ConanPackage/1.2@dummy/final", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/*", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.3@dummy/test", "ConanPackage/1.2@dummy/final", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/1*", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.3@dummy/test", "ConanPackage/1.2@dummy/final", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/*2", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.2@dummy/final", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/1*2", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.2@dummy/final", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/1.2@", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.2@dummy/final", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/1.2@du*", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.2@dummy/final"}},
					{"ConanPackage/1.2@du*/", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.2@dummy/final"}},
					{"ConanPackage/1.2@du*/*test", []string{"ConanPackage/1.2@dummy/test"}},
					{"ConanPackage/1.2@du*/*st", []string{"ConanPackage/1.2@dummy/test"}},
					{"ConanPackage/1.2@gitea/*", []string{"ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"*/*@dummy", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.3@dummy/test", "ConanPackage/1.2@dummy/final"}},
					{"*/*@*/final", []string{"ConanPackage/1.2@dummy/final", "ConanPackage/1.2@gitea/final"}},
				}

				for i, c := range cases {
					req := NewRequest(t, "GET", fmt.Sprintf("%s/v1/conans/search?q=%s", url, stdurl.QueryEscape(c.Query)))
					resp := MakeRequest(t, req, http.StatusOK)

					var result *conan_router.SearchResult
					DecodeJSON(t, resp, &result)

					assert.ElementsMatch(t, c.Expected, result.Results, "case %d: unexpected result", i)
				}
			})

			t.Run("Package", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "GET", fmt.Sprintf("%s/v1/conans/%s/%s/%s/%s/search", url, name, version1, user1, channel2))
				resp := MakeRequest(t, req, http.StatusOK)

				var result map[string]*conan_module.Conaninfo
				DecodeJSON(t, resp, &result)

				assert.Contains(t, result, conanPackageReference)
				info := result[conanPackageReference]
				assert.NotEmpty(t, info.Settings)
			})
		})

		t.Run("Delete", func(t *testing.T) {
			t.Run("Package", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				cases := []struct {
					Channel    string
					References []string
				}{
					{channel1, []string{conanPackageReference}},
					{channel2, []string{}},
				}

				for i, c := range cases {
					rref, _ := conan_module.NewRecipeReference(name, version1, user1, c.Channel, conan_module.DefaultRevision)
					references, err := conan_model.GetPackageReferences(db.DefaultContext, user.ID, rref)
					assert.NoError(t, err)
					assert.NotEmpty(t, references)

					req := NewRequestWithJSON(t, "POST", fmt.Sprintf("%s/v1/conans/%s/%s/%s/%s/packages/delete", url, name, version1, user1, c.Channel), map[string][]string{
						"package_ids": c.References,
					})
					req = addTokenAuthHeader(req, token)
					MakeRequest(t, req, http.StatusOK)

					references, err = conan_model.GetPackageReferences(db.DefaultContext, user.ID, rref)
					assert.NoError(t, err)
					assert.Empty(t, references, "case %d: should be empty", i)
				}
			})

			t.Run("Recipe", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				cases := []struct {
					Channel string
				}{
					{channel1},
					{channel2},
				}

				for i, c := range cases {
					rref, _ := conan_module.NewRecipeReference(name, version1, user1, c.Channel, conan_module.DefaultRevision)
					revisions, err := conan_model.GetRecipeRevisions(db.DefaultContext, user.ID, rref)
					assert.NoError(t, err)
					assert.NotEmpty(t, revisions)

					req := NewRequest(t, "DELETE", fmt.Sprintf("%s/v1/conans/%s/%s/%s/%s", url, name, version1, user1, c.Channel))
					req = addTokenAuthHeader(req, token)
					MakeRequest(t, req, http.StatusOK)

					revisions, err = conan_model.GetRecipeRevisions(db.DefaultContext, user.ID, rref)
					assert.NoError(t, err)
					assert.Empty(t, revisions, "case %d: should be empty", i)
				}
			})
		})
	})

	t.Run("v2", func(t *testing.T) {
		t.Run("Ping", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/v2/ping", url))
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, "revisions", resp.Header().Get("X-Conan-Server-Capabilities"))
		})

		token := ""

		t.Run("Authenticate", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/v2/users/authenticate", url))
			req = AddBasicAuthHeader(req, user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			body := resp.Body.String()
			assert.NotEmpty(t, body)

			token = fmt.Sprintf("Bearer %s", body)
		})

		t.Run("CheckCredentials", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/v2/users/check_credentials", url))
			req = addTokenAuthHeader(req, token)
			MakeRequest(t, req, http.StatusOK)
		})

		t.Run("Upload", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			uploadConanPackageV2(t, url, token, name, version1, user1, channel1, revision1, revision1)

			t.Run("Validate", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeConan)
				assert.NoError(t, err)
				assert.Len(t, pvs, 2)
			})
		})

		t.Run("Latest", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			recipeURL := fmt.Sprintf("%s/v2/conans/%s/%s/%s/%s", url, name, version1, user1, channel1)

			req := NewRequest(t, "GET", fmt.Sprintf("%s/latest", recipeURL))
			resp := MakeRequest(t, req, http.StatusOK)

			obj := make(map[string]string)
			DecodeJSON(t, resp, &obj)
			assert.Contains(t, obj, "revision")
			assert.Equal(t, revision1, obj["revision"])

			req = NewRequest(t, "GET", fmt.Sprintf("%s/revisions/%s/packages/%s/latest", recipeURL, revision1, conanPackageReference))
			resp = MakeRequest(t, req, http.StatusOK)

			obj = make(map[string]string)
			DecodeJSON(t, resp, &obj)
			assert.Contains(t, obj, "revision")
			assert.Equal(t, revision1, obj["revision"])
		})

		t.Run("ListRevisions", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			uploadConanPackageV2(t, url, token, name, version1, user1, channel1, revision1, revision2)
			uploadConanPackageV2(t, url, token, name, version1, user1, channel1, revision2, revision1)
			uploadConanPackageV2(t, url, token, name, version1, user1, channel1, revision2, revision2)

			recipeURL := fmt.Sprintf("%s/v2/conans/%s/%s/%s/%s/revisions", url, name, version1, user1, channel1)

			req := NewRequest(t, "GET", recipeURL)
			resp := MakeRequest(t, req, http.StatusOK)

			type RevisionInfo struct {
				Revision string    `json:"revision"`
				Time     time.Time `json:"time"`
			}

			type RevisionList struct {
				Revisions []*RevisionInfo `json:"revisions"`
			}

			var list *RevisionList
			DecodeJSON(t, resp, &list)
			assert.Len(t, list.Revisions, 2)
			revs := make([]string, 0, len(list.Revisions))
			for _, rev := range list.Revisions {
				revs = append(revs, rev.Revision)
			}
			assert.ElementsMatch(t, []string{revision1, revision2}, revs)

			req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/packages/%s/revisions", recipeURL, revision1, conanPackageReference))
			resp = MakeRequest(t, req, http.StatusOK)

			DecodeJSON(t, resp, &list)
			assert.Len(t, list.Revisions, 2)
			revs = make([]string, 0, len(list.Revisions))
			for _, rev := range list.Revisions {
				revs = append(revs, rev.Revision)
			}
			assert.ElementsMatch(t, []string{revision1, revision2}, revs)
		})

		t.Run("Search", func(t *testing.T) {
			t.Run("Recipe", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				cases := []struct {
					Query    string
					Expected []string
				}{
					{"ConanPackage", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.3@dummy/test", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/1.2", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/1.1", []string{}},
					{"Conan*", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.3@dummy/test", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.3@dummy/test", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/*", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.3@dummy/test", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/1*", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.3@dummy/test", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/*2", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/1*2", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/1.2@", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"ConanPackage/1.2@du*", []string{"ConanPackage/1.2@dummy/test"}},
					{"ConanPackage/1.2@du*/", []string{"ConanPackage/1.2@dummy/test"}},
					{"ConanPackage/1.2@du*/*test", []string{"ConanPackage/1.2@dummy/test"}},
					{"ConanPackage/1.2@du*/*st", []string{"ConanPackage/1.2@dummy/test"}},
					{"ConanPackage/1.2@gitea/*", []string{"ConanPackage/1.2@gitea/test", "ConanPackage/1.2@gitea/final"}},
					{"*/*@dummy", []string{"ConanPackage/1.2@dummy/test", "ConanPackage/1.3@dummy/test"}},
					{"*/*@*/final", []string{"ConanPackage/1.2@gitea/final"}},
				}

				for i, c := range cases {
					req := NewRequest(t, "GET", fmt.Sprintf("%s/v2/conans/search?q=%s", url, stdurl.QueryEscape(c.Query)))
					resp := MakeRequest(t, req, http.StatusOK)

					var result *conan_router.SearchResult
					DecodeJSON(t, resp, &result)

					assert.ElementsMatch(t, c.Expected, result.Results, "case %d: unexpected result", i)
				}
			})

			t.Run("Package", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "GET", fmt.Sprintf("%s/v2/conans/%s/%s/%s/%s/search", url, name, version1, user1, channel1))
				resp := MakeRequest(t, req, http.StatusOK)

				var result map[string]*conan_module.Conaninfo
				DecodeJSON(t, resp, &result)

				assert.Contains(t, result, conanPackageReference)
				info := result[conanPackageReference]
				assert.NotEmpty(t, info.Settings)

				req = NewRequest(t, "GET", fmt.Sprintf("%s/v2/conans/%s/%s/%s/%s/revisions/%s/search", url, name, version1, user1, channel1, revision1))
				resp = MakeRequest(t, req, http.StatusOK)

				result = make(map[string]*conan_module.Conaninfo)
				DecodeJSON(t, resp, &result)

				assert.Contains(t, result, conanPackageReference)
				info = result[conanPackageReference]
				assert.NotEmpty(t, info.Settings)
			})
		})

		t.Run("Delete", func(t *testing.T) {
			t.Run("Package", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				rref, _ := conan_module.NewRecipeReference(name, version1, user1, channel1, revision1)
				pref, _ := conan_module.NewPackageReference(rref, conanPackageReference, conan_module.DefaultRevision)

				checkPackageRevisionCount := func(count int) {
					revisions, err := conan_model.GetPackageRevisions(db.DefaultContext, user.ID, pref)
					assert.NoError(t, err)
					assert.Len(t, revisions, count)
				}
				checkPackageReferenceCount := func(count int) {
					references, err := conan_model.GetPackageReferences(db.DefaultContext, user.ID, rref)
					assert.NoError(t, err)
					assert.Len(t, references, count)
				}

				checkPackageRevisionCount(2)

				req := NewRequest(t, "DELETE", fmt.Sprintf("%s/v2/conans/%s/%s/%s/%s/revisions/%s/packages/%s/revisions/%s", url, name, version1, user1, channel1, revision1, conanPackageReference, revision1))
				req = addTokenAuthHeader(req, token)
				MakeRequest(t, req, http.StatusOK)

				checkPackageRevisionCount(1)

				req = NewRequest(t, "DELETE", fmt.Sprintf("%s/v2/conans/%s/%s/%s/%s/revisions/%s/packages/%s", url, name, version1, user1, channel1, revision1, conanPackageReference))
				req = addTokenAuthHeader(req, token)
				MakeRequest(t, req, http.StatusOK)

				checkPackageRevisionCount(0)

				rref = rref.WithRevision(revision2)

				checkPackageReferenceCount(1)

				req = NewRequest(t, "DELETE", fmt.Sprintf("%s/v2/conans/%s/%s/%s/%s/revisions/%s/packages", url, name, version1, user1, channel1, revision2))
				req = addTokenAuthHeader(req, token)
				MakeRequest(t, req, http.StatusOK)

				checkPackageReferenceCount(0)
			})

			t.Run("Recipe", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				rref, _ := conan_module.NewRecipeReference(name, version1, user1, channel1, conan_module.DefaultRevision)

				checkRecipeRevisionCount := func(count int) {
					revisions, err := conan_model.GetRecipeRevisions(db.DefaultContext, user.ID, rref)
					assert.NoError(t, err)
					assert.Len(t, revisions, count)
				}

				checkRecipeRevisionCount(2)

				req := NewRequest(t, "DELETE", fmt.Sprintf("%s/v2/conans/%s/%s/%s/%s/revisions/%s", url, name, version1, user1, channel1, revision1))
				req = addTokenAuthHeader(req, token)
				MakeRequest(t, req, http.StatusOK)

				checkRecipeRevisionCount(1)

				req = NewRequest(t, "DELETE", fmt.Sprintf("%s/v2/conans/%s/%s/%s/%s", url, name, version1, user1, channel1))
				req = addTokenAuthHeader(req, token)
				MakeRequest(t, req, http.StatusOK)

				checkRecipeRevisionCount(0)
			})
		})
	})
}
