// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package console

import (
	"bytes"
	"io"
	"path"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"

	trend "github.com/buildkite/terminal-to-html/v3"
	"github.com/go-enry/go-enry/v2"
)

func init() {
	markup.RegisterRenderer(Renderer{})
}

// Renderer implements markup.Renderer
type Renderer struct{}

// Name implements markup.Renderer
func (Renderer) Name() string {
	return "console"
}

// Extensions implements markup.Renderer
func (Renderer) Extensions() []string {
	return []string{".sh-session"}
}

// SanitizerRules implements markup.Renderer
func (Renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return []setting.MarkupSanitizerRule{
		{Element: "span", AllowAttr: "class", Regexp: `^term-((fg[ix]?|bg)\d+|container)$`},
	}
}

// CanRender implements markup.RendererContentDetector
func (Renderer) CanRender(filename string, input io.Reader) bool {
	buf, err := io.ReadAll(input)
	if err != nil {
		return false
	}
	if enry.GetLanguage(path.Base(filename), buf) != enry.OtherLanguage {
		return false
	}
	return bytes.ContainsRune(buf, '\x1b')
}

// Render renders terminal colors to HTML with all specific handling stuff.
func (Renderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	buf, err := io.ReadAll(input)
	if err != nil {
		return err
	}
	buf = []byte(trend.Render(buf))
	buf = bytes.ReplaceAll(buf, []byte("\n"), []byte(`<br>`))
	_, err = output.Write(buf)
	return err
}
