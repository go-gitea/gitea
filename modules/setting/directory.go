// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"fmt"
	"os"
)

// PrepareAppDataPath creates app data directory if necessary
func PrepareAppDataPath() error {
	// FIXME: There are too many calls to MkdirAll in old code. It is incorrect.
	// For example, if someDir=/mnt/vol1/gitea-home/data, if the mount point /mnt/vol1 is not mounted when Gitea runs,
	// then gitea will make new empty directories in /mnt/vol1, all are stored in the root filesystem.
	// The correct behavior should be: creating parent directories is end users' duty. We only create sub-directories in existing parent directories.
	// For quickstart, the parent directories should be created automatically for first startup (eg: a flag or a check of INSTALL_LOCK).
	// Now we can take the first step to do correctly (using Mkdir) in other packages, and prepare the AppDataPath here, then make a refactor in future.

	st, err := os.Stat(AppDataPath)

	if os.IsNotExist(err) {
		err = os.MkdirAll(AppDataPath, os.ModePerm)
		if err != nil {
			return fmt.Errorf("unable to create the APP_DATA_PATH directory: %q, Error: %v", AppDataPath, err)
		}
		return nil
	}

	if err != nil {
		return fmt.Errorf("unable to use APP_DATA_PATH %q. Error: %v", AppDataPath, err)
	}

	if !st.IsDir() /* also works for symlink */ {
		return fmt.Errorf("the APP_DATA_PATH %q is not a directory (or symlink to a directory) and can't be used", AppDataPath)
	}

	return nil
}
