// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package options

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/util"
)

func walkAssetDir(root string, callback func(path, name string, d fs.DirEntry, err error) error) error {
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		// name is the path relative to the root
		name := path[len(root):]
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
		if err != nil {
			if os.IsNotExist(err) {
				return callback(path, name, d, err)
			}
			return err
		}
		if util.CommonSkip(d.Name()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		return callback(path, name, d, err)
	}); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to get files for assets in %s: %w", root, err)
	}
	return nil
}
