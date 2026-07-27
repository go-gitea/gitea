// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"compress/gzip"

	"gitea.dev/modules/log"
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
			LogRotate:        true,
			DailyRotate:      true,
			MaxDays:          7,
			Compress:         true,
			CompressionLevel: gzip.DefaultCompression,
		}

		if err := sec.MapTo(opts); err != nil {
			log.Fatal("Failed to map audit file settings: %v", err)
		}

		// a relative FILE_NAME is resolved against the log root path
		opts.FileName = LogPrepareFilenameForWriter(opts.FileName, "audit.log")

		opts.MaxSize = mustBytes(sec, "MAXIMUM_SIZE")
		if opts.MaxSize <= 0 {
			opts.MaxSize = 1 << 28
		}

		Audit.FileOptions = opts
	}
}
