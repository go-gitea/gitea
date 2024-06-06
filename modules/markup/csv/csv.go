// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"bufio"
	"html"
	"io"
	"regexp"
	"strconv"

	"code.gitea.io/gitea/modules/csv"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"
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
	maxRows := setting.UI.CSV.MaxRows

	if maxSize != 0 {
		input = io.LimitReader(input, maxSize+1)
	}

	rd, err := csv.CreateReaderAndDetermineDelimiter(ctx, input)
	if err != nil {
		return err
	}
	if _, err := tmpBlock.WriteString(`<table class="data-table">`); err != nil {
		return err
	}

	row := 0
	for {
		fields, err := rd.Read()
		if err == io.EOF || (row >= maxRows && maxRows != 0) {
			break
		}
		if err != nil {
			continue
		}

		if _, err := tmpBlock.WriteString("<tr>"); err != nil {
			return err
		}
		element := "td"
		if row == 0 {
			element = "th"
		}
		if err := writeField(tmpBlock, element, "line-num", strconv.Itoa(row+1)); err != nil {
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

	// Check if maxRows or maxSize is reached, and if true, warn.
	if (row >= maxRows && maxRows != 0) || (rd.InputOffset() >= maxSize && maxSize != 0) {
		warn := `<table class="data-table"><tr><td>`
		rawLink := ` <a href="` + ctx.Links.RawLink() + `/` + util.PathEscapeSegments(ctx.RelativePath) + `">`

		// Try to get the user translation
		if locale, ok := ctx.Ctx.Value(translation.ContextKey).(translation.Locale); ok {
			warn += locale.TrString("repo.file_too_large")
			rawLink += locale.TrString("repo.file_view_raw")
		} else {
			warn += "The file is too large to be shown."
			rawLink += "View Raw"
		}

		warn += rawLink + `</a></td></tr></table>`

		// Write the HTML string to the output
		if _, err := tmpBlock.WriteString(warn); err != nil {
			return err
		}
	}

	return tmpBlock.Flush()
}
