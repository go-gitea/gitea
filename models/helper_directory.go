// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/unknwon/com"
)

// LocalCopyPath returns the local repository temporary copy path.
func LocalCopyPath() string {
	if filepath.IsAbs(setting.Repository.Local.LocalCopyPath) {
		return setting.Repository.Local.LocalCopyPath
	}
	return path.Join(setting.AppDataPath, setting.Repository.Local.LocalCopyPath)
}

// CreateTemporaryPath creates a temporary path
func CreateTemporaryPath(prefix string) (string, error) {
	timeStr := com.ToStr(time.Now().Nanosecond()) // SHOULD USE SOMETHING UNIQUE
	basePath := path.Join(LocalCopyPath(), prefix+"-"+timeStr+".git")
	if err := os.MkdirAll(filepath.Dir(basePath), os.ModePerm); err != nil {
		log.Error("Unable to create temporary directory: %s (%v)", basePath, err)
		return "", fmt.Errorf("Failed to create dir %s: %v", basePath, err)
	}
	return basePath, nil
}

// RemoveTemporaryPath removes the temporary path
func RemoveTemporaryPath(basePath string) error {
	if _, err := os.Stat(basePath); !os.IsNotExist(err) {
		return os.RemoveAll(basePath)
	}
	return nil
}
