// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/packages/pypi"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPackagePyPI(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	packageName := "test-package"
	packageVersion := "1!1.0.1+r1234"
	packageAuthor := "KN4CK3R"
	packageDescription := "Test Description"

	content := "test"
	hashSHA256 := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

	root := fmt.Sprintf("/api/packages/%s/pypi", user.Name)

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

		req := NewRequestWithBody(t, "POST", root, body).
			SetHeader("Content-Type", writer.FormDataContentType()).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, expectedStatus)
	}

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		filename := "test.whl"
		uploadFile(t, filename, content, http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypePyPI)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		assert.NoError(t, err)
		assert.Nil(t, pd.SemVer)
		assert.IsType(t, &pypi.Metadata{}, pd.Metadata)
		assert.Equal(t, packageName, pd.Package.Name)
		assert.Equal(t, packageVersion, pd.Version.Version)

		pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, filename, pfs[0].Name)
		assert.True(t, pfs[0].IsLead)

		pb, err := packages.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
		assert.NoError(t, err)
		assert.Equal(t, int64(4), pb.Size)
	})

	t.Run("UploadAddFile", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		filename := "test.tar.gz"
		uploadFile(t, filename, content, http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypePyPI)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		assert.NoError(t, err)
		assert.Nil(t, pd.SemVer)
		assert.IsType(t, &pypi.Metadata{}, pd.Metadata)
		assert.Equal(t, packageName, pd.Package.Name)
		assert.Equal(t, packageVersion, pd.Version.Version)

		pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 2)

		pf, err := packages.GetFileForVersionByName(db.DefaultContext, pvs[0].ID, filename, packages.EmptyFileKey)
		assert.NoError(t, err)
		assert.Equal(t, filename, pf.Name)
		assert.True(t, pf.IsLead)

		pb, err := packages.GetBlobByID(db.DefaultContext, pf.BlobID)
		assert.NoError(t, err)
		assert.Equal(t, int64(4), pb.Size)
	})

	t.Run("UploadHashMismatch", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		filename := "test2.whl"
		uploadFile(t, filename, "dummy", http.StatusBadRequest)
	})

	t.Run("UploadExists", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		uploadFile(t, "test.whl", content, http.StatusConflict)
		uploadFile(t, "test.tar.gz", content, http.StatusConflict)
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		downloadFile := func(filename string) {
			req := NewRequest(t, "GET", fmt.Sprintf("%s/files/%s/%s/%s", root, packageName, packageVersion, filename)).
				AddBasicAuth(user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, []byte(content), resp.Body.Bytes())
		}

		downloadFile("test.whl")
		downloadFile("test.tar.gz")

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypePyPI)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)
		assert.Equal(t, int64(2), pvs[0].DownloadCount)
	})

	t.Run("PackageMetadata", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/simple/%s", root, packageName)).
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)
		nodes := htmlDoc.doc.Find("a").Nodes
		assert.Len(t, nodes, 2)

		hrefMatcher := regexp.MustCompile(fmt.Sprintf(`%s/files/%s/%s/test\..+#sha256=%s`, root, regexp.QuoteMeta(packageName), regexp.QuoteMeta(packageVersion), hashSHA256))

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
