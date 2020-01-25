// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
)

// CreateDelegateHooks creates all the hooks scripts for the repo
func CreateDelegateHooks(repoPath string) error {
	return createDelegateHooks(repoPath)
}

// createDelegateHooks creates all the hooks scripts for the repo
func createDelegateHooks(repoPath string) (err error) {

	var (
		hookNames = []string{"pre-receive", "update", "post-receive"}
		hookTpls  = []string{
			fmt.Sprintf("#!/usr/bin/env %s\ndata=$(cat)\nexitcodes=\"\"\nhookname=$(basename $0)\nGIT_DIR=${GIT_DIR:-$(dirname $0)}\n\nfor hook in ${GIT_DIR}/hooks/${hookname}.d/*; do\ntest -x \"${hook}\" || continue\necho \"${data}\" | \"${hook}\"\nexitcodes=\"${exitcodes} $?\"\ndone\n\nfor i in ${exitcodes}; do\n[ ${i} -eq 0 ] || exit ${i}\ndone\n", setting.ScriptType),
			fmt.Sprintf("#!/usr/bin/env %s\nexitcodes=\"\"\nhookname=$(basename $0)\nGIT_DIR=${GIT_DIR:-$(dirname $0)}\n\nfor hook in ${GIT_DIR}/hooks/${hookname}.d/*; do\ntest -x \"${hook}\" || continue\n\"${hook}\" $1 $2 $3\nexitcodes=\"${exitcodes} $?\"\ndone\n\nfor i in ${exitcodes}; do\n[ ${i} -eq 0 ] || exit ${i}\ndone\n", setting.ScriptType),
			fmt.Sprintf("#!/usr/bin/env %s\ndata=$(cat)\nexitcodes=\"\"\nhookname=$(basename $0)\nGIT_DIR=${GIT_DIR:-$(dirname $0)}\n\nfor hook in ${GIT_DIR}/hooks/${hookname}.d/*; do\ntest -x \"${hook}\" || continue\necho \"${data}\" | \"${hook}\"\nexitcodes=\"${exitcodes} $?\"\ndone\n\nfor i in ${exitcodes}; do\n[ ${i} -eq 0 ] || exit ${i}\ndone\n", setting.ScriptType),
		}
		giteaHookTpls = []string{
			fmt.Sprintf("#!/usr/bin/env %s\n\"%s\" hook --config='%s' pre-receive\n", setting.ScriptType, setting.AppPath, setting.CustomConf),
			fmt.Sprintf("#!/usr/bin/env %s\n\"%s\" hook --config='%s' update $1 $2 $3\n", setting.ScriptType, setting.AppPath, setting.CustomConf),
			fmt.Sprintf("#!/usr/bin/env %s\n\"%s\" hook --config='%s' post-receive\n", setting.ScriptType, setting.AppPath, setting.CustomConf),
		}
	)

	hookDir := filepath.Join(repoPath, "hooks")

	for i, hookName := range hookNames {
		oldHookPath := filepath.Join(hookDir, hookName)
		newHookPath := filepath.Join(hookDir, hookName+".d", "gitea")

		if err := os.MkdirAll(filepath.Join(hookDir, hookName+".d"), os.ModePerm); err != nil {
			return fmt.Errorf("create hooks dir '%s': %v", filepath.Join(hookDir, hookName+".d"), err)
		}

		// WARNING: This will override all old server-side hooks
		if err = os.Remove(oldHookPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("unable to pre-remove old hook file '%s' prior to rewriting: %v ", oldHookPath, err)
		}
		if err = ioutil.WriteFile(oldHookPath, []byte(hookTpls[i]), 0777); err != nil {
			return fmt.Errorf("write old hook file '%s': %v", oldHookPath, err)
		}

		if err = os.Remove(newHookPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("unable to pre-remove new hook file '%s' prior to rewriting: %v", newHookPath, err)
		}
		if err = ioutil.WriteFile(newHookPath, []byte(giteaHookTpls[i]), 0777); err != nil {
			return fmt.Errorf("write new hook file '%s': %v", newHookPath, err)
		}
	}

	return nil
}

// SyncRepositoryHooks rewrites all repositories' pre-receive, update and post-receive hooks
// to make sure the binary and custom conf path are up-to-date.
func SyncRepositoryHooks(ctx context.Context) error {
	log.Trace("Doing: SyncRepositoryHooks")

	if err := models.Iterate(
		models.DefaultDBContext(),
		new(models.Repository),
		builder.Gt{"id": 0},
		func(idx int, bean interface{}) error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("Aborted due to shutdown")
			default:
			}

			if err := createDelegateHooks(bean.(*models.Repository).RepoPath()); err != nil {
				return fmt.Errorf("SyncRepositoryHook: %v", err)
			}
			if bean.(*models.Repository).HasWiki() {
				if err := createDelegateHooks(bean.(*models.Repository).WikiPath()); err != nil {
					return fmt.Errorf("SyncRepositoryHook: %v", err)
				}
			}
			return nil
		},
	); err != nil {
		return err
	}

	log.Trace("Finished: SyncRepositoryHooks")
	return nil
}
