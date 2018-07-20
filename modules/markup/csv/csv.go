// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bytes"
	"encoding/csv"
	"html"
	"io"

	"code.gitea.io/gitea/modules/markup"
)

func init() {
	markup.RegisterParser(Parser{})
}

// Parser implements markup.Parser for orgmode
type Parser struct {
}

// Name implements markup.Parser
func (Parser) Name() string {
	return "csv"
}

// Extensions implements markup.Parser
func (Parser) Extensions() []string {
	return []string{".csv"}
}

// Render implements markup.Parser
func (Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	rd := csv.NewReader(bytes.NewReader(rawBytes))
	var tmpBlock bytes.Buffer
	tmpBlock.WriteString(`<table class="table">`)
	for {
		fields, err := rd.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		tmpBlock.WriteString("<tr>")
		for _, field := range fields {
			tmpBlock.WriteString("<td>")
			tmpBlock.WriteString(html.EscapeString(field))
			tmpBlock.WriteString("</td>")
		}
		tmpBlock.WriteString("<tr>")
	}
	tmpBlock.WriteString("</table>")

	return tmpBlock.Bytes()
}
