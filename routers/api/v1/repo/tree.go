// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/modules/context"
	"fmt"
	"code.gitea.io/gitea/modules/setting"
	"strings"
	"code.gitea.io/git"
)

type TreeEntry struct {
	Path		string 		`json:"path"`
	Mode		string 		`json:"mode"`
	Type		string 		`json:"type"`
	Size		int64 		`json:"size,omitempty"`
	SHA		string		`json:"sha"`
	URL		string		`json:"url"`
}

type Tree struct {
	SHA		string		`json:"sha"`
	URL		string		`json:"url"`
	Entries		[]TreeEntry	`json:"tree,omitempty"`
	Truncated 	bool		`json:"truncated"`
}

func GetTree(ctx *context.APIContext) {
	sha := ctx.Params("sha")
	if len(sha) == 0 {
		ctx.Error(400, "sha not provided", nil)
		return
	}
	Tree := GetTreeBySHA(ctx, sha)
	if Tree != nil {
		ctx.JSON(200, Tree)
	} else {
		ctx.Error(400, "sha invalid", nil)
	}
}

func GetTreeBySHA(ctx *context.APIContext, sha string) *Tree {
	GitTree, err := ctx.Repo.GitRepo.GetTree(sha)
	if err != nil || GitTree == nil{
		return nil
	}
	tree := new(Tree)
	RepoID := strings.TrimRight(setting.AppURL, "/") + "/api/v1/repos/" + ctx.Repo.Repository.Owner.Name + "/" + ctx.Repo.Repository.Name
	tree.SHA = GitTree.ID.String()
	tree.URL = RepoID + "/trees/" + tree.SHA
	var Entries git.Entries
	if ctx.QueryBool("recursive") {
		Entries, err = GitTree.ListEntriesRecursive()
	} else {
		Entries, err = GitTree.ListEntries()
	}
	if err != nil {
		return tree
	}
	RepoIDLen := len(RepoID)
	BlobURL := make([]byte, RepoIDLen + 47)
	copy(BlobURL[:], RepoID)
	copy(BlobURL[RepoIDLen:], "/blobs/")
	TreeURL := make([]byte, RepoIDLen + 47)
	copy(TreeURL[:], RepoID)
	copy(TreeURL[RepoIDLen:], "/trees/")
	CopyPos := len(TreeURL) - 40

	if len(Entries) > 1000 {
		tree.Entries = make([]TreeEntry, 1000)
	} else {
		tree.Entries = make([]TreeEntry, len(Entries))
	}
	for e := range Entries {
		if e > 1000 {
			tree.Truncated = true
			break
		}

		tree.Entries[e].Path = Entries[e].Name()
		tree.Entries[e].Mode = fmt.Sprintf("%06x", Entries[e].Mode())
		tree.Entries[e].Type = string(Entries[e].Type)
		tree.Entries[e].Size = Entries[e].Size()
		tree.Entries[e].SHA = Entries[e].ID.String()

		if Entries[e].IsDir() {
			copy(TreeURL[CopyPos:], Entries[e].ID.String())
			tree.Entries[e].URL = string(TreeURL[:])
		} else {
			copy(BlobURL[CopyPos:], Entries[e].ID.String())
			tree.Entries[e].URL = string(BlobURL[:])
		}
	}
	return tree
}
