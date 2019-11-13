// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/sync"

	"github.com/Unknwon/com"
)

var (
	reservedWikiNames = []string{"_pages", "_new", "_edit", "raw"}
	wikiWorkingPool   = sync.NewExclusivePool()
)

// NormalizeWikiName normalizes a wiki name
func NormalizeWikiName(name string) string {
	return strings.Replace(name, "-", " ", -1)
}

// WikiNameToSubURL converts a wiki name to its corresponding sub-URL.
func WikiNameToSubURL(name string) string {
	return url.QueryEscape(strings.Replace(name, " ", "-", -1))
}

// WikiNameToFilename converts a wiki name to its corresponding filename.
func WikiNameToFilename(name string) string {
	name = strings.Replace(name, " ", "-", -1)
	return url.QueryEscape(name) + ".md"
}

// WikiFilenameToName converts a wiki filename to its corresponding page name.
func WikiFilenameToName(filename string) (string, error) {
	if !strings.HasSuffix(filename, ".md") {
		return "", ErrWikiInvalidFileName{filename}
	}
	basename := filename[:len(filename)-3]
	unescaped, err := url.QueryUnescape(basename)
	if err != nil {
		return "", err
	}
	return NormalizeWikiName(unescaped), nil
}

// WikiCloneLink returns clone URLs of repository wiki.
func (repo *Repository) WikiCloneLink() *CloneLink {
	return repo.cloneLink(x, true)
}

// WikiPath returns wiki data path by given user and repository name.
func WikiPath(userName, repoName string) string {
	return filepath.Join(UserPath(userName), strings.ToLower(repoName)+".wiki.git")
}

// WikiPath returns wiki data path for given repository.
func (repo *Repository) WikiPath() string {
	return WikiPath(repo.MustOwnerName(), repo.Name)
}

// HasWiki returns true if repository has wiki.
func (repo *Repository) HasWiki() bool {
	return com.IsDir(repo.WikiPath())
}

// InitWiki initializes a wiki for repository,
// it does nothing when repository already has wiki.
func (repo *Repository) InitWiki() error {
	if repo.HasWiki() {
		return nil
	}

	if err := git.InitRepository(repo.WikiPath(), true); err != nil {
		return fmt.Errorf("InitRepository: %v", err)
	} else if err = createDelegateHooks(repo.WikiPath()); err != nil {
		return fmt.Errorf("createDelegateHooks: %v", err)
	}
	return nil
}

// nameAllowed checks if a wiki name is allowed
func nameAllowed(name string) error {
	for _, reservedName := range reservedWikiNames {
		if name == reservedName {
			return ErrWikiReservedName{name}
		}
	}
	return nil
}

// updateWikiPage adds a new page to the repository wiki.
func (repo *Repository) updateWikiPage(doer *User, oldWikiName, newWikiName, content, message string, isNew bool) (err error) {
	if err = nameAllowed(newWikiName); err != nil {
		return err
	}
	wikiWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer wikiWorkingPool.CheckOut(com.ToStr(repo.ID))

	if err = repo.InitWiki(); err != nil {
		return fmt.Errorf("InitWiki: %v", err)
	}

	hasMasterBranch := git.IsBranchExist(repo.WikiPath(), "master")

	basePath, err := CreateTemporaryPath("update-wiki")
	if err != nil {
		return err
	}
	defer func() {
		if err := RemoveTemporaryPath(basePath); err != nil {
			log.Error("Merge: RemoveTemporaryPath: %s", err)
		}
	}()

	cloneOpts := git.CloneRepoOptions{
		Bare:   true,
		Shared: true,
	}

	if hasMasterBranch {
		cloneOpts.Branch = "master"
	}

	if err := git.Clone(repo.WikiPath(), basePath, cloneOpts); err != nil {
		log.Error("Failed to clone repository: %s (%v)", repo.FullName(), err)
		return fmt.Errorf("Failed to clone repository: %s (%v)", repo.FullName(), err)
	}

	gitRepo, err := git.OpenRepository(basePath)
	if err != nil {
		log.Error("Unable to open temporary repository: %s (%v)", basePath, err)
		return fmt.Errorf("Failed to open new temporary repository in: %s %v", basePath, err)
	}
	defer gitRepo.Close()

	if hasMasterBranch {
		if err := gitRepo.ReadTreeToIndex("HEAD"); err != nil {
			log.Error("Unable to read HEAD tree to index in: %s %v", basePath, err)
			return fmt.Errorf("Unable to read HEAD tree to index in: %s %v", basePath, err)
		}
	}

	newWikiPath := WikiNameToFilename(newWikiName)
	if isNew {
		filesInIndex, err := gitRepo.LsFiles(newWikiPath)
		if err != nil {
			log.Error("%v", err)
			return err
		}
		for _, file := range filesInIndex {
			if file == newWikiPath {
				return ErrWikiAlreadyExist{newWikiPath}
			}
		}
	} else {
		oldWikiPath := WikiNameToFilename(oldWikiName)
		filesInIndex, err := gitRepo.LsFiles(oldWikiPath)
		if err != nil {
			log.Error("%v", err)
			return err
		}
		found := false
		for _, file := range filesInIndex {
			if file == oldWikiPath {
				found = true
				break
			}
		}
		if found {
			err := gitRepo.RemoveFilesFromIndex(oldWikiPath)
			if err != nil {
				log.Error("%v", err)
				return err
			}
		}
	}

	// FIXME: The wiki doesn't have lfs support at present - if this changes need to check attributes here

	objectHash, err := gitRepo.HashObject(strings.NewReader(content))
	if err != nil {
		log.Error("%v", err)
		return err
	}

	if err := gitRepo.AddObjectToIndex("100644", objectHash, newWikiPath); err != nil {
		log.Error("%v", err)
		return err
	}

	tree, err := gitRepo.WriteTree()
	if err != nil {
		log.Error("%v", err)
		return err
	}

	commitTreeOpts := git.CommitTreeOpts{
		Message: message,
	}
	if hasMasterBranch {
		commitTreeOpts.Parents = []string{"HEAD"}
	}
	commitHash, err := gitRepo.CommitTree(doer.NewGitSig(), tree, commitTreeOpts)
	if err != nil {
		log.Error("%v", err)
		return err
	}

	if err := git.Push(basePath, git.PushOptions{
		Remote: "origin",
		Branch: fmt.Sprintf("%s:%s%s", commitHash.String(), git.BranchPrefix, "master"),
		Env: FullPushingEnvironment(
			doer,
			doer,
			repo,
			repo.Name+".wiki",
			0,
		),
	}); err != nil {
		log.Error("%v", err)
		return fmt.Errorf("Push: %v", err)
	}

	return nil
}

// AddWikiPage adds a new wiki page with a given wikiPath.
func (repo *Repository) AddWikiPage(doer *User, wikiName, content, message string) error {
	return repo.updateWikiPage(doer, "", wikiName, content, message, true)
}

// EditWikiPage updates a wiki page identified by its wikiPath,
// optionally also changing wikiPath.
func (repo *Repository) EditWikiPage(doer *User, oldWikiName, newWikiName, content, message string) error {
	return repo.updateWikiPage(doer, oldWikiName, newWikiName, content, message, false)
}

// DeleteWikiPage deletes a wiki page identified by its path.
func (repo *Repository) DeleteWikiPage(doer *User, wikiName string) (err error) {
	wikiWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer wikiWorkingPool.CheckOut(com.ToStr(repo.ID))

	if err = repo.InitWiki(); err != nil {
		return fmt.Errorf("InitWiki: %v", err)
	}

	basePath, err := CreateTemporaryPath("update-wiki")
	if err != nil {
		return err
	}
	defer func() {
		if err := RemoveTemporaryPath(basePath); err != nil {
			log.Error("Merge: RemoveTemporaryPath: %s", err)
		}
	}()

	if err := git.Clone(repo.WikiPath(), basePath, git.CloneRepoOptions{
		Bare:   true,
		Shared: true,
		Branch: "master",
	}); err != nil {
		log.Error("Failed to clone repository: %s (%v)", repo.FullName(), err)
		return fmt.Errorf("Failed to clone repository: %s (%v)", repo.FullName(), err)
	}

	gitRepo, err := git.OpenRepository(basePath)
	if err != nil {
		log.Error("Unable to open temporary repository: %s (%v)", basePath, err)
		return fmt.Errorf("Failed to open new temporary repository in: %s %v", basePath, err)
	}
	defer gitRepo.Close()

	if err := gitRepo.ReadTreeToIndex("HEAD"); err != nil {
		log.Error("Unable to read HEAD tree to index in: %s %v", basePath, err)
		return fmt.Errorf("Unable to read HEAD tree to index in: %s %v", basePath, err)
	}

	wikiPath := WikiNameToFilename(wikiName)
	filesInIndex, err := gitRepo.LsFiles(wikiPath)
	found := false
	for _, file := range filesInIndex {
		if file == wikiPath {
			found = true
			break
		}
	}
	if found {
		err := gitRepo.RemoveFilesFromIndex(wikiPath)
		if err != nil {
			return err
		}
	} else {
		return os.ErrNotExist
	}

	// FIXME: The wiki doesn't have lfs support at present - if this changes need to check attributes here

	tree, err := gitRepo.WriteTree()
	if err != nil {
		return err
	}
	message := "Delete page '" + wikiName + "'"

	commitHash, err := gitRepo.CommitTree(doer.NewGitSig(), tree, git.CommitTreeOpts{
		Message: message,
		Parents: []string{"HEAD"},
	})
	if err != nil {
		return err
	}

	if err := git.Push(basePath, git.PushOptions{
		Remote: "origin",
		Branch: fmt.Sprintf("%s:%s%s", commitHash.String(), git.BranchPrefix, "master"),
		Env:    PushingEnvironment(doer, repo),
	}); err != nil {
		return fmt.Errorf("Push: %v", err)
	}

	return nil
}
