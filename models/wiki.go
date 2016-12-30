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

var wikiWorkingPool = sync.NewExclusivePool()

// ToWikiPageURL formats a string to corresponding wiki URL name.
func ToWikiPageURL(name string) string {
	return url.QueryEscape(strings.Replace(name, " ", "-", -1))
}

// ToWikiPageName formats a URL back to corresponding wiki page name,
// and removes leading characters './' to prevent changing files
// that are not belong to wiki repository.
func ToWikiPageName(urlString string) string {
	name, _ := url.QueryUnescape(strings.Replace(urlString, "-", " ", -1))
	name = strings.Replace(name, "\t", " ", -1)
	return strings.Replace(strings.TrimLeft(name, "./"), "/", " ", -1)
}

// WikiCloneLink returns clone URLs of repository wiki.
func (repo *Repository) WikiCloneLink() (cl *CloneLink) {
	return repo.cloneLink(true)
}

// WikiPath returns wiki data path by given user and repository name.
func WikiPath(userName, repoName string) string {
	return filepath.Join(UserPath(userName), strings.ToLower(repoName)+".wiki.git")
}

// WikiPath returns wiki data path for given repository.
func (repo *Repository) WikiPath() string {
	return WikiPath(repo.MustOwner().Name, repo.Name)
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
	} else if err = createUpdateHook(repo.WikiPath()); err != nil {
		return fmt.Errorf("createUpdateHook: %v", err)
	}
	return nil
}

// LocalWikiPath returns the path to the local wiki repository (?).
func (repo *Repository) LocalWikiPath() string {
	return path.Join(setting.AppDataPath, "tmp/local-wiki", com.ToStr(repo.ID))
}

// UpdateLocalWiki makes sure the local copy of repository wiki is up-to-date.
func (repo *Repository) UpdateLocalWiki() error {
	// Don't pass branch name here because it fails to clone and
	// checkout to a specific branch when wiki is an empty repository.
	return UpdateLocalCopyBranch(repo.WikiPath(), repo.LocalWikiPath(), "")
}

func discardLocalWikiChanges(localPath string) error {
	return discardLocalRepoBranchChanges(localPath, "master")
}

// updateWikiPage adds new page to repository wiki.
func (repo *Repository) updateWikiPage(doer *User, oldTitle, title, content, message string, isNew bool) (err error) {
	wikiWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer wikiWorkingPool.CheckOut(com.ToStr(repo.ID))

	if err = repo.InitWiki(); err != nil {
		return fmt.Errorf("InitWiki: %v", err)
	}

	localPath := repo.LocalWikiPath()
	if err = discardLocalWikiChanges(localPath); err != nil {
		return fmt.Errorf("discardLocalWikiChanges: %v", err)
	} else if err = repo.UpdateLocalWiki(); err != nil {
		return fmt.Errorf("UpdateLocalWiki: %v", err)
	}

	title = ToWikiPageName(title)
	filename := path.Join(localPath, title+".md")

	// If not a new file, show perform update not create.
	if isNew {
		if com.IsExist(filename) {
			return ErrWikiAlreadyExist{filename}
		}
	} else {
		file := path.Join(localPath, oldTitle+".md")

		if err := os.Remove(file); err != nil {
			return fmt.Errorf("Fail to remove %s: %v", file, err)
		}
	}

	// SECURITY: if new file is a symlink to non-exist critical file,
	// attack content can be written to the target file (e.g. authorized_keys2)
	// as a new page operation.
	// So we want to make sure the symlink is removed before write anything.
	// The new file we created will be in normal text format.

	_ = os.Remove(filename)

	if err = ioutil.WriteFile(filename, []byte(content), 0666); err != nil {
		return fmt.Errorf("WriteFile: %v", err)
	}

	if len(message) == 0 {
		message = "Update page '" + title + "'"
	}
	if err = git.AddChanges(localPath, true); err != nil {
		return fmt.Errorf("AddChanges: %v", err)
	} else if err = git.CommitChanges(localPath, git.CommitChangesOptions{
		Committer: doer.NewGitSig(),
		Message:   message,
	}); err != nil {
		return fmt.Errorf("CommitChanges: %v", err)
	} else if err = git.Push(localPath, "origin", "master"); err != nil {
		return fmt.Errorf("Push: %v", err)
	}

	return nil
}

// AddWikiPage adds a new wiki page with a given title.
func (repo *Repository) AddWikiPage(doer *User, title, content, message string) error {
	return repo.updateWikiPage(doer, "", title, content, message, true)
}

// EditWikiPage updates a wiki page identified by its title,
// optionally also changing title.
func (repo *Repository) EditWikiPage(doer *User, oldTitle, title, content, message string) error {
	return repo.updateWikiPage(doer, oldTitle, title, content, message, false)
}

// DeleteWikiPage deletes a wiki page identified by its title.
func (repo *Repository) DeleteWikiPage(doer *User, title string) (err error) {
	wikiWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer wikiWorkingPool.CheckOut(com.ToStr(repo.ID))

	localPath := repo.LocalWikiPath()
	if err = discardLocalWikiChanges(localPath); err != nil {
		return fmt.Errorf("discardLocalWikiChanges: %v", err)
	} else if err = repo.UpdateLocalWiki(); err != nil {
		return fmt.Errorf("UpdateLocalWiki: %v", err)
	}

	title = ToWikiPageName(title)
	filename := path.Join(localPath, title+".md")

	if err := os.Remove(filename); err != nil {
		return fmt.Errorf("Fail to remove %s: %v", filename, err)
	}

	message := "Delete page '" + title + "'"

	if err = git.AddChanges(localPath, true); err != nil {
		return fmt.Errorf("AddChanges: %v", err)
	} else if err = git.CommitChanges(localPath, git.CommitChangesOptions{
		Committer: doer.NewGitSig(),
		Message:   message,
	}); err != nil {
		return fmt.Errorf("CommitChanges: %v", err)
	} else if err = git.Push(localPath, "origin", "master"); err != nil {
		return fmt.Errorf("Push: %v", err)
	}

	return nil
}
