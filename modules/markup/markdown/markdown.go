// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/common"
	"code.gitea.io/gitea/modules/markup/markdown/math"
	"code.gitea.io/gitea/modules/setting"
	giteautil "code.gitea.io/gitea/modules/util"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

var (
	specMarkdown     goldmark.Markdown
	specMarkdownOnce sync.Once
)

var (
	urlPrefixKey     = parser.NewContextKey()
	isWikiKey        = parser.NewContextKey()
	renderMetasKey   = parser.NewContextKey()
	renderContextKey = parser.NewContextKey()
	renderConfigKey  = parser.NewContextKey()
)

type limitWriter struct {
	w     io.Writer
	sum   int64
	limit int64
}

// Write implements the standard Write interface:
func (l *limitWriter) Write(data []byte) (int, error) {
	leftToWrite := l.limit - l.sum
	if leftToWrite < int64(len(data)) {
		n, err := l.w.Write(data[:leftToWrite])
		l.sum += int64(n)
		if err != nil {
			return n, err
		}
		return n, fmt.Errorf("rendered content too large - truncating render")
	}
	n, err := l.w.Write(data)
	l.sum += int64(n)
	return n, err
}

// newParserContext creates a parser.Context with the render context set
func newParserContext(ctx *markup.RenderContext) parser.Context {
	pc := parser.NewContext(parser.WithIDs(newPrefixedIDs()))
	pc.Set(urlPrefixKey, ctx.URLPrefix)
	pc.Set(isWikiKey, ctx.IsWiki)
	pc.Set(renderMetasKey, ctx.Metas)
	pc.Set(renderContextKey, ctx)
	return pc
}

// SpecializedMarkdown sets up the Gitea specific markdown extensions
func SpecializedMarkdown() goldmark.Markdown {
	specMarkdownOnce.Do(func() {
		specMarkdown = goldmark.New(
			goldmark.WithExtensions(
				extension.NewTable(
					extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute)),
				extension.Strikethrough,
				extension.TaskList,
				extension.DefinitionList,
				common.FootnoteExtension,
				highlighting.NewHighlighting(
					highlighting.WithFormatOptions(
						chromahtml.WithClasses(true),
						chromahtml.PreventSurroundingPre(true),
					),
					highlighting.WithWrapperRenderer(func(w util.BufWriter, c highlighting.CodeBlockContext, entering bool) {
						if entering {
							language, _ := c.Language()
							if language == nil {
								language = []byte("text")
							}

							languageStr := string(language)

							preClasses := []string{"code-block"}
							if languageStr == "mermaid" || languageStr == "math" {
								preClasses = append(preClasses, "is-loading")
							}

							_, err := w.WriteString(`<pre class="` + strings.Join(preClasses, " ") + `">`)
							if err != nil {
								return
							}

							// include language-x class as part of commonmark spec
							_, err = w.WriteString(`<code class="chroma language-` + string(language) + `">`)
							if err != nil {
								return
							}
						} else {
							_, err := w.WriteString("</code></pre>")
							if err != nil {
								return
							}
						}
					}),
				),
				math.NewExtension(
					math.Enabled(setting.Markdown.EnableMath),
				),
				meta.Meta,
			),
			goldmark.WithParserOptions(
				parser.WithAttribute(),
				parser.WithAutoHeadingID(),
				parser.WithASTTransformers(
					util.Prioritized(&ASTTransformer{}, 10000),
				),
			),
			goldmark.WithRendererOptions(
				html.WithUnsafe(),
			),
		)

		// Override the original Tasklist renderer!
		specMarkdown.Renderer().AddOptions(
			renderer.WithNodeRenderers(
				util.Prioritized(NewHTMLRenderer(), 10),
			),
		)
	})
	return specMarkdown
}

// actualRender renders Markdown to HTML without handling special links.
func actualRender(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	converter := SpecializedMarkdown()
	lw := &limitWriter{
		w:     output,
		limit: setting.UI.MaxDisplayFileSize * 3,
	}

	// FIXME: should we include a timeout to abort the renderer if it takes too long?
	defer func() {
		err := recover()
		if err == nil {
			return
		}

		log.Warn("Unable to render markdown due to panic in goldmark: %v", err)
		if log.IsDebug() {
			log.Debug("Panic in markdown: %v\n%s", err, log.Stack(2))
		}
	}()

	// FIXME: Don't read all to memory, but goldmark doesn't support
	pc := newParserContext(ctx)
	buf, err := io.ReadAll(input)
	if err != nil {
		log.Error("Unable to ReadAll: %v", err)
		return err
	}
	buf = giteautil.NormalizeEOL(buf)

	rc := &RenderConfig{
		Meta: renderMetaModeFromString(string(ctx.RenderMetaAs)),
		Icon: "table",
		Lang: "",
	}
	buf, _ = ExtractMetadataBytes(buf, rc)

	pc.Set(renderConfigKey, rc)

	if err := converter.Convert(buf, lw, parser.WithContext(pc)); err != nil {
		log.Error("Unable to render: %v", err)
		return err
	}

	return nil
}

// Note: The output of this method must get sanitized.
func render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	defer func() {
		err := recover()
		if err == nil {
			return
		}

		log.Warn("Unable to render markdown due to panic in goldmark - will return raw bytes")
		if log.IsDebug() {
			log.Debug("Panic in markdown: %v\n%s", err, log.Stack(2))
		}
		_, err = io.Copy(output, input)
		if err != nil {
			log.Error("io.Copy failed: %v", err)
		}
	}()
	return actualRender(ctx, input, output)
}

// MarkupName describes markup's name
var MarkupName = "markdown"

func init() {
	markup.RegisterRenderer(Renderer{})
}

// Renderer implements markup.Renderer
type Renderer struct{}

var _ markup.PostProcessRenderer = (*Renderer)(nil)

// Name implements markup.Renderer
func (Renderer) Name() string {
	return MarkupName
}

// NeedPostProcess implements markup.PostProcessRenderer
func (Renderer) NeedPostProcess() bool { return true }

// Extensions implements markup.Renderer
func (Renderer) Extensions() []string {
	return setting.Markdown.FileExtensions
}

// SanitizerRules implements markup.Renderer
func (Renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return []setting.MarkupSanitizerRule{}
}

// Render implements markup.Renderer
func (Renderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	return render(ctx, input, output)
}

// Render renders Markdown to HTML with all specific handling stuff.
func Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	if ctx.Type == "" {
		ctx.Type = MarkupName
	}
	return markup.Render(ctx, input, output)
}

// RenderString renders Markdown string to HTML with all specific handling stuff and return string
func RenderString(ctx *markup.RenderContext, content string) (string, error) {
	var buf strings.Builder
	if err := Render(ctx, strings.NewReader(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderRaw renders Markdown to HTML without handling special links.
func RenderRaw(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	rd, wr := io.Pipe()
	defer func() {
		_ = rd.Close()
		_ = wr.Close()
	}()

	go func() {
		if err := render(ctx, input, wr); err != nil {
			_ = wr.CloseWithError(err)
			return
		}
		_ = wr.Close()
	}()

	return markup.SanitizeReader(rd, "", output)
}

// RenderRawString renders Markdown to HTML without handling special links and return string
func RenderRawString(ctx *markup.RenderContext, content string) (string, error) {
	var buf strings.Builder
	if err := RenderRaw(ctx, strings.NewReader(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
