// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"

	"github.com/chaseadamsio/goorgeous"
	"github.com/russross/blackfriday"
)

func init() {
	markup.RegisterParser(Parser{})
}

// Parser implements markup.Parser for orgmode
type Parser struct {
}

// Name implements markup.Parser
func (Parser) Name() string {
	return "orgmode"
}

// Extensions implements markup.Parser
func (Parser) Extensions() []string {
	return []string{".org"}
}

// Render renders orgmode rawbytes to HTML
func Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) (result []byte) {
	defer func() {
		if err := recover(); err != nil {
			log.Error(4, "Panic in orgmode.Render: %v Just returning the rawBytes", err)
			result = rawBytes
		}
	}()
	htmlFlags := blackfriday.HTML_USE_XHTML
	htmlFlags |= blackfriday.HTML_SKIP_STYLE
	htmlFlags |= blackfriday.HTML_OMIT_CONTENTS
	renderer := &markdown.Renderer{
		Renderer:  blackfriday.HtmlRenderer(htmlFlags, "", ""),
		URLPrefix: urlPrefix,
		IsWiki:    isWiki,
	}
	result = goorgeous.Org(rawBytes, renderer)
	return
}

// RenderString reners orgmode string to HTML string
func RenderString(rawContent string, urlPrefix string, metas map[string]string, isWiki bool) string {
	return string(Render([]byte(rawContent), urlPrefix, metas, isWiki))
}

// Render implements markup.Parser
func (Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	return Render(rawBytes, urlPrefix, metas, isWiki)
}
