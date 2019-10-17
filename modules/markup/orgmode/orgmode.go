// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bytes"
	"fmt"
	"html"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"

	"github.com/niklasfasching/go-org/org"
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
func Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	// 	defer func() {
	// 		if err := recover(); err != nil {
	// 			log.Error("Panic in orgmode.Render: %v Just returning the rawBytes", err)
	// 			result = rawBytes
	// 		}
	// 	}()
	// 	htmlFlags := blackfriday.HTML_USE_XHTML
	// 	htmlFlags |= blackfriday.HTML_SKIP_STYLE
	// 	htmlFlags |= blackfriday.HTML_OMIT_CONTENTS
	// 	renderer := &markdown.Renderer{
	// 		Renderer:  blackfriday.HtmlRenderer(htmlFlags, "", ""),
	// 		URLPrefix: urlPrefix,
	// 		IsWiki:    isWiki,
	// 	}
	// 	result = goorgeous.Org(rawBytes, renderer)
	// 	return
	renderer := &Renderer{
		HTMLWriter: org.NewHTMLWriter(),
		URLPrefix:  urlPrefix,
		IsWiki:     isWiki,
	}
	res, err := org.New().Silent().Parse(bytes.NewReader(rawBytes), "").Write(renderer)
	if err != nil {
		log.Error("Panic in orgmode.Render: %v Just returning the rawBytes", err)
		//result = rawBytes
		return rawBytes
	}
	//result = []byte(res)
	return []byte(res)
}

// RenderString reners orgmode string to HTML string
func RenderString(rawContent string, urlPrefix string, metas map[string]string, isWiki bool) string {
	return string(Render([]byte(rawContent), urlPrefix, metas, isWiki))
}

// Render implements markup.Parser
func (Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	return Render(rawBytes, urlPrefix, metas, isWiki)
}

type Renderer struct {
	*org.HTMLWriter
	URLPrefix string
	IsWiki    bool
}

func (r *Renderer) WriteRegularLink(l org.RegularLink) {
	url := html.EscapeString(l.URL)
	if l.Protocol == "file" {
		url = url[len("file:"):]
	}
	description := url
	if l.Description != nil {
		description = r.nodesAsString(l.Description...)
	}
	switch l.Kind() {
	case "image":
		r.WriteString(fmt.Sprintf(`<img src="%s" alt="%s" title="%s" />`, url, description, description))
	case "video":
		r.WriteString(fmt.Sprintf(`<video src="%s" title="%s">%s</video>`, url, description, description))
	default:
		r.WriteString(fmt.Sprintf(`<a href="%s">%s</a>`, url, description))
	}
}

func (r *Renderer) emptyClone() *Renderer {
	wcopy := *r
	wcopy.Builder = strings.Builder{}
	return &wcopy
}

func (r *Renderer) nodesAsString(nodes ...org.Node) string {
	tmp := r.emptyClone()
	org.WriteNodes(tmp, nodes...)
	return tmp.String()
}
