// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package console

import (
	"bytes"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"

	trend "github.com/buildkite/terminal-to-html/v3"
	"github.com/go-enry/go-enry/v2"
)

// MarkupName describes markup's name
var MarkupName = "console"

func init() {
	markup.RegisterRenderer(Renderer{})
}

// Renderer implements markup.Renderer
type Renderer struct{}

// Name implements markup.Renderer
func (Renderer) Name() string {
	return MarkupName
}

// Extensions implements markup.Renderer
func (Renderer) Extensions() []string {
	return []string{".sh-session"}
}

// SanitizerRules implements markup.Renderer
func (Renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return []setting.MarkupSanitizerRule{
		{Element: "span", AllowAttr: "class", Regexp: regexp.MustCompile(`^term-((fg[ix]?|bg)\d+|container)$`)},
	}
}

// CanRender implements markup.RendererContentDetector
func (Renderer) CanRender(filename string, input io.Reader) bool {
	buf, err := io.ReadAll(input)
	if err != nil {
		return false
	}
	if enry.GetLanguage(filepath.Base(filename), buf) != enry.OtherLanguage {
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
	buf = trend.Render(buf)
	buf = bytes.ReplaceAll(buf, []byte("\n"), []byte(`<br>`))
	_, err = output.Write(buf)
	return err
}

// Render renders terminal colors to HTML with all specific handling stuff.
func Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	if ctx.Type == "" {
		ctx.Type = MarkupName
	}
	return markup.Render(ctx, input, output)
}

// RenderString renders terminal colors in string to HTML with all specific handling stuff and return string
func RenderString(ctx *markup.RenderContext, content string) (string, error) {
	var buf strings.Builder
	if err := Render(ctx, strings.NewReader(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
