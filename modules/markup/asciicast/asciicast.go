// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asciicast

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"regexp"

	"code.gitea.io/gitea/modules/log"
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
	return []setting.MarkupSanitizerRule{
		{Element: "div", AllowAttr: "class", Regexp: regexp.MustCompile(playerClassName)},
		{Element: "div", AllowAttr: "style"},
		{Element: "div", AllowAttr: playerSrcAttr},
	}
}

// Render implements markup.Renderer
func (Renderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	var h *header
	if firstLine, err := bufio.NewReader(input).ReadBytes('\n'); err != nil {
		return err
	} else if h, err = extractHeader(firstLine); err != nil {
		log.Warn("extract header from %s: %v", ctx.RelativePath, err)
	}

	rawURL := fmt.Sprintf("%s/%s/%s/raw/%s/%s",
		setting.AppSubURL,
		url.PathEscape(ctx.Metas["user"]),
		url.PathEscape(ctx.Metas["repo"]),
		ctx.Metas["BranchNameSubURL"],
		url.PathEscape(ctx.RelativePath),
	)

	style := ""
	if h != nil {
		style = fmt.Sprintf("aspect-ratio: %d / %d", h.Width, h.Height)
	}

	_, err := io.WriteString(output, fmt.Sprintf(
		`<div class="%s" style="%s" %s="%s"></div>`,
		playerClassName,
		style,
		playerSrcAttr,
		rawURL,
	))
	return err
}
