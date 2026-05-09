// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/auth"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/routers/web"
	"code.gitea.io/gitea/tests"

	gzipp "github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/assert"
)

func storeObjectInRepo(t *testing.T, repositoryID int64, content string) string {
	pointer, err := lfs.GeneratePointer(strings.NewReader(content))
	assert.NoError(t, err)

	_, err = git_model.NewLFSMetaObject(t.Context(), repositoryID, pointer)
	assert.NoError(t, err)
	contentStore := lfs.NewContentStore()
	exist, err := contentStore.Exists(pointer)
	assert.NoError(t, err)
	if !exist {
		err := contentStore.Put(pointer, strings.NewReader(content))
		assert.NoError(t, err)
	}
	return pointer.Oid
}

func storeAndGetLfsToken(t *testing.T, content string, extraHeader *http.Header, expectedStatus int, ts ...auth.AccessTokenScope) *httptest.ResponseRecorder {
	repo, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "repo1")
	assert.NoError(t, err)
	oid := storeObjectInRepo(t, repo.ID, content)
	defer git_model.RemoveLFSMetaObjectByOid(t.Context(), repo.ID, oid)

	token := getUserToken(t, "user2", ts...)

	// Request OID
	req := NewRequest(t, "GET", "/user2/repo1.git/info/lfs/objects/"+oid+"/test")
	req.Header.Set("Accept-Encoding", "gzip")
	req.SetBasicAuth("user2", token)
	if extraHeader != nil {
		for key, values := range *extraHeader {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	resp := MakeRequest(t, req, expectedStatus)

	return resp
}

func storeAndGetLfs(t *testing.T, content string, extraHeader *http.Header, expectedStatus int) *httptest.ResponseRecorder {
	repo, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "repo1")
	assert.NoError(t, err)
	oid := storeObjectInRepo(t, repo.ID, content)
	defer git_model.RemoveLFSMetaObjectByOid(t.Context(), repo.ID, oid)

	session := loginUser(t, "user2")

	// Request OID
	req := NewRequest(t, "GET", "/user2/repo1.git/info/lfs/objects/"+oid+"/test")
	req.Header.Set("Accept-Encoding", "gzip")
	if extraHeader != nil {
		for key, values := range *extraHeader {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	resp := session.MakeRequest(t, req, expectedStatus)

	return resp
}

func checkResponseTestContentEncoding(t *testing.T, content string, resp *httptest.ResponseRecorder, expectGzip bool) {
	contentEncoding := resp.Header().Get("Content-Encoding")
	if !expectGzip || !setting.EnableGzip {
		assert.NotContains(t, contentEncoding, "gzip")
		assert.Equal(t, content, resp.Body.String())
	} else {
		assert.Contains(t, contentEncoding, "gzip")
		gzipReader, err := gzipp.NewReader(resp.Body)
		assert.NoError(t, err)
		result, err := io.ReadAll(gzipReader)
		assert.NoError(t, err)
		assert.Equal(t, content, string(result))
	}
}

func TestLFSGetObject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("GetLFSSmall", testGetLFSSmall)
	t.Run("GetLFSSmallToken", testGetLFSSmallToken)
	t.Run("GetLFSSmallTokenFail", testGetLFSSmallTokenFail)
	t.Run("GetLFSLarge", testGetLFSLarge)
	t.Run("GetLFSGzip", testGetLFSGzip)
	t.Run("GetLFSZip", testGetLFSZip)
	t.Run("GetLFSRangeNo", testGetLFSRangeNo)
	t.Run("GetLFSRange", testGetLFSRange)
}

func testGetLFSSmall(t *testing.T) {
	content := "A very small file\n"
	resp := storeAndGetLfs(t, content, nil, http.StatusOK)
	checkResponseTestContentEncoding(t, content, resp, false)
}

func testGetLFSSmallToken(t *testing.T) {
	content := "A very small file\n"
	resp := storeAndGetLfsToken(t, content, nil, http.StatusOK, auth.AccessTokenScopePublicOnly, auth.AccessTokenScopeReadRepository)
	checkResponseTestContentEncoding(t, content, resp, false)
}

func testGetLFSSmallTokenFail(t *testing.T) {
	content := "A very small file\n"
	storeAndGetLfsToken(t, content, nil, http.StatusForbidden, auth.AccessTokenScopeReadNotification)
}

func testGetLFSLarge(t *testing.T) {
	content := strings.Repeat("a", web.GzipMinSize*10)
	resp := storeAndGetLfs(t, content, nil, http.StatusOK)
	checkResponseTestContentEncoding(t, content, resp, true)
}

func testGetLFSGzip(t *testing.T) {
	s := strings.Repeat("a", web.GzipMinSize*10)
	outputBuffer := &bytes.Buffer{}
	gzipWriter := gzipp.NewWriter(outputBuffer)
	_, _ = gzipWriter.Write([]byte(s))
	_ = gzipWriter.Close()
	content := outputBuffer.String()
	resp := storeAndGetLfs(t, content, nil, http.StatusOK)
	checkResponseTestContentEncoding(t, content, resp, false)
}

func testGetLFSZip(t *testing.T) {
	b := strings.Repeat("a", web.GzipMinSize*10)
	content := test.WriteZipArchive(map[string]string{"default": b}).String()
	resp := storeAndGetLfs(t, content, nil, http.StatusOK)
	checkResponseTestContentEncoding(t, content, resp, false)
}

func testGetLFSRangeNo(t *testing.T) {
	content := "123456789\n"
	resp := storeAndGetLfs(t, content, nil, http.StatusOK)
	assert.Equal(t, content, resp.Body.String())
}

func testGetLFSRange(t *testing.T) {
	content := "123456789\n"

	cases := []struct {
		in     string
		out    string
		status int
	}{
		{"bytes=0-0", "1", http.StatusPartialContent},
		{"bytes=0-1", "12", http.StatusPartialContent},
		{"bytes=1-1", "2", http.StatusPartialContent},
		{"bytes=1-3", "234", http.StatusPartialContent},
		{"bytes=1-", "23456789\n", http.StatusPartialContent},
		// end-range smaller than start-range is ignored
		{"bytes=1-0", "23456789\n", http.StatusPartialContent},
		{"bytes=0-10", "123456789\n", http.StatusPartialContent},
		// end-range bigger than length-1 is ignored
		{"bytes=0-11", "123456789\n", http.StatusPartialContent},
		{"bytes=11-", "Requested Range Not Satisfiable", http.StatusRequestedRangeNotSatisfiable},
		// incorrect header value cause whole header to be ignored
		{"bytes=-", "123456789\n", http.StatusOK},
		{"foobar", "123456789\n", http.StatusOK},
	}

	for _, tt := range cases {
		t.Run(tt.in, func(t *testing.T) {
			h := http.Header{
				"Range": []string{tt.in},
			}
			resp := storeAndGetLfs(t, content, &h, tt.status)
			if tt.status == http.StatusPartialContent || tt.status == http.StatusOK {
				assert.Equal(t, tt.out, resp.Body.String())
			} else {
				var er lfs.ErrorResponse
				err := json.Unmarshal(resp.Body.Bytes(), &er)
				assert.NoError(t, err)
				assert.Equal(t, tt.out, er.Message)
			}
		})
	}
}
