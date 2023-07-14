// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"
	"fmt"
	"html"
	"html/template"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/avatars"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	gitea_html "code.gitea.io/gitea/modules/html"
	"code.gitea.io/gitea/modules/setting"
)

// AvatarHTML creates the HTML for an avatar
func AvatarHTML(src string, size int, class, name string) template.HTML {
	sizeStr := fmt.Sprintf(`%d`, size)

	if name == "" {
		name = "avatar"
	}

	return template.HTML(`<img class="` + class + `" src="` + src + `" title="` + html.EscapeString(name) + `" width="` + sizeStr + `" height="` + sizeStr + `"/>`)
}

// Avatar renders user avatars. args: user, size (int), class (string)
func Avatar(ctx context.Context, item any, others ...any) template.HTML {
	size, class := gitea_html.ParseSizeAndClass(avatars.DefaultAvatarPixelSize, avatars.DefaultAvatarClass, others...)

	switch t := item.(type) {
	case *user_model.User:
		src := t.AvatarLinkWithSize(ctx, size*setting.Avatar.RenderedSizeFactor)
		if src != "" {
			return AvatarHTML(src, size, class, t.DisplayName())
		}
	case *repo_model.Collaborator:
		src := t.AvatarLinkWithSize(ctx, size*setting.Avatar.RenderedSizeFactor)
		if src != "" {
			return AvatarHTML(src, size, class, t.DisplayName())
		}
	case *organization.Organization:
		src := t.AsUser().AvatarLinkWithSize(ctx, size*setting.Avatar.RenderedSizeFactor)
		if src != "" {
			return AvatarHTML(src, size, class, t.AsUser().DisplayName())
		}
	}

	return template.HTML("")
}

// AvatarByAction renders user avatars from action. args: action, size (int), class (string)
func AvatarByAction(ctx context.Context, action *activities_model.Action, others ...any) template.HTML {
	action.LoadActUser(ctx)
	return Avatar(ctx, action.ActUser, others...)
}

// RepoAvatar renders repo avatars. args: repo, size(int), class (string)
func RepoAvatar(repo *repo_model.Repository, others ...any) template.HTML {
	size, class := gitea_html.ParseSizeAndClass(avatars.DefaultAvatarPixelSize, avatars.DefaultAvatarClass, others...)

	src := repo.RelAvatarLink()
	if src != "" {
		return AvatarHTML(src, size, class, repo.FullName())
	}
	return template.HTML("")
}

// AvatarByEmail renders avatars by email address. args: email, name, size (int), class (string)
func AvatarByEmail(ctx context.Context, email, name string, others ...any) template.HTML {
	size, class := gitea_html.ParseSizeAndClass(avatars.DefaultAvatarPixelSize, avatars.DefaultAvatarClass, others...)
	src := avatars.GenerateEmailAvatarFastLink(ctx, email, size*setting.Avatar.RenderedSizeFactor)

	if src != "" {
		return AvatarHTML(src, size, class, name)
	}

	return template.HTML("")
}
