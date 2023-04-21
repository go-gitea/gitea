// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"compress/gzip"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
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
			Filename:         filepath.Join(Log.RootPath, "audit.log"),
			Rotate:           false,
			MaximumSize:      1 << 28,
			RotateDaily:      true,
			KeepDays:         7,
			CompressionLevel: gzip.DefaultCompression,
		}

		if err := sec.MapTo(opts); err != nil {
			log.Error("audit.%s: %v", name, err.Error())
		}

		Audit.AppenderOptions[name] = opts
	}
}
