// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "fmt"

var RepoArchive = struct {
	*Storage
}{}

func loadRepoArchiveFrom(rootCfg ConfigProvider) error {
	sec := rootCfg.Section("repo-archive")
	if err := sec.MapTo(&RepoArchive); err != nil {
		return fmt.Errorf("mapto repoarchive failed: %v", err)
	}
	storageType := sec.Key("STORAGE_TYPE").MustString("")
	var err error
	RepoArchive.Storage, err = getStorage(rootCfg, sec, "repo-archive", storageType)
	return err
}
