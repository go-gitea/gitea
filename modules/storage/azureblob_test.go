// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"os"
	"testing"

	"code.gitea.io/gitea/modules/setting"
)

func TestAzureBlobStorageIterator(t *testing.T) {
	if os.Getenv("CI") == "" {
		t.Skip("azureBlobStorage not present outside of CI")
		return
	}
	testStorageIterator(t, setting.AzureBlobStorageType, &setting.Storage{
		AzureBlobConfig: setting.AzureBlobStorageConfig{
			// https://learn.microsoft.com/azure/storage/common/storage-use-azurite?tabs=visual-studio-code#ip-style-url
			Endpoint: "http://127.0.0.1:10000/devstoreaccount1/",
			// https://learn.microsoft.com/azure/storage/common/storage-use-azurite?tabs=visual-studio-code#well-known-storage-account-and-key
			AccountName: "devstoreaccount1",
			AccountKey:  "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==",
			Container:   "test",
		},
	})
}
