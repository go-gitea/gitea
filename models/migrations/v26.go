// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"github.com/unknwon/com"
	"xorm.io/xorm"
)

func generateAndMigrateGitHookChains(x *xorm.Engine) (err error) {
	type Repository struct {
		ID      int64
		OwnerID int64
		Name    string
	}
	type User struct {
		ID   int64
		Name string
	}

	var (
		hookNames = []string{"pre-receive", "update", "post-receive"}
		hookTpl   = fmt.Sprintf("#!/usr/bin/env %s\ndata=$(cat)\nexitcodes=\"\"\nhookname=$(basename $0)\nGIT_DIR=${GIT_DIR:-$(dirname $0)}\n\nfor hook in ${GIT_DIR}/hooks/${hookname}.d/*; do\ntest -x \"${hook}\" || continue\necho \"${data}\" | \"${hook}\"\nexitcodes=\"${exitcodes} $?\"\ndone\n\nfor i in ${exitcodes}; do\n[ ${i} -eq 0 ] || exit ${i}\ndone\n", setting.ScriptType)
	)

	return x.Where("id > 0").BufferSize(setting.Database.IterateBufferSize).Iterate(new(Repository),
		func(idx int, bean interface{}) error {
			repo := bean.(*Repository)
			user := new(User)
			has, err := x.Where("id = ?", repo.OwnerID).Get(user)
			if err != nil {
				return fmt.Errorf("query owner of repository [repo_id: %d, owner_id: %d]: %v", repo.ID, repo.OwnerID, err)
			} else if !has {
				return nil
			}

			repoPaths := []string{
				filepath.Join(setting.RepoRootPath, strings.ToLower(user.Name), strings.ToLower(repo.Name)) + ".git",
				filepath.Join(setting.RepoRootPath, strings.ToLower(user.Name), strings.ToLower(repo.Name)) + ".wiki.git",
			}

			for _, repoPath := range repoPaths {
				if com.IsExist(repoPath) {
					hookDir := filepath.Join(repoPath, "hooks")

					for _, hookName := range hookNames {
						oldHookPath := filepath.Join(hookDir, hookName)

						// compare md5sums of hooks
						if com.IsExist(oldHookPath) {

							f, err := os.Open(oldHookPath)
							if err != nil {
								return fmt.Errorf("cannot open old hook file '%s': %v", oldHookPath, err)
							}
							defer f.Close()
							h := md5.New()
							if _, err := io.Copy(h, f); err != nil {
								return fmt.Errorf("cannot read old hook file '%s': %v", oldHookPath, err)
							}
							if hex.EncodeToString(h.Sum(nil)) == "6718ef67d0834e0a7908259acd566e3f" {
								return nil
							}
						}

						if err = ioutil.WriteFile(oldHookPath, []byte(hookTpl), 0777); err != nil {
							return fmt.Errorf("write old hook file '%s': %v", oldHookPath, err)
						}
					}
				}
			}
			return nil
		})
}
