// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"bufio"
	"bytes"
	"html"
	"io"
	"regexp"
	"strconv"

	"code.gitea.io/gitea/modules/csv"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
)

func init() {
	markup.RegisterRenderer(Renderer{})
}

// Renderer implements markup.Renderer for csv files
type Renderer struct{}

// Name implements markup.Renderer
func (Renderer) Name() string {
	return "csv"
}

// Extensions implements markup.Renderer
func (Renderer) Extensions() []string {
	return []string{".csv", ".tsv"}
}

// SanitizerRules implements markup.Renderer
func (Renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return []setting.MarkupSanitizerRule{
		{Element: "table", AllowAttr: "class", Regexp: regexp.MustCompile(`data-table`)},
		{Element: "th", AllowAttr: "class", Regexp: regexp.MustCompile(`line-num`)},
		{Element: "td", AllowAttr: "class", Regexp: regexp.MustCompile(`line-num`)},
	}
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

// Render implements markup.Renderer
func (r Renderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	tmpBlock := bufio.NewWriter(output)
	maxSize := setting.UI.CSV.MaxFileSize

	if maxSize == 0 {
		return r.tableRender(ctx, input, tmpBlock)
	}

	rawBytes, err := io.ReadAll(io.LimitReader(input, maxSize+1))
	if err != nil {
		return err
	}

	if int64(len(rawBytes)) <= maxSize {
		return r.tableRender(ctx, bytes.NewReader(rawBytes), tmpBlock)
	}
	return r.fallbackRender(io.MultiReader(bytes.NewReader(rawBytes), input), tmpBlock)
}

func (Renderer) fallbackRender(input io.Reader, tmpBlock *bufio.Writer) error {
	_, err := tmpBlock.WriteString("<pre>")
	if err != nil {
		return err
	}

	scan := bufio.NewScanner(input)
	scan.Split(bufio.ScanRunes)
	for scan.Scan() {
		switch scan.Text() {
		case `&`:
			_, err = tmpBlock.WriteString("&amp;")
		case `'`:
			_, err = tmpBlock.WriteString("&#39;") // "&#39;" is shorter than "&apos;" and apos was not in HTML until HTML5.
		case `<`:
			_, err = tmpBlock.WriteString("&lt;")
		case `>`:
			_, err = tmpBlock.WriteString("&gt;")
		case `"`:
			_, err = tmpBlock.WriteString("&#34;") // "&#34;" is shorter than "&quot;".
		default:
			_, err = tmpBlock.Write(scan.Bytes())
		}
		if err != nil {
			return err
		}
	}

	_, err = tmpBlock.WriteString("</pre>")
	if err != nil {
		return err
	}
	return tmpBlock.Flush()
}

func (Renderer) tableRender(ctx *markup.RenderContext, input io.Reader, tmpBlock *bufio.Writer) error {
	rd, err := csv.CreateReaderAndDetermineDelimiter(ctx, input)
	if err != nil {
		return err
	}

	if _, err := tmpBlock.WriteString(`<table class="data-table">`); err != nil {
		return err
	}
	row := 1
	for {
		fields, err := rd.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if _, err := tmpBlock.WriteString("<tr>"); err != nil {
			return err
		}
		element := "td"
		if row == 1 {
			element = "th"
		}
		if err := writeField(tmpBlock, element, "line-num", strconv.Itoa(row)); err != nil {
			return err
		}
		for _, field := range fields {
			if err := writeField(tmpBlock, element, "", field); err != nil {
				return err
			}
		}
		if _, err := tmpBlock.WriteString("</tr>"); err != nil {
			return err
		}

		row++
	}
	if _, err = tmpBlock.WriteString("</table>"); err != nil {
		return err
	}
	return tmpBlock.Flush()
}
