// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bytes"
	"html"
	"io"
	"strconv"

	"code.gitea.io/gitea/modules/csv"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
)

func init() {
	markup.RegisterParser(Parser{})
}

// Parser implements markup.Parser for csv files
type Parser struct {
}

// Name implements markup.Parser
func (Parser) Name() string {
	return "csv"
}

// Extensions implements markup.Parser
func (Parser) Extensions() []string {
	return []string{".csv", ".tsv"}
}

// Render implements markup.Parser
func (Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	var tmpBlock bytes.Buffer

	if setting.UI.CSV.MaxFileSize != 0 && setting.UI.CSV.MaxFileSize < int64(len(rawBytes)) {
		tmpBlock.WriteString("<pre>")
		tmpBlock.WriteString(html.EscapeString(string(rawBytes)))
		tmpBlock.WriteString("</pre>")
		return tmpBlock.Bytes()
	}

	rd := csv.CreateReaderAndGuessDelimiter(rawBytes)

	writeField := func(element, class, field string) {
		tmpBlock.WriteString("<")
		tmpBlock.WriteString(element)
		if len(class) > 0 {
			tmpBlock.WriteString(" class=\"")
			tmpBlock.WriteString(class)
			tmpBlock.WriteString("\"")
		}
		tmpBlock.WriteString(">")
		tmpBlock.WriteString(html.EscapeString(field))
		tmpBlock.WriteString("</")
		tmpBlock.WriteString(element)
		tmpBlock.WriteString(">")
	}

	tmpBlock.WriteString(`<table class="data-table">`)
	row := 1
	for {
		fields, err := rd.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		tmpBlock.WriteString("<tr>")
		element := "td"
		if row == 1 {
			element = "th"
		}
		writeField(element, "line-num", strconv.Itoa(row))
		for _, field := range fields {
			writeField(element, "", field)
		}
		tmpBlock.WriteString("</tr>")

		row++
	}
	tmpBlock.WriteString("</table>")

	return tmpBlock.Bytes()
}
