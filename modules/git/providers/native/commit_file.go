// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"bufio"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
)

//  _
// |_ o |  _
// |  | | (/_
//

// GetFilesChangedSinceCommit get all changed file names between pastCommit to current revision
func (commit *Commit) GetFilesChangedSinceCommit(pastCommit string) ([]string, error) {
	return gitService.GetFilesChanged(commit.Repository(), pastCommit, commit.ID().String())
}

// FileChangedSinceCommit Returns true if the file given has changed since the the past commit
// YOU MUST ENSURE THAT pastCommit is a valid commit ID.
func (commit *Commit) FileChangedSinceCommit(filename, pastCommit string) (bool, error) {
	return gitService.FileChangedBetweenCommits(commit.Repository(), filename, pastCommit, commit.ID().String())
}

// HasFile returns true if the file given exists on this commit
// This does only mean it's there - it does not mean the file was changed during the commit.
func (commit *Commit) HasFile(filename string) (bool, error) {
	_, err := commit.Tree().GetBlobByPath(filename)
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetSubModules get all the sub modules of current revision git tree
func (commit *Commit) GetSubModules() (service.ObjectCache, error) {
	if commit.submoduleCache != nil {
		return commit.submoduleCache, nil
	}

	entry, err := commit.Tree().GetTreeEntryByPath(".gitmodules")
	if err != nil {
		if _, ok := err.(git.ErrNotExist); ok {
			return nil, nil
		}
		return nil, err
	}

	rd, err := entry.Reader()
	if err != nil {
		return nil, err
	}

	defer rd.Close()
	scanner := bufio.NewScanner(rd)
	commit.submoduleCache = git.NewObjectCache()
	var ismodule bool
	var path string
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "[submodule") {
			ismodule = true
			continue
		}
		if ismodule {
			fields := strings.Split(scanner.Text(), "=")
			k := strings.TrimSpace(fields[0])
			if k == "path" {
				path = strings.TrimSpace(fields[1])
			} else if k == "url" {
				commit.submoduleCache.Set(path, &service.SubModule{
					Name: path,
					URL:  strings.TrimSpace(fields[1]),
				})
				ismodule = false
			}
		}
	}

	return commit.submoduleCache, nil
}

// GetSubModule get the sub module according entryname
func (commit *Commit) GetSubModule(entryname string) (*service.SubModule, error) {
	modules, err := commit.GetSubModules()
	if err != nil {
		return nil, err
	}

	if modules != nil {
		module, has := modules.Get(entryname)
		if has {
			return module.(*service.SubModule), nil
		}
	}
	return nil, nil
}
