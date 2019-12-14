// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/unknwon/com"
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
