// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package options

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func walkAssetDir(root string, callback func(path, name string, d fs.DirEntry, err error) error) error {
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
		if d.Name() == ".DS_Store" && d.IsDir() { // Because Macs...
			return fs.SkipDir
		}
		return callback(path, name, d, err)
	}); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to get files for assets in %s: %w", root, err)
	}
	return nil
}
