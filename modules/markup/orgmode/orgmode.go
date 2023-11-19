// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/niklasfasching/go-org/org"
)

func init() {
	markup.RegisterRenderer(Renderer{})
}

// Renderer implements markup.Renderer for orgmode
type Renderer struct{}

var _ markup.PostProcessRenderer = (*Renderer)(nil)

// Name implements markup.Renderer
func (Renderer) Name() string {
	return "orgmode"
}

// NeedPostProcess implements markup.PostProcessRenderer
func (Renderer) NeedPostProcess() bool { return true }

// Extensions implements markup.Renderer
func (Renderer) Extensions() []string {
	return []string{".org"}
}

// SanitizerRules implements markup.Renderer
func (Renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return []setting.MarkupSanitizerRule{}
}

// Render renders orgmode rawbytes to HTML
func Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	htmlWriter := org.NewHTMLWriter()
	htmlWriter.HighlightCodeBlock = func(source, lang string, inline bool, params map[string]string) string {
		defer func() {
			if err := recover(); err != nil {
				log.Error("Panic in HighlightCodeBlock: %v\n%s", err, log.Stack(2))
				panic(err)
			}
		}()
		var w strings.Builder
		if _, err := w.WriteString(`<pre>`); err != nil {
			return ""
		}

		lexer := lexers.Get(lang)
		if lexer == nil && lang == "" {
			lexer = lexers.Analyse(source)
			if lexer == nil {
				lexer = lexers.Fallback
			}
			lang = strings.ToLower(lexer.Config().Name)
		}

		if lexer == nil {
			// include language-x class as part of commonmark spec
			if _, err := w.WriteString(`<code class="chroma language-` + lang + `">`); err != nil {
				return ""
			}
			if _, err := w.WriteString(html.EscapeString(source)); err != nil {
				return ""
			}
		} else {
			// include language-x class as part of commonmark spec
			if _, err := w.WriteString(`<code class="chroma language-` + lang + `">`); err != nil {
				return ""
			}
			lexer = chroma.Coalesce(lexer)

			if _, err := w.WriteString(highlight.CodeFromLexer(lexer, source)); err != nil {
				return ""
			}
		}

		if _, err := w.WriteString("</code></pre>"); err != nil {
			return ""
		}

		return w.String()
	}

	w := &Writer{
		HTMLWriter: htmlWriter,
		URLPrefix:  ctx.URLPrefix,
		IsWiki:     ctx.IsWiki,
	}

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
func (Renderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	return Render(ctx, input, output)
}

// Writer implements org.Writer
type Writer struct {
	*org.HTMLWriter
	URLPrefix string
	IsWiki    bool
}

var byteMailto = []byte("mailto:")

// WriteRegularLink renders images, links or videos
func (r *Writer) WriteRegularLink(l org.RegularLink) {
	link := []byte(html.EscapeString(l.URL))
	if l.Protocol == "file" {
		link = link[len("file:"):]
	}
	if len(link) > 0 && !markup.IsLink(link) &&
		link[0] != '#' && !bytes.HasPrefix(link, byteMailto) {
		lnk := string(link)
		if r.IsWiki {
			lnk = util.URLJoin("wiki", lnk)
		}
		link = []byte(util.URLJoin(r.URLPrefix, lnk))
	}

	// Inspired by https://github.com/niklasfasching/go-org/blob/6eb20dbda93cb88c3503f7508dc78cbbc639378f/org/html_writer.go#L406-L427
	switch l.Kind() {
	case "image":
		if l.Description == nil {
			imageSrc := getMediaURL(link)
			fmt.Fprintf(r, `<img src="%s" alt="%s" />`, imageSrc, link)
		} else {
			description := strings.TrimPrefix(org.String(l.Description...), "file:")
			imageSrc := getMediaURL([]byte(description))
			fmt.Fprintf(r, `<a href="%s"><img src="%s" alt="%s" /></a>`, link, imageSrc, imageSrc)
		}
	case "video":
		if l.Description == nil {
			imageSrc := getMediaURL(link)
			fmt.Fprintf(r, `<video src="%s">%s</video>`, imageSrc, link)
		} else {
			description := strings.TrimPrefix(org.String(l.Description...), "file:")
			videoSrc := getMediaURL([]byte(description))
			fmt.Fprintf(r, `<a href="%s"><video src="%s">%s</video></a>`, link, videoSrc, videoSrc)
		}
	default:
		description := string(link)
		if l.Description != nil {
			description = r.WriteNodesAsString(l.Description...)
		}
		fmt.Fprintf(r, `<a href="%s">%s</a>`, link, description)
	}
}

func getMediaURL(l []byte) string {
	srcURL := string(l)

	// Check if link is valid
	if len(srcURL) > 0 && !markup.IsLink(l) {
		srcURL = strings.Replace(srcURL, "/src/", "/media/", 1)
	}

	return srcURL
}
