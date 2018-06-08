// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/modules/context"
	"fmt"
	_"code.gitea.io/git"
	"code.gitea.io/gitea/modules/setting"
	"strings"
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
	Temp := GetTreeBySHA(ctx, nil, "", sha, ctx.QueryBool("recursive"))
	if Temp != nil {
		ctx.JSON(200, Temp)
	} else {
		ctx.Error(400, "sha invalid", nil)
	}

}

func GetTreeBySHA(ctx *context.APIContext, tree *Tree, CurrentPath string, sha string, recursive bool) *Tree {
	GitTree, err := ctx.Repo.GitRepo.GetTree(sha)
	if err != nil {
		return tree
	}
	RepoID := strings.TrimRight(setting.AppURL, "/") + "/api/v1/repos/" + ctx.Repo.Repository.Owner.Name + "/" + ctx.Repo.Repository.Name
	if tree == nil {
		tree = new(Tree)
		if GitTree != nil {
			tree.SHA = GitTree.ID.String()
			tree.URL = RepoID + "/trees/" + tree.SHA;
		}
	}
	if GitTree == nil {
		return tree
	}
	Trees, err := GitTree.ListEntries()
	if err != nil {
		return tree
	}
	if len(CurrentPath) != 0 {
		CurrentPath += "/"
	}
	for e := range Trees {
		if len(tree.Entries) > 1000 {
			tree.Truncated = true
			break
		}
		E_URL := RepoID
		if Trees[e].IsDir() {
			E_URL += "/trees/"
		} else {
			E_URL += "/blobs/"
		}
		tree.Entries = append(tree.Entries, TreeEntry{
			CurrentPath + Trees[e].Name(),
			fmt.Sprintf("%06x", Trees[e].Mode()),
			string(Trees[e].Type),
			Trees[e].Size(),
			Trees[e].ID.String(),
			E_URL + Trees[e].ID.String()})

		if recursive && Trees[e].IsDir() {
			tree = GetTreeBySHA(ctx, tree, CurrentPath + Trees[e].Name(), Trees[e].ID.String(), recursive)
		}
	}
	return tree
}
