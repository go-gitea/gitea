// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitlog

import (
	"fmt"
	"os"
	"path"

	"code.gitea.io/log"
)

// GitLogger logger for git
var GitLogger *log.Logger

// NewGitLogger create a logger for git
// FIXME: use same log level as other loggers.
func NewGitLogger(logPath string) {
	path := path.Dir(logPath)

	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		log.Fatal("Failed to create dir %s: %v", path, err)
	}

	GitLogger = &log.Logger{
		MultiChannelledLog: log.NewMultiChannelledLog("git", 0),
	}
	GitLogger.SetLogger("file", "file", fmt.Sprintf(`{"level":"TRACE","filename":"%s","rotate":true,"maxsize":%d,"daily":true,"maxdays":7,"compress":true,"compressionLevel":-1, "stacktraceLevel":"NONE"}`, logPath, 1<<28))
}
