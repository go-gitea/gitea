// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asciicast

import (
	"fmt"
	"io"
	"net/url"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
)

func init() {
	markup.RegisterRenderer(Renderer{})
}

// Renderer implements markup.Renderer for asciicast files.
// See https://github.com/asciinema/asciinema/blob/develop/doc/asciicast-v2.md
type Renderer struct{}

// Name implements markup.Renderer
func (Renderer) Name() string {
	return "asciicast"
}

// Extensions implements markup.Renderer
func (Renderer) Extensions() []string {
	return []string{".cast"}
}

const (
	playerClassName = "asciinema-player-container"
	playerSrcAttr   = "data-asciinema-player-src"
)

// SanitizerRules implements markup.Renderer
func (Renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return []setting.MarkupSanitizerRule{{Element: "div", AllowAttr: playerSrcAttr}}
}

// Render implements markup.Renderer
func (Renderer) Render(ctx *markup.RenderContext, _ io.Reader, output io.Writer) error {
	rawURL := fmt.Sprintf("%s/%s/%s/raw/%s/%s",
		setting.AppSubURL,
		url.PathEscape(ctx.RenderOptions.Metas["user"]),
		url.PathEscape(ctx.RenderOptions.Metas["repo"]),
		ctx.RenderOptions.Metas["BranchNameSubURL"],
		url.PathEscape(ctx.RenderOptions.RelativePath),
	)
	return ctx.RenderInternal.FormatWithSafeAttrs(output, `<div class="%s" %s="%s"></div>`, playerClassName, playerSrcAttr, rawURL)
}
