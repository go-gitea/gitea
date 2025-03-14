// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"fmt"
	"html"
	"html/template"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/htmlutil"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/niklasfasching/go-org/org"
)

func init() {
	markup.RegisterRenderer(renderer{})
}

// Renderer implements markup.Renderer for orgmode
type renderer struct{}

var (
	_ markup.Renderer            = (*renderer)(nil)
	_ markup.PostProcessRenderer = (*renderer)(nil)
)

// Name implements markup.Renderer
func (renderer) Name() string {
	return "orgmode"
}

// NeedPostProcess implements markup.PostProcessRenderer
func (renderer) NeedPostProcess() bool { return true }

// Extensions implements markup.Renderer
func (renderer) Extensions() []string {
	return []string{".org"}
}

// SanitizerRules implements markup.Renderer
func (renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return []setting.MarkupSanitizerRule{}
}

// Render renders orgmode raw bytes to HTML
func Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	htmlWriter := org.NewHTMLWriter()
	htmlWriter.HighlightCodeBlock = func(source, lang string, inline bool, params map[string]string) string {
		defer func() {
			if err := recover(); err != nil {
				log.Error("Panic in HighlightCodeBlock: %v\n%s", err, log.Stack(2))
				panic(err)
			}
		}()
		w := &strings.Builder{}

		lexer := lexers.Get(lang)
		if lexer == nil && lang == "" {
			lexer = lexers.Analyse(source)
			if lexer == nil {
				lexer = lexers.Fallback
			}
			lang = strings.ToLower(lexer.Config().Name)
		}

		// include language-x class as part of commonmark spec
		if err := ctx.RenderInternal.FormatWithSafeAttrs(w, `<pre><code class="chroma language-%s">`, lang); err != nil {
			return ""
		}
		if lexer == nil {
			if _, err := w.WriteString(html.EscapeString(source)); err != nil {
				return ""
			}
		} else {
			lexer = chroma.Coalesce(lexer)
			if _, err := w.WriteString(string(highlight.CodeFromLexer(lexer, source))); err != nil {
				return ""
			}
		}
		if _, err := w.WriteString("</code></pre>"); err != nil {
			return ""
		}

		return w.String()
	}

	w := &orgWriter{rctx: ctx, HTMLWriter: htmlWriter}
	htmlWriter.ExtendingWriter = w

	res, err := org.New().Silent().Parse(input, "").Write(w)
	if err != nil {
		return fmt.Errorf("orgmode.Render failed: %w", err)
	}
	_, err = io.Copy(output, strings.NewReader(res))
	return err
}

// RenderString renders orgmode string to HTML string
func RenderString(ctx *markup.RenderContext, content string) (string, error) {
	var buf strings.Builder
	if err := Render(ctx, strings.NewReader(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Render renders orgmode string to HTML string
func (renderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	return Render(ctx, input, output)
}

type orgWriter struct {
	*org.HTMLWriter
	rctx *markup.RenderContext
}

var _ org.Writer = (*orgWriter)(nil)

func (r *orgWriter) resolveLink(kind, link string) string {
	link = strings.TrimPrefix(link, "file:")
	if !strings.HasPrefix(link, "#") && // not a URL fragment
		!markup.IsFullURLString(link) {
		if kind == "regular" {
			// orgmode reports the link kind as "regular" for "[[ImageLink.svg][The Image Desc]]"
			// so we need to try to guess the link kind again here
			kind = org.RegularLink{URL: link}.Kind()
		}
		if kind == "image" || kind == "video" {
			link = r.rctx.RenderHelper.ResolveLink(link, markup.LinkTypeMedia)
		} else {
			link = r.rctx.RenderHelper.ResolveLink(link, markup.LinkTypeDefault)
		}
	}
	return link
}

// WriteRegularLink renders images, links or videos
func (r *orgWriter) WriteRegularLink(l org.RegularLink) {
	link := r.resolveLink(l.Kind(), l.URL)

	printHTML := func(html template.HTML, a ...any) {
		_, _ = fmt.Fprint(r, htmlutil.HTMLFormat(html, a...))
	}
	// Inspired by https://github.com/niklasfasching/go-org/blob/6eb20dbda93cb88c3503f7508dc78cbbc639378f/org/html_writer.go#L406-L427
	switch l.Kind() {
	case "image":
		if l.Description == nil {
			printHTML(`<img src="%s" alt="%s">`, link, link)
		} else {
			imageSrc := r.resolveLink(l.Kind(), org.String(l.Description...))
			printHTML(`<a href="%s"><img src="%s" alt="%s"></a>`, link, imageSrc, imageSrc)
		}
	case "video":
		if l.Description == nil {
			printHTML(`<video src="%s">%s</video>`, link, link)
		} else {
			videoSrc := r.resolveLink(l.Kind(), org.String(l.Description...))
			printHTML(`<a href="%s"><video src="%s">%s</video></a>`, link, videoSrc, videoSrc)
		}
	default:
		var description any = link
		if l.Description != nil {
			description = template.HTML(r.WriteNodesAsString(l.Description...)) // orgmode HTMLWriter outputs HTML content
		}
		printHTML(`<a href="%s">%s</a>`, link, description)
	}
}
