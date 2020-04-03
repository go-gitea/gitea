// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"sync"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/common"
	"code.gitea.io/gitea/modules/setting"
	giteautil "code.gitea.io/gitea/modules/util"

	"github.com/yuin/goldmark"
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

// NewGiteaParseContext creates a parser.Context with the gitea context set
func NewGiteaParseContext(urlPrefix string, isWiki bool) parser.Context {
	pc := parser.NewContext(parser.WithIDs(newPrefixedIDs()))
	pc.Set(urlPrefixKey, urlPrefix)
	pc.Set(isWikiKey, isWiki)
	return pc
}

// RenderRaw renders Markdown to HTML without handling special links.
func RenderRaw(body []byte, urlPrefix string, wikiMarkdown bool) []byte {
	once.Do(func() {
		converter = goldmark.New(
			goldmark.WithExtensions(extension.Table,
				extension.Strikethrough,
				extension.TaskList,
				extension.DefinitionList,
				common.FootnoteExtension,
				extension.NewTypographer(
					extension.WithTypographicSubstitutions(extension.TypographicSubstitutions{
						extension.EnDash:   nil,
						extension.EmDash:   nil,
						extension.Ellipsis: nil,
					}),
				),
			),
			goldmark.WithParserOptions(
				parser.WithAttribute(),
				parser.WithAutoHeadingID(),
				parser.WithASTTransformers(
					util.Prioritized(&GiteaASTTransformer{}, 10000),
				),
			),
			goldmark.WithRendererOptions(
				html.WithUnsafe(),
			),
		)

		// Override the original Tasklist renderer!
		converter.Renderer().AddOptions(
			renderer.WithNodeRenderers(
				util.Prioritized(NewTaskCheckBoxHTMLRenderer(), 1000),
			),
		)

		if setting.Markdown.EnableHardLineBreak {
			converter.Renderer().AddOptions(html.WithHardWraps())
		}
	})

	pc := NewGiteaParseContext(urlPrefix, wikiMarkdown)
	var buf bytes.Buffer
	if err := converter.Convert(giteautil.NormalizeEOL(body), &buf, parser.WithContext(pc)); err != nil {
		log.Error("Unable to render: %v", err)
	}

	return markup.SanitizeReader(&buf).Bytes()
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
	return RenderRaw(rawBytes, urlPrefix, isWiki)
}

// Render renders Markdown to HTML with all specific handling stuff.
func Render(rawBytes []byte, urlPrefix string, metas map[string]string) []byte {
	return markup.Render("a.md", rawBytes, urlPrefix, metas)
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
