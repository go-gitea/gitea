// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	"gitea.dev/services/context/upload"
	user_service "gitea.dev/services/user"
)

type commentContext struct {
	NewFormURL       string
	CreateURL        string
	PermalinkBaseURL string
	ConversationURL  string
	CanModerate      bool
	SupportsReview   bool
}

func prepareCommentTemplateData(ctx *context.Context, commentCtx *commentContext) {
	ctx.Data["CommentContext"] = commentCtx
	ctx.Data["AllowedIssueReactions"] = setting.UI.Reactions
	ctx.Data["CanBlockUser"] = func(blocker, blockee *user_model.User) bool {
		return user_service.CanBlockUser(ctx, ctx.Doer, blocker, blockee)
	}
	upload.AddUploadContext(ctx, "comment")
}

func prepareIssueCommentTemplateData(ctx *context.Context, issue *issues_model.Issue, pageIsPullFiles bool) {
	ctx.Data["PageIsPullFiles"] = pageIsPullFiles
	prepareCommentTemplateData(ctx, &commentContext{
		NewFormURL:       issue.Link() + "/files/reviews/new_comment",
		CreateURL:        issue.Link() + "/files/reviews/comments",
		PermalinkBaseURL: issue.HTMLURL(ctx) + util.Iif(pageIsPullFiles, "/files", ""),
		ConversationURL:  issue.Link(),
		CanModerate:      ctx.Repo.Permission.IsAdmin() || ctx.Repo.Permission.CanWriteIssuesOrPulls(issue.IsPull),
		SupportsReview:   issue.IsPull,
	})
}

func prepareCommitCommentTemplateData(ctx *context.Context, commitSHA string) {
	commitLink := ctx.Repo.Repository.CommitLink(commitSHA)
	prepareCommentTemplateData(ctx, &commentContext{
		NewFormURL:       commitLink + "/comment",
		CreateURL:        commitLink + "/comment",
		PermalinkBaseURL: httplib.MakeAbsoluteURL(ctx, commitLink),
		ConversationURL:  commitLink,
		CanModerate:      ctx.Repo.Permission.IsAdmin() || ctx.Repo.Permission.CanWrite(unit.TypeCode),
	})
}
