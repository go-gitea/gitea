// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryDumperReleaseAssetPrefersDownloadFunc(t *testing.T) {
	var downloadURLHits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&downloadURLHits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("remote"))
	}))
	t.Cleanup(server.Close)

	downloadURL := server.URL + "/asset"
	var downloadFuncCalls int32
	asset := &base.ReleaseAsset{
		Name:        "asset.txt",
		DownloadURL: &downloadURL,
		DownloadFunc: func() (io.ReadCloser, error) {
			atomic.AddInt32(&downloadFuncCalls, 1)
			return io.NopCloser(strings.NewReader("local")), nil
		},
	}
	release := &base.Release{
		TagName: "v1.0.0",
		Assets:  []*base.ReleaseAsset{asset},
	}

	baseDir := t.TempDir()
	dumper, err := NewRepositoryDumper(context.Background(), baseDir, "owner", "repo", base.MigrateOptions{})
	require.NoError(t, err)

	require.NoError(t, dumper.CreateReleases(context.Background(), release))
	assert.Equal(t, int32(1), atomic.LoadInt32(&downloadFuncCalls))
	assert.Equal(t, int32(0), atomic.LoadInt32(&downloadURLHits))

	attachRelative := filepath.Join("release_assets", release.TagName, asset.Name)
	attachPath := filepath.Join(baseDir, "owner", "repo", attachRelative)
	data, err := os.ReadFile(attachPath)
	require.NoError(t, err)
	assert.Equal(t, "local", string(data))
	require.NotNil(t, asset.DownloadURL)
	assert.Equal(t, attachRelative, *asset.DownloadURL)
}

func TestRepositoryDumperReleaseAssetUsesMigrationClient(t *testing.T) {
	oldAllowed := setting.Migrations.AllowedDomains
	oldBlocked := setting.Migrations.BlockedDomains
	oldAllowLocal := setting.Migrations.AllowLocalNetworks
	t.Cleanup(func() {
		setting.Migrations.AllowedDomains = oldAllowed
		setting.Migrations.BlockedDomains = oldBlocked
		setting.Migrations.AllowLocalNetworks = oldAllowLocal
		_ = Init()
	})

	setting.Migrations.AllowedDomains = ""
	setting.Migrations.BlockedDomains = ""
	setting.Migrations.AllowLocalNetworks = false
	require.NoError(t, Init())

	var downloadURLHits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&downloadURLHits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("remote"))
	}))
	t.Cleanup(server.Close)

	downloadURL := server.URL + "/asset"
	asset := &base.ReleaseAsset{
		Name:        "asset.txt",
		DownloadURL: &downloadURL,
	}
	release := &base.Release{
		TagName: "v1.0.0",
		Assets:  []*base.ReleaseAsset{asset},
	}

	baseDir := t.TempDir()
	dumper, err := NewRepositoryDumper(context.Background(), baseDir, "owner", "repo", base.MigrateOptions{})
	require.NoError(t, err)

	assert.Error(t, dumper.CreateReleases(context.Background(), release))
	assert.Equal(t, int32(0), atomic.LoadInt32(&downloadURLHits))
}
