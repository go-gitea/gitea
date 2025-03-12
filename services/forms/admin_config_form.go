// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"net/http"

	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/context"

	"gitea.com/go-chi/binding"
)

type UIForm struct {
	ExplorePagingNum         int
	SitemapPagingNum         int
	IssuePagingNum           int
	RepoSearchPagingNum      int
	MembersPagingNum         int
	FeedMaxCommitNum         int
	FeedPagingNum            int
	PackagesPagingNum        int
	GraphMaxCommitNum        int
	CodeCommentLines         int
	ReactionMaxUserNum       int
	MaxDisplayFileSize       int64
	ShowUserEmail            bool
	DefaultShowFullName      bool
	DefaultTheme             string
	Themes                   []string
	SearchRepoDescription    bool
	OnlyShowRelevantRepos    bool
	ExplorePagingDefaultSort string `binding:"In(recentupdate,alphabetically,reverselastlogin,newest,oldest)"`
	PreferredTimestampTense  string `binding:"In(mixed,absolute)"`

	AmbiguousUnicodeDetection bool
}

// Validate validates fields
func (f *UIForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}
