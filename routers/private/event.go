// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	access_model "code.gitea.io/gitea/models/perm/access"
	user_model "code.gitea.io/gitea/models/user"
	gitea_context "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/web"
)

type BranchUpdateEvent struct {
	CommitID      string
	Branch        string
	BranchDeleted bool
	Owner         string
	RefFullName   string
	Repository    string
}

func SendBranchUpdateEvent(readers []*user_model.User, commitID, branchName, repoName, refFullName, ownerName string) {
	manager := eventsource.GetManager()

	for _, reader := range readers {
		manager.SendMessage(reader.ID, &eventsource.Event{
			Name: "branch-update",
			Data: BranchUpdateEvent{
				CommitID:      commitID,
				Branch:        branchName,
				BranchDeleted: commitID == git.EmptySHA,
				Repository:    repoName,
				RefFullName:   refFullName,
				Owner:         ownerName,
			},
		})
	}
}

func SendContextBranchUpdateEvents(ctx *gitea_context.PrivateContext) error {
	opts := web.GetForm(ctx).(*private.HookOptions)

	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")

	repo := loadRepository(ctx, ownerName, repoName)

	readers, err := access_model.GetRepoReaders(repo)
	if err != nil {
		return err
	}

	for i := range opts.OldCommitIDs {
		branch := git.RefEndName(opts.RefFullNames[i])
		commitID := opts.NewCommitIDs[i]
		refFullName := opts.RefFullNames[i]

		SendBranchUpdateEvent(readers, commitID, branch, repoName, refFullName, ownerName)
	}

	return nil
}
