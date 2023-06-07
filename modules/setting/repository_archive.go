// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "fmt"

var RepoArchive = struct {
	Storage *Storage
}{}

func loadRepoArchiveFrom(rootCfg ConfigProvider) (err error) {
	sec, _ := rootCfg.GetSection("repo-archive")
	if sec == nil {
		RepoArchive.Storage, err = getStorage(rootCfg, "repo-archive", "", nil)
		return err
	}

	if err := sec.MapTo(&RepoArchive); err != nil {
		return fmt.Errorf("mapto repoarchive failed: %v", err)
	}

	RepoArchive.Storage, err = getStorage(rootCfg, "repo-archive", "", sec)
	return err
}
