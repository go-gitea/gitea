// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/packages/npm"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestPackageNpm(t *testing.T) {
	defer prepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)

	token := fmt.Sprintf("Bearer %s", getTokenForLoggedInUser(t, loginUser(t, user.Name)))

	packageName := "@scope/test-package"
	packageVersion := "1.0.1-pre"
	packageTag := "latest"
	packageTag2 := "release"
	packageAuthor := "KN4CK3R"
	packageDescription := "Test Description"

	data := "H4sIAAAAAAAA/ytITM5OTE/VL4DQelnF+XkMVAYGBgZmJiYK2MRBwNDcSIHB2NTMwNDQzMwAqA7IMDUxA9LUdgg2UFpcklgEdAql5kD8ogCnhwio5lJQUMpLzE1VslJQcihOzi9I1S9JLS7RhSYIJR2QgrLUouLM/DyQGkM9Az1D3YIiqExKanFyUWZBCVQ2BKhVwQVJDKwosbQkI78IJO/tZ+LsbRykxFXLNdA+HwWjYBSMgpENACgAbtAACAAA"
	upload := `{
		"_id": "` + packageName + `",
		"name": "` + packageName + `",
		"description": "` + packageDescription + `",
		"dist-tags": {
		  "` + packageTag + `": "` + packageVersion + `"
		},
		"versions": {
		  "` + packageVersion + `": {
			"name": "` + packageName + `",
			"version": "` + packageVersion + `",
			"description": "` + packageDescription + `",
			"author": {
			  "name": "` + packageAuthor + `"
			},
			"dist": {
			  "integrity": "sha512-yA4FJsVhetynGfOC1jFf79BuS+jrHbm0fhh+aHzCQkOaOBXKf9oBnC4a6DnLLnEsHQDRLYd00cwj8sCXpC+wIg==",
			  "shasum": "aaa7eaf852a948b0aa05afeda35b1badca155d90"
			}
		  }
		},
		"_attachments": {
		  "` + packageName + `-` + packageVersion + `.tgz": {
			"data": "` + data + `"
		  }
		}
	  }`

	root := fmt.Sprintf("/api/packages/%s/npm/%s", user.Name, url.QueryEscape(packageName))
	tagsRoot := fmt.Sprintf("/api/packages/%s/npm/-/package/%s/dist-tags", user.Name, url.QueryEscape(packageName))
	filename := fmt.Sprintf("%s-%s.tgz", strings.Split(packageName, "/")[1], packageVersion)

	t.Run("Upload", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequestWithBody(t, "PUT", root, strings.NewReader(upload))
		req = addTokenAuthHeader(req, token)
		MakeRequest(t, req, http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeNpm)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		assert.NoError(t, err)
		assert.NotNil(t, pd.SemVer)
		assert.IsType(t, &npm.Metadata{}, pd.Metadata)
		assert.Equal(t, packageName, pd.Package.Name)
		assert.Equal(t, packageVersion, pd.Version.Version)
		assert.Len(t, pd.Properties, 1)
		assert.Equal(t, npm.TagProperty, pd.Properties[0].Name)
		assert.Equal(t, packageTag, pd.Properties[0].Value)

		pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, filename, pfs[0].Name)
		assert.True(t, pfs[0].IsLead)

		pb, err := packages.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
		assert.NoError(t, err)
		assert.Equal(t, int64(192), pb.Size)
	})

	t.Run("UploadExists", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequestWithBody(t, "PUT", root, strings.NewReader(upload))
		req = addTokenAuthHeader(req, token)
		MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("Download", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/-/%s/%s", root, packageVersion, filename))
		req = addTokenAuthHeader(req, token)
		resp := MakeRequest(t, req, http.StatusOK)

		b, _ := base64.StdEncoding.DecodeString(data)
		assert.Equal(t, b, resp.Body.Bytes())

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeNpm)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)
		assert.Equal(t, int64(1), pvs[0].DownloadCount)
	})

	t.Run("PackageMetadata", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/packages/%s/npm/%s", user.Name, "does-not-exist"))
		req = addTokenAuthHeader(req, token)
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "GET", root)
		req = addTokenAuthHeader(req, token)
		resp := MakeRequest(t, req, http.StatusOK)

		var result npm.PackageMetadata
		DecodeJSON(t, resp, &result)

		assert.Equal(t, packageName, result.ID)
		assert.Equal(t, packageName, result.Name)
		assert.Equal(t, packageDescription, result.Description)
		assert.Contains(t, result.DistTags, packageTag)
		assert.Equal(t, packageVersion, result.DistTags[packageTag])
		assert.Equal(t, packageAuthor, result.Author.Name)
		assert.Contains(t, result.Versions, packageVersion)
		pmv := result.Versions[packageVersion]
		assert.Equal(t, fmt.Sprintf("%s@%s", packageName, packageVersion), pmv.ID)
		assert.Equal(t, packageName, pmv.Name)
		assert.Equal(t, packageDescription, pmv.Description)
		assert.Equal(t, packageAuthor, pmv.Author.Name)
		assert.Equal(t, "sha512-yA4FJsVhetynGfOC1jFf79BuS+jrHbm0fhh+aHzCQkOaOBXKf9oBnC4a6DnLLnEsHQDRLYd00cwj8sCXpC+wIg==", pmv.Dist.Integrity)
		assert.Equal(t, "aaa7eaf852a948b0aa05afeda35b1badca155d90", pmv.Dist.Shasum)
		assert.Equal(t, fmt.Sprintf("%s%s/-/%s/%s", setting.AppURL, root[1:], packageVersion, filename), pmv.Dist.Tarball)
	})

	t.Run("AddTag", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		test := func(t *testing.T, status int, tag, version string) {
			req := NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/%s", tagsRoot, tag), strings.NewReader(`"`+version+`"`))
			req = addTokenAuthHeader(req, token)
			MakeRequest(t, req, status)
		}

		test(t, http.StatusBadRequest, "1.0", packageVersion)
		test(t, http.StatusBadRequest, "v1.0", packageVersion)
		test(t, http.StatusNotFound, packageTag2, "1.2")
		test(t, http.StatusOK, packageTag, packageVersion)
		test(t, http.StatusOK, packageTag2, packageVersion)
	})

	t.Run("ListTags", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", tagsRoot)
		req = addTokenAuthHeader(req, token)
		resp := MakeRequest(t, req, http.StatusOK)

		var result map[string]string
		DecodeJSON(t, resp, &result)

		assert.Len(t, result, 2)
		assert.Contains(t, result, packageTag)
		assert.Equal(t, packageVersion, result[packageTag])
		assert.Contains(t, result, packageTag2)
		assert.Equal(t, packageVersion, result[packageTag2])
	})

	t.Run("PackageMetadataDistTags", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", root)
		req = addTokenAuthHeader(req, token)
		resp := MakeRequest(t, req, http.StatusOK)

		var result npm.PackageMetadata
		DecodeJSON(t, resp, &result)

		assert.Len(t, result.DistTags, 2)
		assert.Contains(t, result.DistTags, packageTag)
		assert.Equal(t, packageVersion, result.DistTags[packageTag])
		assert.Contains(t, result.DistTags, packageTag2)
		assert.Equal(t, packageVersion, result.DistTags[packageTag2])
	})

	t.Run("DeleteTag", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		test := func(t *testing.T, status int, tag string) {
			req := NewRequest(t, "DELETE", fmt.Sprintf("%s/%s", tagsRoot, tag))
			req = addTokenAuthHeader(req, token)
			MakeRequest(t, req, status)
		}

		test(t, http.StatusBadRequest, "v1.0")
		test(t, http.StatusBadRequest, "1.0")
		test(t, http.StatusOK, "dummy")
		test(t, http.StatusOK, packageTag2)
	})
}
