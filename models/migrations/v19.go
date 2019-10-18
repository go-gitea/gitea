// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"github.com/unknwon/com"
	"xorm.io/xorm"
)

func generateAndMigrateGitHooks(x *xorm.Engine) (err error) {
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
		hookTpls  = []string{
			fmt.Sprintf("#!/usr/bin/env %s\nORI_DIR=`pwd`\nSHELL_FOLDER=$(cd \"$(dirname \"$0\")\";pwd)\ncd \"$ORI_DIR\"\nfor i in `ls \"$SHELL_FOLDER/pre-receive.d\"`; do\n    sh \"$SHELL_FOLDER/pre-receive.d/$i\"\ndone", setting.ScriptType),
			fmt.Sprintf("#!/usr/bin/env %s\nORI_DIR=`pwd`\nSHELL_FOLDER=$(cd \"$(dirname \"$0\")\";pwd)\ncd \"$ORI_DIR\"\nfor i in `ls \"$SHELL_FOLDER/update.d\"`; do\n    sh \"$SHELL_FOLDER/update.d/$i\" $1 $2 $3\ndone", setting.ScriptType),
			fmt.Sprintf("#!/usr/bin/env %s\nORI_DIR=`pwd`\nSHELL_FOLDER=$(cd \"$(dirname \"$0\")\";pwd)\ncd \"$ORI_DIR\"\nfor i in `ls \"$SHELL_FOLDER/post-receive.d\"`; do\n    sh \"$SHELL_FOLDER/post-receive.d/$i\"\ndone", setting.ScriptType),
		}
		giteaHookTpls = []string{
			fmt.Sprintf("#!/usr/bin/env %s\n\"%s\" hook --config='%s' pre-receive\n", setting.ScriptType, setting.AppPath, setting.CustomConf),
			fmt.Sprintf("#!/usr/bin/env %s\n\"%s\" hook --config='%s' update $1 $2 $3\n", setting.ScriptType, setting.AppPath, setting.CustomConf),
			fmt.Sprintf("#!/usr/bin/env %s\n\"%s\" hook --config='%s' post-receive\n", setting.ScriptType, setting.AppPath, setting.CustomConf),
		}
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

			repoPath := filepath.Join(setting.RepoRootPath, strings.ToLower(user.Name), strings.ToLower(repo.Name)) + ".git"
			hookDir := filepath.Join(repoPath, "hooks")

			for i, hookName := range hookNames {
				oldHookPath := filepath.Join(hookDir, hookName)
				newHookPath := filepath.Join(hookDir, hookName+".d", "gitea")

				customHooksDir := filepath.Join(hookDir, hookName+".d")
				// if it's exist, that means you have upgraded ever
				if com.IsExist(customHooksDir) {
					continue
				}

				if err = os.MkdirAll(customHooksDir, os.ModePerm); err != nil {
					return fmt.Errorf("create hooks dir '%s': %v", customHooksDir, err)
				}

				// WARNING: Old server-side hooks will be moved to sub directory with the same name
				if hookName != "update" && com.IsExist(oldHookPath) {
					newPlace := filepath.Join(hookDir, hookName+".d", hookName)
					if err = os.Rename(oldHookPath, newPlace); err != nil {
						return fmt.Errorf("Remove old hook file '%s' to '%s': %v", oldHookPath, newPlace, err)
					}
				}

				if err = ioutil.WriteFile(oldHookPath, []byte(hookTpls[i]), 0777); err != nil {
					return fmt.Errorf("write old hook file '%s': %v", oldHookPath, err)
				}

				if err = ioutil.WriteFile(newHookPath, []byte(giteaHookTpls[i]), 0777); err != nil {
					return fmt.Errorf("write new hook file '%s': %v", oldHookPath, err)
				}
			}
			return nil
		})
}
