// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Global settings
var (
	// RunUser is the OS user that Gitea is running as. ini:"RUN_USER"
	RunUser string
	// RunMode is the running mode of Gitea, it only accepts two values: "dev" and "prod".
	// Non-dev values will be replaced by "prod". ini: "RUN_MODE"
	RunMode string
	// IsProd is true if RunMode is not "dev"
	IsProd bool

	// AppName is the Application name, used in the page title. ini: "APP_NAME"
	AppName string

	createTempOnce sync.Once
)

// TempDir returns the OS temp directory
func TempDir() string {
	tempDir := filepath.Join(os.TempDir(), "gitea")
	createTempOnce.Do(func() {
		if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
			log.Fatalf("Failed to create temp directory %s: %v", tempDir, err)
		}
	})
	return tempDir
}

func CleanUpTempDirs() {
	_ = os.RemoveAll(TempDir())
}
