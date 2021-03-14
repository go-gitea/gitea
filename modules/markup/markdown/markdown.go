// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"io"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/common"
	"code.gitea.io/gitea/modules/setting"
	giteautil "code.gitea.io/gitea/modules/util"

	chromahtml "github.com/alecthomas/chroma/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

var converter goldmark.Markdown
var once = sync.Once{}

var urlPrefixKey = parser.NewContextKey()
var isWikiKey = parser.NewContextKey()
var renderMetasKey = parser.NewContextKey()

// NewGiteaParseContext creates a parser.Context with the gitea context set
func NewGiteaParseContext(urlPrefix string, metas map[string]string, isWiki bool) parser.Context {
	pc := parser.NewContext(parser.WithIDs(newPrefixedIDs()))
	pc.Set(urlPrefixKey, urlPrefix)
	pc.Set(isWikiKey, isWiki)
	pc.Set(renderMetasKey, metas)
	return pc
}

// actualRender renders Markdown to HTML without handling special links.
func actualRender(body []byte, urlPrefix string, metas map[string]string, wikiMarkdown bool) []byte {
	once.Do(func() {
		converter = goldmark.New(
			goldmark.WithExtensions(extension.Table,
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

							preClasses := []string{}
							if languageStr == "mermaid" {
								preClasses = append(preClasses, "is-loading")
							}

							if len(preClasses) > 0 {
								_, err := w.WriteString(`<pre class="` + strings.Join(preClasses, " ") + `">`)
								if err != nil {
									return
								}
							} else {
								_, err := w.WriteString(`<pre>`)
								if err != nil {
									return
								}
							}

							// include language-x class as part of commonmark spec
							_, err := w.WriteString(`<code class="chroma language-` + string(language) + `">`)
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
		converter.Renderer().AddOptions(
			renderer.WithNodeRenderers(
				util.Prioritized(NewHTMLRenderer(), 10),
			),
		)

	})

	pc := NewGiteaParseContext(urlPrefix, metas, wikiMarkdown)

	rd, wr := io.Pipe()
	defer func() {
		_ = rd.Close()
		_ = wr.Close()
	}()

	// FIXME: should we include a timeout that closes the pipe to abort the parser and sanitizer if it takes too long?
	go func() {
		if err := converter.Convert(giteautil.NormalizeEOL(body), wr, parser.WithContext(pc)); err != nil {
			log.Error("Unable to render: %v", err)
			_ = wr.CloseWithError(err)
			return
		}
		_ = wr.Close()
	}()
	return markup.SanitizeReader(rd).Bytes()
}

func render(body []byte, urlPrefix string, metas map[string]string, wikiMarkdown bool) (ret []byte) {
	defer func() {
		err := recover()
		if err == nil {
			return
		}

		log.Warn("Unable to render markdown due to panic in goldmark - will return sanitized raw bytes")
		if log.IsDebug() {
			log.Debug("Panic in markdown: %v\n%s", err, string(log.Stack(2)))
		}
		ret = markup.SanitizeBytes(body)
	}()
	return actualRender(body, urlPrefix, metas, wikiMarkdown)
}

var (
	// MarkupName describes markup's name
	MarkupName = "markdown"
)

func init() {
	markup.RegisterParser(Parser{})
}

// Parser implements markup.Parser
type Parser struct{}

// Name implements markup.Parser
func (Parser) Name() string {
	return MarkupName
}

// Extensions implements markup.Parser
func (Parser) Extensions() []string {
	return setting.Markdown.FileExtensions
}

// Render implements markup.Parser
func (Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	return render(rawBytes, urlPrefix, metas, isWiki)
}

// Render renders Markdown to HTML with all specific handling stuff.
func Render(rawBytes []byte, urlPrefix string, metas map[string]string) []byte {
	return markup.Render("a.md", rawBytes, urlPrefix, metas)
}

// RenderRaw renders Markdown to HTML without handling special links.
func RenderRaw(body []byte, urlPrefix string, wikiMarkdown bool) []byte {
	return render(body, urlPrefix, map[string]string{}, wikiMarkdown)
}

// RenderString renders Markdown to HTML with special links and returns string type.
func RenderString(raw, urlPrefix string, metas map[string]string) string {
	return markup.RenderString("a.md", raw, urlPrefix, metas)
}

// RenderWiki renders markdown wiki page to HTML and return HTML string
func RenderWiki(rawBytes []byte, urlPrefix string, metas map[string]string) string {
	return markup.RenderWiki("a.md", rawBytes, urlPrefix, metas)
}

// IsMarkdownFile reports whether name looks like a Markdown file
// based on its extension.
func IsMarkdownFile(name string) bool {
	return markup.IsMarkupFile(name, MarkupName)
}
