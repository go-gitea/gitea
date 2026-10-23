// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	user_model "codenest.dev/models/user"
	"codenest.dev/modules/log"
	"codenest.dev/modules/setting"
	"codenest.dev/modules/util"
	"codenest.dev/services/context"
	files_service "codenest.dev/services/repository/files"
)

func WebGitOperationCommonData(ctx *context.Context) {
	// TODO: more places like "wiki page" and "merging a pull request or creating an auto merge merging task"
	emails, err := user_model.GetActivatedEmailAddresses(ctx, ctx.Doer.ID)
	if err != nil {
		log.Error("WebGitOperationCommonData: GetActivatedEmailAddresses: %v", err)
	}
	if ctx.Doer.KeepEmailPrivate {
		emails = append([]string{ctx.Doer.GetPlaceholderEmail()}, emails...)
	}
	ctx.Data["CommitCandidateEmails"] = emails
	ctx.Data["CommitDefaultEmail"] = ctx.Doer.GetEmail()
}

func WebGitOperationGetCommitChosenEmailIdentity(ctx *context.Context, email string) (_ *files_service.IdentityOptions, valid bool) {
	if ctx.Data["CommitCandidateEmails"] == nil {
		setting.PanicInDevOrTesting("no CommitCandidateEmails in context data")
	}
	emails, _ := ctx.Data["CommitCandidateEmails"].([]string)
	if email == "" {
		return nil, true
	}
	if util.SliceContainsString(emails, email, true) {
		return &files_service.IdentityOptions{GitUserEmail: email}, true
	}
	return nil, false
}
