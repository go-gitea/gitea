// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"io"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util/rotatingfilewriter"
)

var rfw *rotatingfilewriter.RotatingFileWriter

func initAuditFile() error {
	if setting.Audit.FileOptions == nil {
		return nil
	}

	opts := setting.Audit.FileOptions

	var err error
	rfw, err = rotatingfilewriter.Open(opts.FileName, &rotatingfilewriter.Options{
		Rotate:           opts.LogRotate,
		MaximumSize:      opts.MaxSize,
		RotateDaily:      opts.DailyRotate,
		KeepDays:         opts.MaxDays,
		Compress:         opts.Compress,
		CompressionLevel: opts.CompressionLevel,
	})
	return err
}

func writeToFile(e *Event) error {
	if rfw == nil {
		return nil
	}
	return WriteEventAsJSON(rfw, e)
}

func WriteEventAsJSON(w io.Writer, e *Event) error {
	return json.NewEncoder(w).Encode(e)
}
