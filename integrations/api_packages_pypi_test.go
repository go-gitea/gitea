// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
)

func TestPackagePyPI(t *testing.T) {
	defer prepareTestEnv(t)()
	repository := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: repository.OwnerID}).(*models.User)

	packageName := "test-package"
	packageVersion := "1.0.1"
	packageAuthor := "KN4CK3R"
	packageDescription := "Test Description"

	content := "test"
	hashSHA256 := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

	root := fmt.Sprintf("/api/v1/repos/%s/%s/packages/pypi", user.Name, repository.Name)

	uploadFile := func(t *testing.T, filename, content string, expectedStatus int) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("content", filename)
		_, _ = io.Copy(part, strings.NewReader(content))

		writer.WriteField("name", packageName)
		writer.WriteField("version", packageVersion)
		writer.WriteField("author", packageAuthor)
		writer.WriteField("summary", packageDescription)
		writer.WriteField("description", packageDescription)
		writer.WriteField("sha256_digest", hashSHA256)
		writer.WriteField("requires_python", "3.6")

		_ = writer.Close()

		req := NewRequestWithBody(t, "POST", root, body)
		req.Header.Add("Content-Type", writer.FormDataContentType())
		req = AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, expectedStatus)
	}

	t.Run("Upload", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		filename := "test.whl"
		uploadFile(t, filename, content, http.StatusCreated)

		ps, err := models.GetPackagesByRepositoryAndType(repository.ID, models.PackagePyPI)
		assert.NoError(t, err)
		assert.Len(t, ps, 1)
		assert.Equal(t, packageName, ps[0].Name)
		assert.Equal(t, packageVersion, ps[0].Version)

		pfs, err := ps[0].GetFiles()
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, filename, pfs[0].Name)
		assert.Equal(t, int64(4), pfs[0].Size)
	})

	t.Run("UploadAddFile", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		filename := "test.tar.gz"
		uploadFile(t, filename, content, http.StatusCreated)

		ps, err := models.GetPackagesByRepositoryAndType(repository.ID, models.PackagePyPI)
		assert.NoError(t, err)
		assert.Len(t, ps, 1)
		assert.Equal(t, packageName, ps[0].Name)
		assert.Equal(t, packageVersion, ps[0].Version)

		pf, err := ps[0].GetFileByName(filename)
		assert.NoError(t, err)
		assert.NotNil(t, pf)
		assert.Equal(t, filename, pf.Name)
		assert.Equal(t, int64(4), pf.Size)

		pfs, err := ps[0].GetFiles()
		assert.NoError(t, err)
		assert.Len(t, pfs, 2)
	})

	t.Run("UploadHashMismatch", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		filename := "test2.whl"
		uploadFile(t, filename, "dummy", http.StatusBadRequest)
	})

	t.Run("UploadExists", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		uploadFile(t, "test.whl", content, http.StatusBadRequest)
		uploadFile(t, "test.tar.gz", content, http.StatusBadRequest)
	})

	t.Run("Download", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		downloadFile := func(filename string) {
			req := NewRequest(t, "GET", fmt.Sprintf("%s/files/%s/%s/%s", root, packageName, packageVersion, filename))
			req = AddBasicAuthHeader(req, user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, []byte(content), resp.Body.Bytes())
		}

		downloadFile("test.whl")
		downloadFile("test.tar.gz")
	})

	t.Run("PackageMetadata", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/simple/%s", root, packageName))
		req = AddBasicAuthHeader(req, user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)
		nodes := htmlDoc.doc.Find("a").Nodes
		assert.Len(t, nodes, 2)

		hrefMatcher := regexp.MustCompile(fmt.Sprintf(`%s/files/%s/%s/test\..+#sha256-%s`, root, packageName, packageVersion, hashSHA256))

		for _, a := range nodes {
			for _, att := range a.Attr {
				switch att.Key {
				case "href":
					assert.Regexp(t, hrefMatcher, att.Val)
				case "data-requires-python":
					assert.Equal(t, "3.6", att.Val)
				default:
					t.Fail()
				}
			}
		}
	})
}
