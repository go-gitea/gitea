// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"context"
	"html/template"

	"code.gitea.io/gitea/modules/setting"
)

type LinkType string

const (
	LinkTypeApp     LinkType = "app"     // the link is relative to the AppSubURL
	LinkTypeDefault LinkType = "default" // the link is relative to the default base (eg: repo link, or current ref tree path)
	LinkTypeMedia   LinkType = "media"   // the link should be used to access media files (images, videos)
	LinkTypeRaw     LinkType = "raw"     // not really useful, mainly for environment GITEA_PREFIX_RAW for external renders
)

type RenderHelper interface {
	CleanUp()

	// TODO: such dependency is not ideal. We should decouple the processors step by step.
	// It should make the render choose different processors for different purposes,
	// but not make processors to guess "is it rendering a comment or a wiki?" or "does it need to check commit ID?"

	IsCommitIDExisting(commitID string) bool
	ResolveLink(link string, likeType LinkType) string
}

// RenderHelperFuncs is used to decouple cycle-import
// At the moment there are different packages:
// modules/markup: basic markup rendering
// models/renderhelper: need to access models and git repo, and models/issues needs it
// services/markup: some real helper functions could only be provided here because it needs to access various services & templates
type RenderHelperFuncs struct {
	IsUsernameMentionable     func(ctx context.Context, username string) bool
	RenderRepoFileCodePreview func(ctx context.Context, options RenderCodePreviewOptions) (template.HTML, error)
	RenderRepoIssueIconTitle  func(ctx context.Context, options RenderIssueIconTitleOptions) (template.HTML, error)
}

var DefaultRenderHelperFuncs *RenderHelperFuncs

type SimpleRenderHelper struct{}

func (r *SimpleRenderHelper) CleanUp() {}

func (r *SimpleRenderHelper) IsCommitIDExisting(commitID string) bool {
	return false
}

func (r *SimpleRenderHelper) ResolveLink(link string, likeType LinkType) string {
	return resolveLinkRelative(context.Background(), setting.AppSubURL+"/", "", link, false)
}

var _ RenderHelper = (*SimpleRenderHelper)(nil)
