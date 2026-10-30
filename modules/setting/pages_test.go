// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPagesStorageInheritsGlobalStorage(t *testing.T) {
	cfg, err := NewConfigProviderFromData("[storage]\nSTORAGE_TYPE = minio\n")
	assert.NoError(t, err)
	assert.NoError(t, loadPagesFrom(cfg))
	assert.Equal(t, MinioStorageType, Pages.Storage.Type)
	assert.Equal(t, "pages/", Pages.Storage.MinioConfig.BasePath)
}

func TestPagesDefaults(t *testing.T) {
	cfg, err := NewConfigProviderFromData("")
	require.NoError(t, err)
	require.NoError(t, loadPagesFrom(cfg))
	assert.EqualValues(t, 100*1024*1024, Pages.MaxTarGzSize)
	assert.EqualValues(t, 512*1024*1024, Pages.MaxSiteSize)
	assert.EqualValues(t, 100*1024*1024, Pages.MaxFileSize)
	assert.Equal(t, 10000, Pages.MaxEntries)
	assert.Equal(t, 10, Pages.MaxVersionsPerSite)
	assert.Equal(t, 50, Pages.BranchSourceBatchSize)
	assert.Equal(t, 100, Pages.DomainVerificationBatchSize)
	assert.Equal(t, 15*time.Minute, Pages.DomainVerificationInterval)
	assert.True(t, Pages.RequireGiteaTXTRecord)
	assert.Empty(t, Pages.DNSResolver)
}

func TestPagesConfiguration(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`[pages]
MAX_TAR_GZ_SIZE = 200MiB
MAX_SITE_SIZE = 1GiB
MAX_FILE_SIZE = 150MiB
MAX_ENTRIES = 20000
MAX_VERSIONS_PER_SITE = 25
BRANCH_SOURCE_BATCH_SIZE = 75
DOMAIN_VERIFICATION_BATCH_SIZE = 125
DOMAIN_VERIFICATION_INTERVAL = 20m
REQUIRE_GITEA_TXT_RECORD = false
DNS_RESOLVER = [2001:db8::53]:5353
`)
	require.NoError(t, err)
	require.NoError(t, loadPagesFrom(cfg))
	assert.EqualValues(t, 200*1024*1024, Pages.MaxTarGzSize)
	assert.EqualValues(t, 1024*1024*1024, Pages.MaxSiteSize)
	assert.EqualValues(t, 150*1024*1024, Pages.MaxFileSize)
	assert.Equal(t, 20000, Pages.MaxEntries)
	assert.Equal(t, 25, Pages.MaxVersionsPerSite)
	assert.Equal(t, 75, Pages.BranchSourceBatchSize)
	assert.Equal(t, 125, Pages.DomainVerificationBatchSize)
	assert.Equal(t, 20*time.Minute, Pages.DomainVerificationInterval)
	assert.False(t, Pages.RequireGiteaTXTRecord)
	assert.Equal(t, "[2001:db8::53]:5353", Pages.DNSResolver)
}

func TestPagesDNSResolver(t *testing.T) {
	for _, testCase := range []struct {
		value      string
		normalized string
	}{
		{value: "1.1.1.1", normalized: "1.1.1.1:53"},
		{value: "1.1.1.1:5353", normalized: "1.1.1.1:5353"},
		{value: "2001:db8::53", normalized: "[2001:db8::53]:53"},
		{value: "[2001:db8::53]:5353", normalized: "[2001:db8::53]:5353"},
	} {
		t.Run(testCase.value, func(t *testing.T) {
			cfg, err := NewConfigProviderFromData("[pages]\nDNS_RESOLVER = " + testCase.value)
			require.NoError(t, err)
			require.NoError(t, loadPagesFrom(cfg))
			assert.Equal(t, testCase.normalized, Pages.DNSResolver)
		})
	}
}

func TestPagesRejectsInvalidConfiguration(t *testing.T) {
	for _, config := range []string{
		"MAX_TAR_GZ_SIZE = 0",
		"MAX_SITE_SIZE = invalid",
		"MAX_SITE_SIZE = 10MiB\nMAX_FILE_SIZE = 11MiB",
		"MAX_ENTRIES = 0",
		"MAX_ENTRIES = invalid",
		"MAX_VERSIONS_PER_SITE = 0",
		"MAX_VERSIONS_PER_SITE = 101",
		"MAX_VERSIONS_PER_SITE = invalid",
		"BRANCH_SOURCE_BATCH_SIZE = 501",
		"BRANCH_SOURCE_BATCH_SIZE = invalid",
		"DOMAIN_VERIFICATION_BATCH_SIZE = 0",
		"DOMAIN_VERIFICATION_INTERVAL = 30s",
		"DOMAIN_VERIFICATION_INTERVAL = 25h",
		"DOMAIN_VERIFICATION_INTERVAL = invalid",
		"REQUIRE_GITEA_TXT_RECORD = invalid",
		"DNS_RESOLVER = resolver.example.com",
		"DNS_RESOLVER = 1.1.1.1:0",
		"DNS_RESOLVER = 1.1.1.1:dns",
	} {
		cfg, err := NewConfigProviderFromData("[pages]\n" + config)
		require.NoError(t, err)
		assert.Error(t, loadPagesFrom(cfg), config)
	}
}

func TestPagesStorageCanOverridePath(t *testing.T) {
	cfg, err := NewConfigProviderFromData("[storage.pages]\nPATH = page-content\n")
	assert.NoError(t, err)
	assert.NoError(t, loadPagesFrom(cfg))
	assert.Equal(t, LocalStorageType, Pages.Storage.Type)
	assert.Equal(t, filepath.Join(AppDataPath, "page-content"), Pages.Storage.Path)
}
