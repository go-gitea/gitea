// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rst

import (
	"bufio"
	"bytes"

	"code.gitea.io/gitea/modules/markup"

	gorst "github.com/hhatto/gorst"
)

func init() {
	markup.RegisterParser(Parser{})
}

// Parser implements markup.Parser for reStructuredText
type Parser struct {
}

// Name implements markup.Parser
func (Parser) Name() string {
	return "reStructuredText"
}

// Extensions implements markup.Parser
func (Parser) Extensions() []string {
	return []string{".rst"}
}

// Render renders reStructuredText rawbytes to HTML
func Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	p := gorst.NewParser(nil)
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	p.ReStructuredText(bytes.NewReader(rawBytes), gorst.ToHTML(w))
	w.Flush()

	return b.Bytes()
}

// RenderString reners reStructuredText string to HTML string
func RenderString(rawContent string, urlPrefix string, metas map[string]string, isWiki bool) string {
	return string(Render([]byte(rawContent), urlPrefix, metas, isWiki))
}

// Render implements markup.Parser
func (Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	return Render(rawBytes, urlPrefix, metas, isWiki)
}
