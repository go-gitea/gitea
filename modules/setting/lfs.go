// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"time"
)

// LFS represents the configuration for Git LFS
var LFS = struct {
	StartServer    bool          `ini:"LFS_START_SERVER"`
	HTTPAuthExpiry time.Duration `ini:"LFS_HTTP_AUTH_EXPIRY"`
	MaxFileSize    int64         `ini:"LFS_MAX_FILE_SIZE"`
	LocksPagingNum int           `ini:"LFS_LOCKS_PAGING_NUM"`

	Storage *Storage
}{}

func loadLFSFrom(rootCfg ConfigProvider) error {
	sec := rootCfg.Section("server")
	if err := sec.MapTo(&LFS); err != nil {
		return fmt.Errorf("failed to map LFS settings: %v", err)
	}

	lfsSec, _ := rootCfg.GetSection("lfs")

	// Specifically default PATH to LFS_CONTENT_PATH
	// DEPRECATED should not be removed because users maybe upgrade from lower version to the latest version
	// if these are removed, the warning will not be shown
	deprecatedSetting(rootCfg, "server", "LFS_CONTENT_PATH", "lfs", "PATH", "v1.19.0")

	if val := sec.Key("LFS_CONTENT_PATH").String(); val != "" {
		if lfsSec == nil {
			lfsSec = rootCfg.Section("lfs")
		}
		lfsSec.Key("PATH").MustString(val)
	}

	var err error
	LFS.Storage, err = getStorage(rootCfg, "lfs", "", lfsSec)
	if err != nil {
		return err
	}

	// Rest of LFS service settings
	if LFS.LocksPagingNum == 0 {
		LFS.LocksPagingNum = 50
	}

	LFS.HTTPAuthExpiry = sec.Key("LFS_HTTP_AUTH_EXPIRY").MustDuration(24 * time.Hour)

	return nil
}
