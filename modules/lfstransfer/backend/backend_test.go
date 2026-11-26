// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package backend

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"gitea.dev/modules/json"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/charmbracelet/git-lfs-transfer/transfer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noopLogger struct{}

func (noopLogger) Log(string, ...any) {}

func encodeActionID(t *testing.T, actionName, href string) string {
	t.Helper()
	b, err := json.Marshal(map[string]*lfs.Link{
		actionName: lfs.NewLink(href),
	})
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(b)
}

func TestDownloadServeDirectExternalHref(t *testing.T) {
	const testOID = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	payload := []byte("hello-direct-lfs-over-ssh")

	var requestedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("X-Gitea-LFS-Content-Length", strconv.Itoa(len(payload)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	defer test.MockVariableValue(&setting.LocalURL, srv.URL+"/")()
	defer test.MockVariableValue(&setting.AppSubURL, "")()
	defer test.MockVariableValue(&setting.InternalToken, "mock-token")()

	b, err := New(t.Context(), "api/internal/repo/owner/repo.git/info/lfs", "download", "Bearer lfs-token", noopLogger{})
	require.NoError(t, err)

	args := transfer.Args{
		argID: encodeActionID(t, actionDownload, "https://s3.example.com/bucket/ab/cd/"+testOID+"?X-Amz-Signature=sig"),
	}
	rc, size, err := b.Download(testOID, args)
	require.NoError(t, err)
	defer rc.Close()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, int64(len(payload)), size)
	assert.Equal(t, payload, got)
	assert.Equal(t, "/api/internal/repo/owner/repo.git/info/lfs/objects/"+testOID, requestedPath)
}

func TestNonInternalURLReturnsErrorWithoutPanic(t *testing.T) {
	defer test.MockVariableValue(&setting.LocalURL, "http://localurl/")()
	defer test.MockVariableValue(&setting.AppSubURL, "")()
	defer test.MockVariableValue(&setting.InternalToken, "mock-token")()

	b, err := New(t.Context(), "api/internal/repo/owner/repo.git/info/lfs", "download", "Bearer lfs-token", noopLogger{})
	require.NoError(t, err)

	t.Run("Download empty or non-HTTP href", func(t *testing.T) {
		for _, href := range []string{"", "not-a-url", "ftp://example.com/file"} {
			args := transfer.Args{argID: encodeActionID(t, actionDownload, href)}
			_, _, err := b.Download("abcdef", args)
			assert.Error(t, err, href)
		}
		args := transfer.Args{argID: encodeActionID(t, actionDownload, "https://s3.example.com/obj")}
		_, _, err := b.Download("..", args)
		assert.Error(t, err)
	})

	t.Run("Upload non-internal href", func(t *testing.T) {
		args := transfer.Args{argID: encodeActionID(t, actionUpload, "https://s3.example.com/upload")}
		err := b.Upload("abcdef", 4, bytes.NewReader([]byte("test")), args)
		assert.Error(t, err)
	})

	t.Run("Verify non-internal href", func(t *testing.T) {
		args := transfer.Args{argID: encodeActionID(t, actionVerify, "https://s3.example.com/verify")}
		_, err := b.Verify("abcdef", 4, args)
		assert.Error(t, err)
	})

	t.Run("Batch and LockBackend non-internal server", func(t *testing.T) {
		badBackend, err := New(t.Context(), "api/other/invalid", "download", "Bearer lfs-token", noopLogger{})
		require.NoError(t, err)

		_, err = badBackend.Batch("", []transfer.BatchItem{{Pointer: transfer.Pointer{Oid: "abcdef", Size: 4}}}, transfer.Args{})
		assert.Error(t, err)

		lb := badBackend.LockBackend(transfer.Args{})
		_, err = lb.Create("file.bin", "refs/heads/main")
		assert.Error(t, err)

		_, err = lb.FromPath("file.bin")
		assert.Error(t, err)

		glb, ok := lb.(*giteaLockBackend)
		require.True(t, ok)
		err = lb.Unlock(newGiteaLock(glb, "1", "file.bin", time.Now(), "user"))
		assert.Error(t, err)
	})
}
