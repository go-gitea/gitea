// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/git"
	"code.gitea.io/sdk/gitea"
)

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

func GetTreeBySHA(ctx *context.APIContext, sha string) *gitea.GitTreeResponse {
	GitTree, err := ctx.Repo.GitRepo.GetTree(sha)
	if err != nil || GitTree == nil{
		return nil
	}
	tree := new(gitea.GitTreeResponse)
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
		tree.Entries = make([]gitea.GitTreeEntry, 1000)
	} else {
		tree.Entries = make([]gitea.GitTreeEntry, len(Entries))
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
