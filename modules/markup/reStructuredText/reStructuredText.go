// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rst

import (
	"bufio"
	"bytes"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"

	gorst "github.com/hhatto/gorst"
)

func init() {
	markup.RegisterParser(Parser{})
}

// Parser implements markup.Parser for reStructuredText
type Parser struct {
}

// Name return the parser's name
func (Parser) Name() string {
	return "reStructuredText"
}

// Extensions return the parser supported extensions
func (Parser) Extensions() []string {
	return []string{".rst"}
}

// Render renders reStructuredText bytes to HTML
func Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	p := gorst.NewParser(nil)
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	p.ReStructuredText(bytes.NewReader(rawBytes), gorst.ToHTML(w))
	if err := w.Flush(); err != nil {
		log.Error(4, "Render ReStructuredText failed: %v", err)
		return []byte("")
	}

	return b.Bytes()
}

// RenderString renders reStructuredText string to HTML
func RenderString(rawContent string, urlPrefix string, metas map[string]string, isWiki bool) string {
	return string(Render([]byte(rawContent), urlPrefix, metas, isWiki))
}

// Render renders reStructuredText bytes to HTML, for implementations of markup.Parser
func (Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	return Render(rawBytes, urlPrefix, metas, isWiki)
}
