// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"io"
	"io/fs"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type azureBlobFailOnceTransport struct {
	failed atomic.Bool
}

func (t *azureBlobFailOnceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.failed.CompareAndSwap(false, true) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: http.NoBody}, nil
	}
	return http.DefaultTransport.RoundTrip(req)
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
	testStorageGeneral(t, newAzureBlobTestStorage(t, ""))
	testStorageGeneral(t, newAzureBlobTestStorage(t, "test-base-path"))

	s := newAzureBlobTestStorage(t, "")
	s.blockSize, s.concurrency, s.retryDelay = 4, 2, 0
	transport := &azureBlobFailOnceTransport{}
	s.client.Transport = transport

	data := "Q2xTckt6Y1hDOWh0"
	written, err := s.Save("test.txt", strings.NewReader(data), -1)
	require.NoError(t, err)
	assert.EqualValues(t, len(data), written)
	assert.True(t, transport.failed.Load())
	info, err := s.Stat("test.txt")
	require.NoError(t, err)
	assert.EqualValues(t, len(data), info.Size())

	obj, err := s.Open("test.txt")
	require.NoError(t, err)
	buf := make([]byte, 4)
	_, err = io.ReadFull(obj, buf)
	require.NoError(t, err)
	assert.Equal(t, data[:4], string(buf))
	_, err = obj.Seek(-5, io.SeekEnd)
	require.NoError(t, err)
	_, err = io.ReadFull(obj, buf)
	require.NoError(t, err)
	assert.Equal(t, data[11:15], string(buf))
	assert.NoError(t, obj.Close())

	u, err := s.ServeDirectURL("direct.txt", "direct.txt", http.MethodPut, nil)
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, u.String(), strings.NewReader("direct"))
	require.NoError(t, err)
	req.Header.Set("x-ms-blob-type", "BlockBlob")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	obj, err = s.Open("direct.txt")
	require.NoError(t, err)
	content, err := io.ReadAll(obj)
	require.NoError(t, err)
	assert.Equal(t, "direct", string(content))

	assert.NoError(t, s.Delete("test.txt"))
	assert.NoError(t, s.Delete("direct.txt"))
	info, err = s.Stat("test.txt")
	assert.ErrorIs(t, err, fs.ErrNotExist)
	assert.Nil(t, info)
}
