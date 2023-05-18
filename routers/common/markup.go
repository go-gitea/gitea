// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"mvdan.cc/xurls/v2"
)

// RenderMarkup renders markup text for the /markup and /markdown endpoints
func RenderMarkup(ctx *context.Base, repo *context.Repository, mode, text, urlPrefix, filePath string, wiki bool) {
	var markupType string
	relativePath := ""

	if len(text) == 0 {
		_, _ = ctx.Write([]byte(""))
		return
	}

	switch mode {
	case "markdown":
		// Raw markdown
		if err := markdown.RenderRaw(&markup.RenderContext{
			Ctx:       ctx,
			URLPrefix: urlPrefix,
		}, strings.NewReader(text), ctx.Resp); err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
		}
		return
	case "comment":
		// Comment as markdown
		markupType = markdown.MarkupName
	case "gfm":
		// Github Flavored Markdown as document
		markupType = markdown.MarkupName
	case "file":
		// File as document based on file extension
		markupType = ""
		relativePath = filePath
	default:
		ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("Unknown mode: %s", mode))
		return
	}

	if !strings.HasPrefix(setting.AppSubURL+"/", urlPrefix) {
		// check if urlPrefix is already set to a URL
		linkRegex, _ := xurls.StrictMatchingScheme("https?://")
		m := linkRegex.FindStringIndex(urlPrefix)
		if m == nil {
			urlPrefix = util.URLJoin(setting.AppURL, urlPrefix)
		}
	}

	meta := map[string]string{}
	if repo != nil && repo.Repository != nil {
		if mode == "comment" {
			meta = repo.Repository.ComposeMetas()
		} else {
			meta = repo.Repository.ComposeDocumentMetas()
		}
	}
	if mode != "comment" {
		meta["mode"] = "document"
	}

	if err := markup.Render(&markup.RenderContext{
		Ctx:          ctx,
		URLPrefix:    urlPrefix,
		Metas:        meta,
		IsWiki:       wiki,
		Type:         markupType,
		RelativePath: relativePath,
	}, strings.NewReader(text), ctx.Resp); err != nil {
		if markup.IsErrUnsupportedRenderExtension(err) {
			ctx.Error(http.StatusUnprocessableEntity, err.Error())
		} else {
			ctx.Error(http.StatusInternalServerError, err.Error())
		}
		return
	}
}
