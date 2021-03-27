// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

// Init initialize regexps for markdown parsing
func Init() {
	getIssueFullPattern()
	NewSanitizer()
	if len(setting.Markdown.CustomURLSchemes) > 0 {
		CustomLinkURLSchemes(setting.Markdown.CustomURLSchemes)
	}

	// since setting maybe changed extensions, this will reload all renderer extensions mapping
	extRenderers = make(map[string]Renderer)
	for _, renderer := range renderers {
		for _, ext := range renderer.Extensions() {
			extRenderers[strings.ToLower(ext)] = renderer
		}
	}
}

// RenderContext represents a render context
type RenderContext struct {
	Ctx         context.Context
	Filename    string
	Type        string
	IsWiki      bool
	URLPrefix   string
	Metas       map[string]string
	DefaultLink string
}

// Renderer defines an interface for rendering markup file to HTML
type Renderer interface {
	Name() string // markup format name
	Extensions() []string
	Render(ctx *RenderContext, input io.Reader, output io.Writer) error
}

var (
	extRenderers = make(map[string]Renderer)
	renderers    = make(map[string]Renderer)
)

// RegisterRenderer registers a new markup file renderer
func RegisterRenderer(renderer Renderer) {
	renderers[renderer.Name()] = renderer
	for _, ext := range renderer.Extensions() {
		extRenderers[strings.ToLower(ext)] = renderer
	}
}

// GetRendererByFileName get renderer by filename
func GetRendererByFileName(filename string) Renderer {
	extension := strings.ToLower(filepath.Ext(filename))
	return extRenderers[extension]
}

// GetRendererByType returns a renderer according type
func GetRendererByType(tp string) Renderer {
	return renderers[tp]
}

// Render renders markup file to HTML with all specific handling stuff.
func Render(ctx *RenderContext, input io.Reader, output io.Writer) error {
	if ctx.Type != "" {
		return renderByType(ctx, input, output)
	} else if ctx.Filename != "" {
		return renderFile(ctx, input, output)
	}
	return errors.New("Render options both filename and type missing")
}

// RenderString renders Markup string to HTML with all specific handling stuff and return string
func RenderString(ctx *RenderContext, content string) (string, error) {
	var buf strings.Builder
	if err := Render(ctx, strings.NewReader(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func render(ctx *RenderContext, parser Renderer, input io.Reader, output io.Writer) error {
	var buf1 strings.Builder
	if err := parser.Render(ctx, input, &buf1); err != nil {
		return err
	}

	var buf2 strings.Builder
	if err := PostProcess(ctx, strings.NewReader(buf1.String()), &buf2); err != nil {
		return fmt.Errorf("PostProcess: %v", err)
	}
	buf := SanitizeReader(strings.NewReader(buf2.String()))
	_, err := io.Copy(output, buf)
	return err
}

func renderByType(ctx *RenderContext, input io.Reader, output io.Writer) error {
	if parser, ok := renderers[ctx.Type]; ok {
		return render(ctx, parser, input, output)
	}
	return nil
}

func renderFile(ctx *RenderContext, input io.Reader, output io.Writer) error {
	extension := strings.ToLower(filepath.Ext(ctx.Filename))
	if parser, ok := extRenderers[extension]; ok {
		return render(ctx, parser, input, output)
	}
	return nil
}

// Type returns if markup format via the filename
func Type(filename string) string {
	if parser := GetRendererByFileName(filename); parser != nil {
		return parser.Name()
	}
	return ""
}

// IsMarkupFile reports whether file is a markup type file
func IsMarkupFile(name, markup string) bool {
	if parser := GetRendererByFileName(name); parser != nil {
		return parser.Name() == markup
	}
	return false
}

// IsReadmeFile reports whether name looks like a README file
// based on its name. If an extension is provided, it will strictly
// match that extension.
// Note that the '.' should be provided in ext, e.g ".md"
func IsReadmeFile(name string, ext ...string) bool {
	name = strings.ToLower(name)
	if len(ext) > 0 {
		return name == "readme"+ext[0]
	}
	if len(name) < 6 {
		return false
	} else if len(name) == 6 {
		return name == "readme"
	}
	return name[:7] == "readme."
}
