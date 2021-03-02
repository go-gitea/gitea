// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package stats

import (
	"io/ioutil"
	"os"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// DBIndexer implements Indexer interface to use database's like search
type DBIndexer struct {
}

// Index repository status function
func (db *DBIndexer) Index(id int64) error {
	repo, err := models.GetRepositoryByID(id)
	if err != nil {
		return err
	}
	if repo.IsEmpty {
		return nil
	}

	status, err := repo.GetIndexerStatus(models.RepoIndexerTypeStats)
	if err != nil {
		return err
	}

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return err
	}
	defer gitRepo.Close()

	// Get latest commit for default branch
	commitID, err := gitRepo.GetBranchCommitID(repo.DefaultBranch)
	if err != nil {
		log.Error("Unable to get commit ID for defaultbranch %s in %s", repo.DefaultBranch, repo.RepoPath())
		return err
	}

	// Do not recalculate stats if already calculated for this commit
	if status.CommitSha == commitID {
		return nil
	}

	var tmpIndex *os.File
	if git.CheckGitVersionAtLeast("1.7.8") == nil {
		tmpIndex, err = ioutil.TempFile("", "index")
		if err != nil {
			return err
		}
		defer func() {
			err := util.Remove(tmpIndex.Name())
			if err != nil {
				log.Error("failed to remove tmp index file: %v", err)
			}
		}()

		_, err = git.NewCommand("read-tree", commitID).
			RunInDirWithEnv(gitRepo.Path, []string{"GIT_INDEX_FILE=" + tmpIndex.Name()})
		if err != nil {
			return err
		}
	}

	// Calculate and save language statistics to database
	stats, err := gitRepo.GetLanguageStats(commitID, func(path string) (string, bool) {
		// get language follow linguist rulers
		// linguist-language=<lang> attribute to an language
		// linguist-vendored attribute to vendor or un-vendor paths

		if tmpIndex == nil {
			return "", false
		}

		name2attribute2info, err := gitRepo.CheckAttribute(git.CheckAttributeOpts{
			Attributes: []string{"linguist-vendored", "linguist-language"},
			Filenames:  []string{path},
			CachedOnly: true,
			IndexFile:  tmpIndex.Name(),
		})
		if err != nil {
			log.Error("gitRepo.CheckAttribute: %v", err)
			return "", false
		}

		attribute2info, has := name2attribute2info[path]
		if !has {
			return "", false
		}
		if attribute2info["linguist-vendored"] == "set" {
			return "", true
		}

		lang := attribute2info["linguist-language"]
		if lang == "unspecified" {
			lang = ""
		}

		return lang, false
	})
	if err != nil {
		log.Error("Unable to get language stats for ID %s for defaultbranch %s in %s. Error: %v", commitID, repo.DefaultBranch, repo.RepoPath(), err)
		return err
	}
	return repo.UpdateLanguageStats(commitID, stats)
}

// Close dummy function
func (db *DBIndexer) Close() {
}
