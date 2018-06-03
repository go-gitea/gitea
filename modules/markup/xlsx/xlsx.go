// Copyright 20178 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package xlsx

import (
	"bytes"
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"

	"github.com/360EntSecGroup-Skylar/excelize"
)

func init() {
	markup.RegisterParser(Parser{})
}

// Parser implements markup.Parser for orgmode
type Parser struct {
}

// Name implements markup.Parser
func (Parser) Name() string {
	return "Excel(.xlsx)"
}

// Extensions implements markup.Parser
func (Parser) Extensions() []string {
	return []string{".xlsx"}
}

// Render implements markup.Parser
func (Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	rd, err := excelize.OpenReader(bytes.NewReader(rawBytes))
	if err != nil {
		return []byte{}
	}
	var sheetMap = rd.GetSheetMap()
	if len(sheetMap) == 0 {
		return []byte{}
	}

	var tmpBlock bytes.Buffer
	tmpBlock.WriteString(`<div class="ui top attached tabular menu">`)
	for i := 0; i < len(sheetMap); i++ {
		var active string
		if i == 0 {
			active = "active"
		}
		tmpBlock.WriteString(fmt.Sprintf(`<a class="%s item" data-tab="%d">%s</a>`, active, i, sheetMap[i+1]))
	}
	tmpBlock.WriteString(`</div>`)

	for i := 0; i < len(sheetMap); i++ {
		var active string
		if i == 0 {
			active = "active"
		}
		tmpBlock.WriteString(fmt.Sprintf(`<div data-tab="%d" class="ui bottom attached `+
			active+` tab segment"><table class="table">`, i))
		rows, err := rd.Rows(sheetMap[i+1])
		if err != nil {
			log.Error(1, "Rows: %v", err)
			tmpBlock.WriteString("</table></div>")
			continue
		}
		for rows.Next() {
			fields := rows.Columns()

			tmpBlock.WriteString("<tr>")
			for _, field := range fields {
				tmpBlock.WriteString("<td>")
				tmpBlock.WriteString(field)
				tmpBlock.WriteString("</td>")
			}
			tmpBlock.WriteString("<tr>")
		}
		tmpBlock.WriteString("</table></div>")
	}
	return tmpBlock.Bytes()
}
