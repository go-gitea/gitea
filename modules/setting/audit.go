// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"compress/gzip"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

var Audit = struct {
	Enabled         bool
	Appender        string
	AppenderOptions map[string]*AppenderOptions
}{
	Enabled:         false,
	AppenderOptions: make(map[string]*AppenderOptions),
}

type AppenderOptions struct {
	Filename         string
	Rotate           bool
	MaximumSize      int64
	RotateDaily      bool
	KeepDays         int
	Compress         bool
	CompressionLevel int
}

func loadAuditFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "audit", &Audit)

	for _, name := range strings.Split(Audit.Appender, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		sec, err := rootCfg.GetSection("audit." + name)
		if err != nil {
			sec, _ = rootCfg.NewSection("audit." + name)
		}

		opts := &AppenderOptions{
			Filename:         path.Join(Log.RootPath, "audit.log"),
			Rotate:           true,
			RotateDaily:      true,
			KeepDays:         7,
			CompressionLevel: gzip.DefaultCompression,
		}

		if err := sec.MapTo(opts); err != nil {
			log.Error("audit.%s: %v", name, err.Error())
		}

		opts.Filename = util.FilePathJoinAbs(opts.Filename)
		if !filepath.IsAbs(opts.Filename) {
			opts.Filename = path.Join(Log.RootPath, opts.Filename)
		}

		opts.MaximumSize = mustBytes(sec, "MAXIMUM_SIZE")
		if opts.MaximumSize <= 0 {
			opts.MaximumSize = 1 << 28
		}

		Audit.AppenderOptions[name] = opts
	}
}
