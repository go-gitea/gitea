// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"fmt"
	"html/template"
	"io"
	"strings"

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
	pc.Set(renderContextKey, ctx)
	return pc
}

type GlodmarkRender struct {
	ctx *markup.RenderContext

	goldmarkMarkdown goldmark.Markdown
}

func (r *GlodmarkRender) Convert(source []byte, writer io.Writer, opts ...parser.ParseOption) error {
	return r.goldmarkMarkdown.Convert(source, writer, opts...)
}

func (r *GlodmarkRender) Renderer() renderer.Renderer {
	return r.goldmarkMarkdown.Renderer()
}

func (r *GlodmarkRender) highlightingRenderer(w util.BufWriter, c highlighting.CodeBlockContext, entering bool) {
	if entering {
		languageBytes, _ := c.Language()
		languageStr := giteautil.IfZero(string(languageBytes), "text")

		preClasses := "code-block"
		if languageStr == "mermaid" || languageStr == "math" {
			preClasses += " is-loading"
		}

		err := r.ctx.RenderInternal.FormatWithSafeAttrs(w, `<pre class="%s">`, preClasses)
		if err != nil {
			return
		}

		// include language-x class as part of commonmark spec, "chroma" class is used to highlight the code
		// the "display" class is used by "js/markup/math.ts" to render the code element as a block
		// the "math.ts" strictly depends on the structure: <pre class="code-block is-loading"><code class="language-math display">...</code></pre>
		err = r.ctx.RenderInternal.FormatWithSafeAttrs(w, `<code class="chroma language-%s display">`, languageStr)
		if err != nil {
			return
		}
	} else {
		_, err := w.WriteString("</code></pre>")
		if err != nil {
			return
		}
	}
}

// SpecializedMarkdown sets up the Gitea specific markdown extensions
func SpecializedMarkdown(ctx *markup.RenderContext) *GlodmarkRender {
	// TODO: it could use a pool to cache the renderers to reuse them with different contexts
	// at the moment it is fast enough (see the benchmarks)
	r := &GlodmarkRender{ctx: ctx}
	r.goldmarkMarkdown = goldmark.New(
		goldmark.WithExtensions(
			extension.NewTable(extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute)),
			extension.Strikethrough,
			extension.TaskList,
			extension.DefinitionList,
			common.FootnoteExtension,
			highlighting.NewHighlighting(
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(true),
					chromahtml.PreventSurroundingPre(true),
				),
				highlighting.WithWrapperRenderer(r.highlightingRenderer),
			),
			math.NewExtension(&ctx.RenderInternal, math.Options{
				Enabled:           setting.Markdown.EnableMath,
				ParseDollarInline: true,
				ParseDollarBlock:  true,
				ParseSquareBlock:  true, // TODO: this is a bad syntax "\[ ... \]", it conflicts with normal markdown escaping, it should be deprecated in the future (by some config options)
				// ParseBracketInline: true, // TODO: this is also a bad syntax "\( ... \)", it also conflicts, it should be deprecated in the future
			}),
			meta.Meta,
		),
		goldmark.WithParserOptions(
			parser.WithAttribute(),
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(util.Prioritized(NewASTTransformer(&ctx.RenderInternal), 10000)),
		),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	// Override the original Tasklist renderer!
	r.goldmarkMarkdown.Renderer().AddOptions(
		renderer.WithNodeRenderers(util.Prioritized(NewHTMLRenderer(&ctx.RenderInternal), 10)),
	)

	return r
}

// render calls goldmark render to convert Markdown to HTML
// NOTE: The output of this method MUST get sanitized separately!!!
func render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	converter := SpecializedMarkdown(ctx)
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
		if (!setting.IsProd && !setting.IsInTesting) || log.IsDebug() {
			log.Error("Panic in markdown: %v\n%s", err, log.Stack(2))
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

	// Preserve original length.
	bufWithMetadataLength := len(buf)

	rc := &RenderConfig{
		Meta: markup.RenderMetaAsDetails,
		Icon: "table",
		Lang: "",
	}
	buf, _ = ExtractMetadataBytes(buf, rc)

	metaLength := bufWithMetadataLength - len(buf)
	if metaLength < 0 {
		metaLength = 0
	}
	rc.metaLength = metaLength

	pc.Set(renderConfigKey, rc)

	if err := converter.Convert(buf, lw, parser.WithContext(pc)); err != nil {
		log.Error("Unable to render: %v", err)
		return err
	}

	return nil
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
	ctx.RenderOptions.MarkupType = MarkupName
	return markup.Render(ctx, input, output)
}

// RenderString renders Markdown string to HTML with all specific handling stuff and return string
func RenderString(ctx *markup.RenderContext, content string) (template.HTML, error) {
	var buf strings.Builder
	if err := Render(ctx, strings.NewReader(content), &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
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
