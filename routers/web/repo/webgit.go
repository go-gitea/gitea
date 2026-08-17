// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"strings"

	user_model "gitea.dev/models/user"
	"gitea.dev/services/context"
	files_service "gitea.dev/services/repository/files"
)

func WebGitOperationCommonData(ctx *context.Context) {
	// TODO: more places like "wiki page" and "merging a pull request or creating an auto merge merging task"
	ctx.Data["CommitDefaultEmail"] = ctx.Doer.GetEmail()
	ctx.Data["CommitDefaultName"] = ctx.Doer.GitName()
	ctx.Data["commit_name"] = ctx.Doer.GitName()
}

func WebGitOperationGetCommitChosenEmailIdentity(ctx *context.Context, name, email string) (*files_service.IdentityOptions, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = ctx.Doer.GitName()
	} else if strings.ContainsAny(name, "\r\n<>") {
		return nil, false
	}
	chosenEmail := strings.TrimSpace(email)
	if chosenEmail == "" {
		chosenEmail = ctx.Doer.GetEmail()
	}
	if chosenEmail != ctx.Doer.GetPlaceholderEmail() {
		address, err := user_model.GetEmailAddressOfUser(ctx, chosenEmail, ctx.Doer.ID)
		if err != nil || address == nil || !address.IsActivated {
			return nil, false
		}
	}
	return &files_service.IdentityOptions{GitUserName: name, GitUserEmail: chosenEmail}, true
}
