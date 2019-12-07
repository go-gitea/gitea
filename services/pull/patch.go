// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"

	"github.com/unknwon/com"
)

// PatchPath returns corresponding patch file path of repository by given issue index.
func PatchPath(repo *models.Repository, index int64) (string, error) {
	return repo.PatchPath(models.DefaultDBContext(), index)
}

// SavePatch saves patch data to corresponding location by given issue ID.
func SavePatch(repo *models.Repository, index int64, patch []byte) error {
	return savePatch(models.DefaultDBContext(), repo, index, patch)
}

// savePatch saves patch data to corresponding location by given issue ID.
func savePatch(ctx models.DBContext, repo *models.Repository, index int64, patch []byte) error {
	patchPath, err := repo.PatchPath(ctx, index)
	if err != nil {
		return fmt.Errorf("PatchPath: %v", err)
	}
	dir := filepath.Dir(patchPath)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", dir, err)
	}

	if err = ioutil.WriteFile(patchPath, patch, 0644); err != nil {
		return fmt.Errorf("WriteFile: %v", err)
	}

	return nil
}

// UpdatePatch generates and saves a new patch.
func UpdatePatch(pr *models.PullRequest) (err error) {
	if err = pr.GetHeadRepo(); err != nil {
		return fmt.Errorf("GetHeadRepo: %v", err)
	} else if pr.HeadRepo == nil {
		log.Trace("PullRequest[%d].UpdatePatch: ignored corrupted data", pr.ID)
		return nil
	}

	if err = pr.GetBaseRepo(); err != nil {
		return fmt.Errorf("GetBaseRepo: %v", err)
	}

	headGitRepo, err := git.OpenRepository(pr.HeadRepo.RepoPath())
	if err != nil {
		return fmt.Errorf("OpenRepository: %v", err)
	}
	defer headGitRepo.Close()

	// Add a temporary remote.
	tmpRemote := com.ToStr(time.Now().UnixNano())
	if err = headGitRepo.AddRemote(tmpRemote, models.RepoPath(pr.BaseRepo.MustOwner().Name, pr.BaseRepo.Name), true); err != nil {
		return fmt.Errorf("AddRemote: %v", err)
	}
	defer func() {
		if err := headGitRepo.RemoveRemote(tmpRemote); err != nil {
			log.Error("UpdatePatch: RemoveRemote: %s", err)
		}
	}()
	pr.MergeBase, _, err = headGitRepo.GetMergeBase(tmpRemote, pr.BaseBranch, pr.HeadBranch)
	if err != nil {
		return fmt.Errorf("GetMergeBase: %v", err)
	} else if err = pr.Update(); err != nil {
		return fmt.Errorf("Update: %v", err)
	}

	patch, err := headGitRepo.GetPatch(pr.MergeBase, pr.HeadBranch)
	if err != nil {
		return fmt.Errorf("GetPatch: %v", err)
	}

	if err = SavePatch(pr.BaseRepo, pr.Index, patch); err != nil {
		return fmt.Errorf("BaseRepo.SavePatch: %v", err)
	}

	return nil
}
