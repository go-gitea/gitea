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
			Endpoint:    "https://.blob.core.windows.net/",
			AccountName: "",
			AccountKey:  "",
			Container:   "test",
		},
	})
}
