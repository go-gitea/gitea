// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "fmt"

var RepoArchive = struct {
	Storage *Storage
}{}

func loadRepoArchiveFrom(rootCfg ConfigProvider) error {
	sec := rootCfg.Section("repo-archive")
	if err := sec.MapTo(&RepoArchive); err != nil {
		return fmt.Errorf("mapto repoarchive failed: %v", err)
	}

	RepoArchive.Storage = getStorage(rootCfg, "repo-archive", "", sec)
	return nil
}
