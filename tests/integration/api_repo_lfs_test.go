// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"net/http"
	"path"
	"strconv"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPILFSNotStarted(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	setting.LFS.StartServer = false

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	req := NewRequestf(t, "POST", "/%s/%s.git/info/lfs/objects/batch", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "PUT", "/%s/%s.git/info/lfs/objects/oid/10", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "GET", "/%s/%s.git/info/lfs/objects/oid/name", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "GET", "/%s/%s.git/info/lfs/objects/oid", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "POST", "/%s/%s.git/info/lfs/verify", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPILFSMediaType(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	setting.LFS.StartServer = true

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	req := NewRequestf(t, "POST", "/%s/%s.git/info/lfs/objects/batch", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusUnsupportedMediaType)
	req = NewRequestf(t, "POST", "/%s/%s.git/info/lfs/verify", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusUnsupportedMediaType)
}

func createLFSTestRepository(t *testing.T, name string) *repo_model.Repository {
	ctx := NewAPITestContext(t, "user2", "lfs-"+name+"-repo", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
	t.Run("CreateRepo", doAPICreateRepository(ctx, false))

	repo, err := repo_model.GetRepositoryByOwnerAndName(db.DefaultContext, "user2", "lfs-"+name+"-repo")
	assert.NoError(t, err)

	return repo
}

func TestAPILFSBatch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	setting.LFS.StartServer = true

	repo := createLFSTestRepository(t, "batch")

	content := []byte("dummy1")
	oid := storeObjectInRepo(t, repo.ID, &content)
	defer git_model.RemoveLFSMetaObjectByOid(db.DefaultContext, repo.ID, oid)

	session := loginUser(t, "user2")

	newRequest := func(t testing.TB, br *lfs.BatchRequest) *RequestWrapper {
		return NewRequestWithJSON(t, "POST", "/user2/lfs-batch-repo.git/info/lfs/objects/batch", br).
			SetHeader("Accept", lfs.AcceptHeader).
			SetHeader("Content-Type", lfs.MediaType)
	}
	decodeResponse := func(t *testing.T, b *bytes.Buffer) *lfs.BatchResponse {
		var br lfs.BatchResponse

		assert.NoError(t, json.Unmarshal(b.Bytes(), &br))
		return &br
	}

	t.Run("InvalidJsonRequest", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := newRequest(t, nil)

		session.MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("InvalidOperation", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := newRequest(t, &lfs.BatchRequest{
			Operation: "dummy",
		})

		session.MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("InvalidPointer", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := newRequest(t, &lfs.BatchRequest{
			Operation: "download",
			Objects: []lfs.Pointer{
				{Oid: "dummy"},
				{Oid: oid, Size: -1},
			},
		})

		resp := session.MakeRequest(t, req, http.StatusOK)
		br := decodeResponse(t, resp.Body)
		assert.Len(t, br.Objects, 2)
		assert.Equal(t, "dummy", br.Objects[0].Oid)
		assert.Equal(t, oid, br.Objects[1].Oid)
		assert.Equal(t, int64(0), br.Objects[0].Size)
		assert.Equal(t, int64(-1), br.Objects[1].Size)
		assert.NotNil(t, br.Objects[0].Error)
		assert.NotNil(t, br.Objects[1].Error)
		assert.Equal(t, http.StatusUnprocessableEntity, br.Objects[0].Error.Code)
		assert.Equal(t, http.StatusUnprocessableEntity, br.Objects[1].Error.Code)
		assert.Equal(t, "Oid or size are invalid", br.Objects[0].Error.Message)
		assert.Equal(t, "Oid or size are invalid", br.Objects[1].Error.Message)
	})

	t.Run("PointerSizeMismatch", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := newRequest(t, &lfs.BatchRequest{
			Operation: "download",
			Objects: []lfs.Pointer{
				{Oid: oid, Size: 1},
			},
		})

		resp := session.MakeRequest(t, req, http.StatusOK)
		br := decodeResponse(t, resp.Body)
		assert.Len(t, br.Objects, 1)
		assert.NotNil(t, br.Objects[0].Error)
		assert.Equal(t, http.StatusUnprocessableEntity, br.Objects[0].Error.Code)
		assert.Equal(t, "Object "+oid+" is not 1 bytes", br.Objects[0].Error.Message)
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		t.Run("PointerNotInStore", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := newRequest(t, &lfs.BatchRequest{
				Operation: "download",
				Objects: []lfs.Pointer{
					{Oid: "fb8f7d8435968c4f82a726a92395be4d16f2f63116caf36c8ad35c60831ab042", Size: 6},
				},
			})

			resp := session.MakeRequest(t, req, http.StatusOK)
			br := decodeResponse(t, resp.Body)
			assert.Len(t, br.Objects, 1)
			assert.NotNil(t, br.Objects[0].Error)
			assert.Equal(t, http.StatusNotFound, br.Objects[0].Error.Code)
		})

		t.Run("MetaNotFound", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			p := lfs.Pointer{Oid: "05eeb4eb5be71f2dd291ca39157d6d9effd7d1ea19cbdc8a99411fe2a8f26a00", Size: 6}

			contentStore := lfs.NewContentStore()
			exist, err := contentStore.Exists(p)
			assert.NoError(t, err)
			assert.False(t, exist)
			err = contentStore.Put(p, bytes.NewReader([]byte("dummy0")))
			assert.NoError(t, err)

			req := newRequest(t, &lfs.BatchRequest{
				Operation: "download",
				Objects:   []lfs.Pointer{p},
			})

			resp := session.MakeRequest(t, req, http.StatusOK)
			br := decodeResponse(t, resp.Body)
			assert.Len(t, br.Objects, 1)
			assert.NotNil(t, br.Objects[0].Error)
			assert.Equal(t, http.StatusNotFound, br.Objects[0].Error.Code)
		})

		t.Run("Success", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := newRequest(t, &lfs.BatchRequest{
				Operation: "download",
				Objects: []lfs.Pointer{
					{Oid: oid, Size: 6},
				},
			})

			resp := session.MakeRequest(t, req, http.StatusOK)
			br := decodeResponse(t, resp.Body)
			assert.Len(t, br.Objects, 1)
			assert.Nil(t, br.Objects[0].Error)
			assert.Contains(t, br.Objects[0].Actions, "download")
			l := br.Objects[0].Actions["download"]
			assert.NotNil(t, l)
			assert.NotEmpty(t, l.Href)
		})
	})

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		t.Run("FileTooBig", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			oldMaxFileSize := setting.LFS.MaxFileSize
			setting.LFS.MaxFileSize = 2

			req := newRequest(t, &lfs.BatchRequest{
				Operation: "upload",
				Objects: []lfs.Pointer{
					{Oid: "fb8f7d8435968c4f82a726a92395be4d16f2f63116caf36c8ad35c60831ab042", Size: 6},
				},
			})

			resp := session.MakeRequest(t, req, http.StatusOK)
			br := decodeResponse(t, resp.Body)
			assert.Len(t, br.Objects, 1)
			assert.NotNil(t, br.Objects[0].Error)
			assert.Equal(t, http.StatusUnprocessableEntity, br.Objects[0].Error.Code)
			assert.Equal(t, "Size must be less than or equal to 2", br.Objects[0].Error.Message)

			setting.LFS.MaxFileSize = oldMaxFileSize
		})

		t.Run("AddMeta", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			p := lfs.Pointer{Oid: "05eeb4eb5be71f2dd291ca39157d6d9effd7d1ea19cbdc8a99411fe2a8f26a00", Size: 6}

			contentStore := lfs.NewContentStore()
			exist, err := contentStore.Exists(p)
			assert.NoError(t, err)
			assert.True(t, exist)

			repo2 := createLFSTestRepository(t, "batch2")
			content := []byte("dummy0")
			storeObjectInRepo(t, repo2.ID, &content)

			meta, err := git_model.GetLFSMetaObjectByOid(db.DefaultContext, repo.ID, p.Oid)
			assert.Nil(t, meta)
			assert.Equal(t, git_model.ErrLFSObjectNotExist, err)

			req := newRequest(t, &lfs.BatchRequest{
				Operation: "upload",
				Objects:   []lfs.Pointer{p},
			})

			resp := session.MakeRequest(t, req, http.StatusOK)
			br := decodeResponse(t, resp.Body)
			assert.Len(t, br.Objects, 1)
			assert.Nil(t, br.Objects[0].Error)
			assert.Empty(t, br.Objects[0].Actions)

			meta, err = git_model.GetLFSMetaObjectByOid(db.DefaultContext, repo.ID, p.Oid)
			assert.NoError(t, err)
			assert.NotNil(t, meta)

			// Cleanup
			err = contentStore.Delete(p.RelativePath())
			assert.NoError(t, err)
		})

		t.Run("AlreadyExists", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := newRequest(t, &lfs.BatchRequest{
				Operation: "upload",
				Objects: []lfs.Pointer{
					{Oid: oid, Size: 6},
				},
			})

			resp := session.MakeRequest(t, req, http.StatusOK)
			br := decodeResponse(t, resp.Body)
			assert.Len(t, br.Objects, 1)
			assert.Nil(t, br.Objects[0].Error)
			assert.Empty(t, br.Objects[0].Actions)
		})

		t.Run("NewFile", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := newRequest(t, &lfs.BatchRequest{
				Operation: "upload",
				Objects: []lfs.Pointer{
					{Oid: "d6f175817f886ec6fbbc1515326465fa96c3bfd54a4ea06cfd6dbbd8340e0153", Size: 1},
				},
			})

			resp := session.MakeRequest(t, req, http.StatusOK)
			br := decodeResponse(t, resp.Body)
			assert.Len(t, br.Objects, 1)
			assert.Nil(t, br.Objects[0].Error)
			assert.Contains(t, br.Objects[0].Actions, "upload")
			ul := br.Objects[0].Actions["upload"]
			assert.NotNil(t, ul)
			assert.NotEmpty(t, ul.Href)
			assert.Contains(t, br.Objects[0].Actions, "verify")
			vl := br.Objects[0].Actions["verify"]
			assert.NotNil(t, vl)
			assert.NotEmpty(t, vl.Href)
		})
	})
}

func TestAPILFSUpload(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	setting.LFS.StartServer = true

	repo := createLFSTestRepository(t, "upload")

	content := []byte("dummy3")
	oid := storeObjectInRepo(t, repo.ID, &content)
	defer git_model.RemoveLFSMetaObjectByOid(db.DefaultContext, repo.ID, oid)

	session := loginUser(t, "user2")

	newRequest := func(t testing.TB, p lfs.Pointer, content string) *RequestWrapper {
		return NewRequestWithBody(t, "PUT", path.Join("/user2/lfs-upload-repo.git/info/lfs/objects/", p.Oid, strconv.FormatInt(p.Size, 10)), strings.NewReader(content))
	}

	t.Run("InvalidPointer", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := newRequest(t, lfs.Pointer{Oid: "dummy"}, "")

		session.MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("AlreadyExistsInStore", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		p := lfs.Pointer{Oid: "83de2e488b89a0aa1c97496b888120a28b0c1e15463a4adb8405578c540f36d4", Size: 6}

		contentStore := lfs.NewContentStore()
		exist, err := contentStore.Exists(p)
		assert.NoError(t, err)
		assert.False(t, exist)
		err = contentStore.Put(p, bytes.NewReader([]byte("dummy5")))
		assert.NoError(t, err)

		meta, err := git_model.GetLFSMetaObjectByOid(db.DefaultContext, repo.ID, p.Oid)
		assert.Nil(t, meta)
		assert.Equal(t, git_model.ErrLFSObjectNotExist, err)

		t.Run("InvalidAccess", func(t *testing.T) {
			req := newRequest(t, p, "invalid")
			session.MakeRequest(t, req, http.StatusUnprocessableEntity)
		})

		t.Run("ValidAccess", func(t *testing.T) {
			req := newRequest(t, p, "dummy5")

			session.MakeRequest(t, req, http.StatusOK)
			meta, err = git_model.GetLFSMetaObjectByOid(db.DefaultContext, repo.ID, p.Oid)
			assert.NoError(t, err)
			assert.NotNil(t, meta)
		})

		// Cleanup
		err = contentStore.Delete(p.RelativePath())
		assert.NoError(t, err)
	})

	t.Run("MetaAlreadyExists", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := newRequest(t, lfs.Pointer{Oid: oid, Size: 6}, "")

		session.MakeRequest(t, req, http.StatusOK)
	})

	t.Run("HashMismatch", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := newRequest(t, lfs.Pointer{Oid: "2581dd7bbc1fe44726de4b7dd806a087a978b9c5aec0a60481259e34be09b06a", Size: 1}, "a")

		session.MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("SizeMismatch", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := newRequest(t, lfs.Pointer{Oid: "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb", Size: 2}, "a")

		session.MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("Success", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		p := lfs.Pointer{Oid: "6ccce4863b70f258d691f59609d31b4502e1ba5199942d3bc5d35d17a4ce771d", Size: 5}

		req := newRequest(t, p, "gitea")

		session.MakeRequest(t, req, http.StatusOK)

		contentStore := lfs.NewContentStore()
		exist, err := contentStore.Exists(p)
		assert.NoError(t, err)
		assert.True(t, exist)

		meta, err := git_model.GetLFSMetaObjectByOid(db.DefaultContext, repo.ID, p.Oid)
		assert.NoError(t, err)
		assert.NotNil(t, meta)
	})
}

func TestAPILFSVerify(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	setting.LFS.StartServer = true

	repo := createLFSTestRepository(t, "verify")

	content := []byte("dummy3")
	oid := storeObjectInRepo(t, repo.ID, &content)
	defer git_model.RemoveLFSMetaObjectByOid(db.DefaultContext, repo.ID, oid)

	session := loginUser(t, "user2")

	newRequest := func(t testing.TB, p *lfs.Pointer) *RequestWrapper {
		return NewRequestWithJSON(t, "POST", "/user2/lfs-verify-repo.git/info/lfs/verify", p).
			SetHeader("Accept", lfs.AcceptHeader).
			SetHeader("Content-Type", lfs.MediaType)
	}

	t.Run("InvalidJsonRequest", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := newRequest(t, nil)

		session.MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("InvalidPointer", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := newRequest(t, &lfs.Pointer{})

		session.MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("PointerNotExisting", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := newRequest(t, &lfs.Pointer{Oid: "fb8f7d8435968c4f82a726a92395be4d16f2f63116caf36c8ad35c60831ab042", Size: 6})

		session.MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Success", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := newRequest(t, &lfs.Pointer{Oid: oid, Size: 6})

		session.MakeRequest(t, req, http.StatusOK)
	})
}
