// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
)

// TreeEntryFollowLink returns the entry pointed to by a symlink
func TreeEntryFollowLink(te service.TreeEntry) (service.TreeEntry, error) {
	if !te.Mode().IsLink() {
		return nil, git.ErrBadLink{
			Name:    te.Name(),
			Message: "not a symlink",
		}
	}

	// read the link
	r, err := te.Reader()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	buf := make([]byte, te.Size())
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

	lnk := string(buf)
	t := te.Tree()

	// traverse up directories
	for ; t != nil && strings.HasPrefix(lnk, "../"); lnk = lnk[3:] {
		t, err = t.Parent()
		if err != nil {
			return nil, err
		}
	}

	if t == nil {
		return nil, git.ErrBadLink{
			Name:    te.Name(),
			Message: "points outside of repo",
		}
	}

	target, err := t.GetTreeEntryByPath(lnk)
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil, git.ErrBadLink{
				Name:    te.Name(),
				Message: "broken link",
			}
		}
		return nil, err
	}
	return target, nil
}

// TreeEntryFollowLinks returns the entry ultimately pointed to by a symlink
func TreeEntryFollowLinks(te service.TreeEntry) (service.TreeEntry, error) {
	if !te.Mode().IsLink() {
		return nil, git.ErrBadLink{
			Name:    te.Name(),
			Message: "not a symlink",
		}
	}
	var entry (service.TreeEntry) = te
	for i := 0; i < 999; i++ {
		if entry.Mode().IsLink() {
			next, err := entry.FollowLink()
			if err != nil {
				return nil, err
			}
			if next.ID().String() == entry.ID().String() {
				return nil, git.ErrBadLink{
					Name:    entry.Name(),
					Message: "recursive link",
				}
			}
			entry = next
		} else {
			break
		}
	}
	if entry.Mode().IsLink() {
		return nil, git.ErrBadLink{
			Name:    te.Name(),
			Message: "too many levels of symbolic links",
		}
	}
	return entry, nil
}

// TreeEntryGetSubJumpablePathName return the full path of subdirectory jumpable ( contains only one directory )
func TreeEntryGetSubJumpablePathName(te service.TreeEntry) string {
	if te.Mode().IsSubModule() || !te.Mode().IsDir() {
		return ""
	}
	tree, err := te.Tree().SubTree(te.Name())
	if err != nil {
		return te.Name()
	}
	entries, _ := tree.ListEntries()
	if len(entries) == 1 && entries[0].Mode().IsDir() {
		name := entries[0].GetSubJumpablePathName()
		if name != "" {
			return te.Name() + "/" + name
		}
	}
	return te.Name()
}
