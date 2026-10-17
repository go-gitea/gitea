// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type azureBlobFaultTransport struct {
	failed, truncated atomic.Bool
}

func (t *azureBlobFaultTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.failed.CompareAndSwap(false, true) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: http.NoBody}, nil
	}
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err == nil && req.Method == http.MethodGet && t.truncated.CompareAndSwap(false, true) {
		resp.Body = struct {
			io.Reader
			io.Closer
		}{io.MultiReader(io.LimitReader(resp.Body, 2), iotest.ErrReader(io.ErrUnexpectedEOF)), resp.Body}
	}
	return resp, err
}

func newAzureBlobTestStorage(t *testing.T, basePath string) *AzureBlobStorage {
	endpoint := test.ExternalServiceHTTP(t, "TEST_AZURESTORAGE_ENDPOINT", "http://devstoreaccount1.azurite.local:10000")
	objStore, err := NewStorage(setting.AzureBlobStorageType, &setting.Storage{
		AzureBlobConfig: setting.AzureBlobStorageConfig{
			Endpoint:    endpoint,
			AccountName: "devstoreaccount1",
			AccountKey:  "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==",
			Container:   "test-container",
			BasePath:    basePath,
		},
	})
	require.NoError(t, err)
	s, ok := objStore.(*AzureBlobStorage)
	require.True(t, ok)
	return s
}

func TestAzureBlobStorage(t *testing.T) {
	t.Run("General", func(t *testing.T) { testStorageGeneral(t, newAzureBlobTestStorage(t, "")) })
	t.Run("GeneralWithBasePath", func(t *testing.T) { testStorageGeneral(t, newAzureBlobTestStorage(t, "test-base-path")) })

	s := newAzureBlobTestStorage(t, "")
	s.blockSize, s.concurrency, s.retryDelay = 4, 2, 0
	transport := &azureBlobFaultTransport{}
	s.client.Transport = transport
	data := "Q2xTckt6Y1hDOWh0"

	t.Run("SaveBlocksWithRetryAndRejectTruncatedInput", func(t *testing.T) {
		written, err := s.Save("test.txt", strings.NewReader(data), -1)
		require.NoError(t, err)
		assert.EqualValues(t, len(data), written)
		assert.True(t, transport.failed.Load())
		_, err = s.Save("truncated.txt", io.MultiReader(strings.NewReader(data), iotest.ErrReader(io.ErrUnexpectedEOF)), -1)
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})

	t.Run("ReadResumesSeeksAndFailsOnChangedBlob", func(t *testing.T) {
		obj, err := s.Open("test.txt")
		require.NoError(t, err)
		defer obj.Close()
		buf := make([]byte, 4)
		_, err = io.ReadFull(obj, buf)
		require.NoError(t, err)
		assert.Equal(t, data[:4], string(buf))
		assert.True(t, transport.truncated.Load())
		_, err = obj.Seek(-5, io.SeekEnd)
		require.NoError(t, err)
		_, err = io.ReadFull(obj, buf)
		require.NoError(t, err)
		assert.Equal(t, data[11:15], string(buf))

		_, err = s.Save("test.txt", strings.NewReader("changed"), -1)
		require.NoError(t, err)
		_, err = obj.Seek(0, io.SeekStart)
		require.NoError(t, err)
		_, err = io.ReadAll(obj)
		assert.ErrorIs(t, err, azureBlobError("ConditionNotMet"))
	})

	t.Run("ServeDirectURLAllowsPut", func(t *testing.T) {
		u, err := s.ServeDirectURL("direct.txt", "direct.txt", http.MethodPut, nil)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, u.String(), strings.NewReader("direct"))
		require.NoError(t, err)
		req.Header.Set("x-ms-blob-type", "BlockBlob")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		obj, err := s.Open("direct.txt")
		require.NoError(t, err)
		defer obj.Close()
		content, err := io.ReadAll(obj)
		require.NoError(t, err)
		assert.Equal(t, "direct", string(content))
	})

	t.Run("StatAfterDeleteReturnsNilInfo", func(t *testing.T) {
		assert.NoError(t, s.Delete("test.txt"))
		assert.NoError(t, s.Delete("direct.txt"))
		info, err := s.Stat("test.txt")
		assert.ErrorIs(t, err, fs.ErrNotExist)
		assert.Equal(t, os.FileInfo(nil), info)
	})
}
