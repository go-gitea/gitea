// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"gitea.dev/modules/setting"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageWithContextRemoteCancellation(t *testing.T) {
	for _, operation := range []string{"minio_stat", "minio_save", "azure_stat", "azure_save"} {
		t.Run(operation, func(t *testing.T) {
			started := make(chan struct{}, 1)
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				select {
				case started <- struct{}{}:
				default:
				}
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			defer server.Close()
			defer close(release)
			var original ObjectStorage
			if strings.HasPrefix(operation, "minio_") {
				client, err := minio.New(strings.TrimPrefix(server.URL, "http://"), &minio.Options{Region: "us-east-1"})
				require.NoError(t, err)
				original = &MinioStorage{cfg: &setting.MinioStorageConfig{}, ctx: t.Context(), client: client, bucket: "copy-test"}
			} else {
				client, err := azblob.NewClientWithNoCredential(server.URL, nil)
				require.NoError(t, err)
				original = &AzureBlobStorage{cfg: &setting.AzureBlobStorageConfig{Container: "copy-test"}, ctx: t.Context(), client: client}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			scoped, err := WithContext(ctx, original)
			require.NoError(t, err)
			result := make(chan error, 1)
			go func() {
				var err error
				if strings.HasSuffix(operation, "_stat") {
					_, err = scoped.Stat("object")
				} else {
					_, err = scoped.Save("object", strings.NewReader("x"), 1)
				}
				result <- err
			}()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("storage request did not reach server")
			}
			cancel()
			select {
			case err := <-result:
				require.Error(t, err)
			case <-time.After(3 * time.Second):
				t.Fatal("storage request outlived cancellation")
			}
			switch original := original.(type) {
			case *MinioStorage:
				assert.NoError(t, original.ctx.Err())
			case *AzureBlobStorage:
				assert.NoError(t, original.ctx.Err())
			}
		})
	}
}

type failingCopyReader struct{ cancel context.CancelFunc }

func (r *failingCopyReader) Read(p []byte) (int, error) {
	copy(p, "partial")
	if r.cancel != nil {
		r.cancel()
		return 7, nil
	}
	return 7, errors.New("injected storage read failure")
}

func TestStorageWithContextLocalCleanup(t *testing.T) {
	for _, mode := range []string{"cancelled", "read_failure"} {
		t.Run(mode, func(t *testing.T) {
			original, err := NewLocalStorage(t.Context(), &setting.Storage{Path: t.TempDir()})
			require.NoError(t, err)
			_, err = original.Save("shared", strings.NewReader("keep"), 4)
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			scoped, err := WithContext(ctx, original)
			require.NoError(t, err)
			reader := &failingCopyReader{}
			if mode == "cancelled" {
				reader.cancel = cancel
			}
			_, err = scoped.Save("shared", reader, 7)
			require.Error(t, err)
			object, err := original.Open("shared")
			require.NoError(t, err)
			defer object.Close()
			content, err := io.ReadAll(object)
			require.NoError(t, err)
			assert.Equal(t, "keep", string(content))
			entries, err := os.ReadDir(original.(*LocalStorage).tmpdir)
			require.NoError(t, err)
			assert.Empty(t, entries)
			if mode == "cancelled" {
				_, err := scoped.Stat("shared")
				assert.ErrorIs(t, err, context.Canceled)
			}
		})
	}
}
