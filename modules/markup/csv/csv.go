// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bufio"
	"bytes"
	"html"
	"io"
	"strconv"

	"code.gitea.io/gitea/modules/csv"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
)

func init() {
	markup.RegisterRenderer(Renderer{})

}

// Renderer implements markup.Renderer for orgmode
type Renderer struct {
}

// Name implements markup.Renderer
func (Renderer) Name() string {
	return "csv"
}

// NeedPostProcess implements markup.Renderer
func (Renderer) NeedPostProcess() bool { return false }

// Extensions implements markup.Renderer
func (Renderer) Extensions() []string {
	return []string{".csv", ".tsv"}
}

func writeField(w io.Writer, element, class, field string) error {
	if _, err := io.WriteString(w, "<"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, element); err != nil {
		return err
	}
	if len(class) > 0 {
		if _, err := io.WriteString(w, " class=\""); err != nil {
			return err
		}
		if _, err := io.WriteString(w, class); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\""); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, ">"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, html.EscapeString(field)); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "</"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, element); err != nil {
		return err
	}
	_, err := io.WriteString(w, ">")
	return err
}

// Render implements markup.Parser
func (Renderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	var tmpBlock = bufio.NewWriter(output)

	// FIXME: don't read all to memory
	rawBytes, err := io.ReadAll(input)
	if err != nil {
		return err
	}

	if setting.UI.CSV.MaxFileSize != 0 && setting.UI.CSV.MaxFileSize < int64(len(rawBytes)) {
		tmpBlock.WriteString("<pre>")
		tmpBlock.WriteString(html.EscapeString(string(rawBytes)))
		tmpBlock.WriteString("</pre>")
		return nil
	}

	rd, err := csv.CreateReaderAndGuessDelimiter(bytes.NewReader(rawBytes))
	if err != nil {
		return err
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
		writeField(tmpBlock, element, "line-num", strconv.Itoa(row))
		for _, field := range fields {
			writeField(tmpBlock, element, "", field)
		}
		tmpBlock.WriteString("</tr>")

		row++
	}
	if _, err = tmpBlock.WriteString("</table>"); err != nil {
		return err
	}
	return tmpBlock.Flush()
}
