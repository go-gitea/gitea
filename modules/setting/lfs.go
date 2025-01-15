// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/generate"
)

// LFS represents the server-side configuration for Git LFS.
// Ideally these options should be in a section like "[lfs_server]",
// but they are in "[server]" section due to historical reasons.
// Could be refactored in the future while keeping backwards compatibility.
var LFS = struct {
	StartServer    bool          `ini:"LFS_START_SERVER"`
	AllowPureSSH   bool          `ini:"LFS_ALLOW_PURE_SSH"`
	JWTSecretBytes []byte        `ini:"-"`
	HTTPAuthExpiry time.Duration `ini:"LFS_HTTP_AUTH_EXPIRY"`
	MaxFileSize    int64         `ini:"LFS_MAX_FILE_SIZE"`
	LocksPagingNum int           `ini:"LFS_LOCKS_PAGING_NUM"`
	MaxBatchSize   int           `ini:"LFS_MAX_BATCH_SIZE"`

	Storage *Storage
}{}

// LFSClient represents configuration for Gitea's LFS clients, for example: mirroring upstream Git LFS
var LFSClient = struct {
	BatchSize                 int `ini:"BATCH_SIZE"`
	BatchOperationConcurrency int `ini:"BATCH_OPERATION_CONCURRENCY"`
}{}

func loadLFSFrom(rootCfg ConfigProvider) error {
	mustMapSetting(rootCfg, "lfs_client", &LFSClient)

	mustMapSetting(rootCfg, "server", &LFS)
	sec := rootCfg.Section("server")

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

	if LFSClient.BatchSize < 1 {
		LFSClient.BatchSize = 20
	}

	if LFSClient.BatchOperationConcurrency < 1 {
		// match the default git-lfs's `lfs.concurrenttransfers` https://github.com/git-lfs/git-lfs/blob/main/docs/man/git-lfs-config.adoc#upload-and-download-transfer-settings
		LFSClient.BatchOperationConcurrency = 8
	}

	LFS.HTTPAuthExpiry = sec.Key("LFS_HTTP_AUTH_EXPIRY").MustDuration(24 * time.Hour)

	if !LFS.StartServer || !InstallLock {
		return nil
	}

	jwtSecretBase64 := loadSecret(rootCfg.Section("server"), "LFS_JWT_SECRET_URI", "LFS_JWT_SECRET")
	LFS.JWTSecretBytes, err = generate.DecodeJwtSecretBase64(jwtSecretBase64)
	if err != nil {
		LFS.JWTSecretBytes, jwtSecretBase64, err = generate.NewJwtSecretWithBase64()
		if err != nil {
			return fmt.Errorf("error generating JWT Secret for custom config: %v", err)
		}

		// Save secret
		saveCfg, err := rootCfg.PrepareSaving()
		if err != nil {
			return fmt.Errorf("error saving JWT Secret for custom config: %v", err)
		}
		rootCfg.Section("server").Key("LFS_JWT_SECRET").SetValue(jwtSecretBase64)
		saveCfg.Section("server").Key("LFS_JWT_SECRET").SetValue(jwtSecretBase64)
		if err := saveCfg.Save(); err != nil {
			return fmt.Errorf("error saving JWT Secret for custom config: %v", err)
		}
	}

	return nil
}
