// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"compress/gzip"
	"os"
	"path"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

var Audit = struct {
	Enabled     bool
	FileOptions *log.WriterFileOption `ini:"-"`
}{
	Enabled: false,
}

func loadAuditFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "audit", &Audit)

	sec, err := rootCfg.GetSection("audit.file")
	if err == nil {
		if !ConfigSectionKeyBool(sec, "ENABLED") {
			return
		}

		opts := &log.WriterFileOption{
			FileName:         path.Join(Log.RootPath, "audit.log"),
			LogRotate:        true,
			DailyRotate:      true,
			MaxDays:          7,
			Compress:         true,
			CompressionLevel: gzip.DefaultCompression,
		}

		if err := sec.MapTo(opts); err != nil {
			log.Fatal("Failed to map audit file settings: %v", err)
		}

		opts.FileName = util.FilePathJoinAbs(opts.FileName)
		if !filepath.IsAbs(opts.FileName) {
			opts.FileName = path.Join(Log.RootPath, opts.FileName)
		}

		if err := os.MkdirAll(filepath.Dir(opts.FileName), os.ModePerm); err != nil {
			log.Fatal("Unable to create directory for audit log %s: %v", opts.FileName, err)
		}

		opts.MaxSize = mustBytes(sec, "MAXIMUM_SIZE")
		if opts.MaxSize <= 0 {
			opts.MaxSize = 1 << 28
		}

		Audit.FileOptions = opts
	}
}
