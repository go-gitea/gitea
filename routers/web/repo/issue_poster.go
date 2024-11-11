// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"slices"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
)

type userSearchInfo struct {
	UserID     int64  `json:"user_id"`
	UserName   string `json:"username"`
	AvatarLink string `json:"avatar_link"`
	FullName   string `json:"full_name"`
}

type userSearchResponse struct {
	Results []*userSearchInfo `json:"results"`
}

// IssuePosters get posters for current repo's issues/pull requests
func IssuePosters(ctx *context.Context) {
	issuePosters(ctx, false)
}

func PullPosters(ctx *context.Context) {
	issuePosters(ctx, true)
}

func issuePosters(ctx *context.Context, isPullList bool) {
	repo := ctx.Repo.Repository
	search := strings.TrimSpace(ctx.FormString("q"))
	posters, err := repo_model.GetIssuePostersWithSearch(ctx, repo, isPullList, search, setting.UI.DefaultShowFullName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}

	if search == "" && ctx.Doer != nil {
		// the returned posters slice only contains limited number of users,
		// to make the current user (doer) can quickly filter their own issues, always add doer to the posters slice
		if !slices.ContainsFunc(posters, func(user *user_model.User) bool { return user.ID == ctx.Doer.ID }) {
			posters = append(posters, ctx.Doer)
		}
	}

	posters = shared_user.MakeSelfOnTop(ctx.Doer, posters)

	resp := &userSearchResponse{}
	resp.Results = make([]*userSearchInfo, len(posters))
	for i, user := range posters {
		resp.Results[i] = &userSearchInfo{UserID: user.ID, UserName: user.Name, AvatarLink: user.AvatarLink(ctx)}
		if setting.UI.DefaultShowFullName {
			resp.Results[i].FullName = user.FullName
		}
	}
	ctx.JSON(http.StatusOK, resp)
}
