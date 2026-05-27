// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"
	"net/url"

	"gitea.dev/models/avatars"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/setting"
)

func (g *Group) CustomAvatarRelativePath() string {
	return g.Avatar
}

func (g *Group) relAvatarLink() string {
	// If no avatar - path is empty
	avatarPath := g.CustomAvatarRelativePath()
	if len(avatarPath) == 0 {
		return ""
	}
	return setting.AppSubURL + "/group-avatars/" + url.PathEscape(g.Avatar)
}

func (g *Group) AvatarLink(ctx context.Context) string {
	relLink := g.relAvatarLink()
	if relLink != "" {
		return httplib.MakeAbsoluteURL(ctx, relLink)
	}
	return ""
}

func (g *Group) AvatarLinkWithSize(size int) string {
	if g.Avatar == "" {
		return avatars.DefaultAvatarLink()
	}
	return avatars.GenerateUserAvatarImageLink(g.Avatar, size)
}
