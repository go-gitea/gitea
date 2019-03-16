// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Unknwon/com"

	"code.gitea.io/git"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/sync"
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

// LocalWikiPath returns the local wiki repository copy path.
func LocalWikiPath() string {
	if filepath.IsAbs(setting.Repository.Local.LocalWikiPath) {
		return setting.Repository.Local.LocalWikiPath
	}
	return path.Join(setting.AppDataPath, setting.Repository.Local.LocalWikiPath)
}

// LocalWikiPath returns the path to the local wiki repository (?).
func (repo *Repository) LocalWikiPath() string {
	return path.Join(LocalWikiPath(), com.ToStr(repo.ID))
}

// UpdateLocalWiki makes sure the local copy of repository wiki is up-to-date.
func (repo *Repository) updateLocalWiki() error {
	// Don't pass branch name here because it fails to clone and
	// checkout to a specific branch when wiki is an empty repository.
	var branch = ""
	if com.IsExist(repo.LocalWikiPath()) {
		branch = "master"
	}
	return UpdateLocalCopyBranch(repo.WikiPath(), repo.LocalWikiPath(), branch)
}

func discardLocalWikiChanges(localPath string) error {
	return discardLocalRepoBranchChanges(localPath, "master")
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

	localPath := repo.LocalWikiPath()
	if err = discardLocalWikiChanges(localPath); err != nil {
		return fmt.Errorf("discardLocalWikiChanges: %v", err)
	} else if err = repo.updateLocalWiki(); err != nil {
		return fmt.Errorf("UpdateLocalWiki: %v", err)
	}

	newWikiPath := path.Join(localPath, WikiNameToFilename(newWikiName))

	// If not a new file, show perform update not create.
	if isNew {
		if com.IsExist(newWikiPath) {
			return ErrWikiAlreadyExist{newWikiPath}
		}
	} else {
		oldWikiPath := path.Join(localPath, WikiNameToFilename(oldWikiName))
		if err := os.Remove(oldWikiPath); err != nil {
			return fmt.Errorf("Failed to remove %s: %v", oldWikiPath, err)
		}
	}

	// SECURITY: if new file is a symlink to non-exist critical file,
	// attack content can be written to the target file (e.g. authorized_keys2)
	// as a new page operation.
	// So we want to make sure the symlink is removed before write anything.
	// The new file we created will be in normal text format.
	if err = os.RemoveAll(newWikiPath); err != nil {
		return err
	}

	if err = ioutil.WriteFile(newWikiPath, []byte(content), 0666); err != nil {
		return fmt.Errorf("WriteFile: %v", err)
	}

	if len(message) == 0 {
		message = "Update page '" + newWikiName + "'"
	}
	if err = git.AddChanges(localPath, true); err != nil {
		return fmt.Errorf("AddChanges: %v", err)
	} else if err = git.CommitChanges(localPath, git.CommitChangesOptions{
		Committer: doer.NewGitSig(),
		Message:   message,
	}); err != nil {
		return fmt.Errorf("CommitChanges: %v", err)
	} else if err = git.Push(localPath, git.PushOptions{
		Remote: "origin",
		Branch: "master",
	}); err != nil {
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

	localPath := repo.LocalWikiPath()
	if err = discardLocalWikiChanges(localPath); err != nil {
		return fmt.Errorf("discardLocalWikiChanges: %v", err)
	} else if err = repo.updateLocalWiki(); err != nil {
		return fmt.Errorf("UpdateLocalWiki: %v", err)
	}

	filename := path.Join(localPath, WikiNameToFilename(wikiName))

	if err := os.Remove(filename); err != nil {
		return fmt.Errorf("Failed to remove %s: %v", filename, err)
	}

	message := "Delete page '" + wikiName + "'"

	if err = git.AddChanges(localPath, true); err != nil {
		return fmt.Errorf("AddChanges: %v", err)
	} else if err = git.CommitChanges(localPath, git.CommitChangesOptions{
		Committer: doer.NewGitSig(),
		Message:   message,
	}); err != nil {
		return fmt.Errorf("CommitChanges: %v", err)
	} else if err = git.Push(localPath, git.PushOptions{
		Remote: "origin",
		Branch: "master",
	}); err != nil {
		return fmt.Errorf("Push: %v", err)
	}

	return nil
}
