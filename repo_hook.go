// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"io/ioutil"
	"os"
	"path"
)

const (
	HOOK_PATH_UPDATE = "hooks/update"
)

// SetUpdateHook writes given content to update hook of the reposiotry.
func SetUpdateHook(repoPath, content string) error {
	log("Setting update hook: %s", repoPath)
	hookPath := path.Join(repoPath, HOOK_PATH_UPDATE)
	os.MkdirAll(path.Dir(hookPath), os.ModePerm)
	return ioutil.WriteFile(hookPath, []byte(content), 0777)
}
