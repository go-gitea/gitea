// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jupyter

import (
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"strings"

	"gitea.dev/modules/highlight"
	"gitea.dev/modules/htmlutil"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/setting"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

func init() {
	markup.RegisterRenderer(renderer{})
}

// Renderer implements markup.Renderer for Jupyter notebooks
type renderer struct{}

var (
	_ markup.Renderer            = (*renderer)(nil)
	_ markup.PostProcessRenderer = (*renderer)(nil)
	_ markup.ExternalRenderer    = (*renderer)(nil)
)

func (renderer) Name() string {
	return "jupyter"
}

func (renderer) NeedPostProcess() bool { return true }

func (renderer) GetExternalRendererOptions() markup.ExternalRendererOptions {
	return markup.ExternalRendererOptions{
		SanitizerDisabled: false,
	}
}

func (renderer) FileNamePatterns() []string {
	return []string{"*.ipynb"}
}

func (renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return []setting.MarkupSanitizerRule{
		// Notebook container and messages
		{Element: "div", AllowAttr: "class", Regexp: `^jupyter-notebook$`},
		{Element: "div", AllowAttr: "class", Regexp: `^jupyter-notebook-message$`},
		{Element: "div", AllowAttr: "class", Regexp: `^jupyter-notebook-error$`},

		// Cell structure
		{Element: "div", AllowAttr: "class", Regexp: `^cell (markdown|code)$`},
		{Element: "div", AllowAttr: "class", Regexp: `^(input-wrapper|output-wrapper)$`},
		{Element: "div", AllowAttr: "class", Regexp: `^prompt (input-prompt|output-prompt)$`},
		{Element: "div", AllowAttr: "class", Regexp: `^(input|output)$`},
		{Element: "div", AllowAttr: "class", Regexp: `^input markup$`},

		// Output types
		{Element: "div", AllowAttr: "class", Regexp: `^jupyter-html-output$`},
		{Element: "div", AllowAttr: "class", Regexp: `^jupyter-unsupported-output$`},
		{Element: "pre", AllowAttr: "class", Regexp: `^(stream-stdout|stream-stderr|error-output)$`},

		// Code highlighting (Chroma)
		{Element: "pre"},
		{Element: "code", AllowAttr: "class", Regexp: `^chroma language-[\w-]+$`},
		{Element: "code", AllowAttr: "class", Regexp: `^language-math display$`},
		{Element: "span", AllowAttr: "class", Regexp: `^[\w-]+$`},

		// Images (base64 data URIs only)
		{Element: "img", AllowAttr: "class", Regexp: `^jupyter-output-image$`},
		{Element: "img", AllowAttr: "src", Regexp: `^data:image/(png|jpeg|svg\+xml);base64,[A-Za-z0-9+/=]+$`},

		// Tables (for DataFrames and markdown)
		{Element: "table", AllowAttr: "class", Regexp: `^dataframe$`},
		{Element: "table", AllowAttr: "border", Regexp: `^[0-9]+$`},
		{Element: "thead"},
		{Element: "tbody"},
		{Element: "tr"},
		{Element: "th"},
		{Element: "td"},

		// Markdown elements
		{Element: "h1"},
		{Element: "h2"},
		{Element: "h3"},
		{Element: "h4"},
		{Element: "h5"},
		{Element: "h6"},
		{Element: "p"},
		{Element: "a", AllowAttr: "href", Regexp: `^(https?://|mailto:).*$`},
		{Element: "strong"},
		{Element: "em"},
		{Element: "ul"},
		{Element: "ol"},
		{Element: "li"},
		{Element: "blockquote"},
		{Element: "dl"},
		{Element: "dt"},
		{Element: "dd"},
		{Element: "input", AllowAttr: "type", Regexp: `^checkbox$`},
		{Element: "input", AllowAttr: "disabled", Regexp: `^$`},
		{Element: "input", AllowAttr: "checked", Regexp: `^$`},
	}
}

// Notebook structures
type Notebook struct {
	Cells    []Cell         `json:"cells"`
	Metadata map[string]any `json:"metadata"`
	Nbformat int            `json:"nbformat"`
}

type Cell struct {
	CellType       string         `json:"cell_type"`
	Source         any            `json:"source"` // string or []string
	Outputs        []Output       `json:"outputs,omitempty"`
	ExecutionCount any            `json:"execution_count,omitempty"` // int or null
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type Output struct {
	OutputType string         `json:"output_type"`
	Data       map[string]any `json:"data,omitempty"`
	Text       any            `json:"text,omitempty"` // string or []string
	Name       string         `json:"name,omitempty"`
	Traceback  any            `json:"traceback,omitempty"` // []string
	Ename      string         `json:"ename,omitempty"`
	Evalue     string         `json:"evalue,omitempty"`
}

// Render renders Jupyter notebook to HTML
func (renderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	// Read input
	data, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("failed to read notebook: %w", err)
	}

	// Parse notebook
	var notebook Notebook
	if err := json.Unmarshal(data, &notebook); err != nil {
		return fmt.Errorf("failed to parse notebook JSON: %w", err)
	}

	// Check nbformat version
	if notebook.Nbformat < 4 {
		_, _ = htmlutil.HTMLPrintf(output,
			`<div class="jupyter-notebook-message">This notebook uses an older format (nbformat %d). Only nbformat 4+ is supported for rendering. Please upgrade the notebook in Jupyter or view the raw JSON.</div>`,
			notebook.Nbformat,
		)
		return nil
	}

	// Detect language
	language := "python" // default
	if metadata, ok := notebook.Metadata["language_info"].(map[string]any); ok {
		if name, ok := metadata["name"].(string); ok {
			language = name
		}
	} else if kernelspec, ok := notebook.Metadata["kernelspec"].(map[string]any); ok {
		if lang, ok := kernelspec["language"].(string); ok {
			language = lang
		}
	}

	// Start rendering
	_, _ = output.Write([]byte(`<div class="jupyter-notebook">`))

	executionCount := 1
	for _, cell := range notebook.Cells {
		if err := renderCell(ctx, output, cell, language, &executionCount); err != nil {
			log.Warn("Failed to render cell: %v", err)
			continue
		}
	}

	_, _ = output.Write([]byte(`</div>`))
	return nil
}

func renderCell(ctx *markup.RenderContext, output io.Writer, cell Cell, language string, executionCount *int) error {
	source := joinSource(cell.Source)

	switch cell.CellType {
	case "markdown":
		_, _ = output.Write([]byte(`<div class="cell markdown"><div class="input markup">`))
		if err := renderMarkdown(ctx, output, source); err != nil {
			return err
		}
		_, _ = output.Write([]byte(`</div></div>`))

	case "code":
		execCount := *executionCount
		if cell.ExecutionCount != nil {
			if count, ok := cell.ExecutionCount.(float64); ok {
				execCount = int(count)
			}
		}

		_, _ = output.Write([]byte(`<div class="cell code">`))
		_, _ = output.Write([]byte(`<div class="input-wrapper">`))
		_, _ = htmlutil.HTMLPrintf(output, `<div class="prompt input-prompt">In [%d]:</div>`, execCount)
		_, _ = output.Write([]byte(`<div class="input">`))

		// Highlight code
		lexer := highlight.DetectChromaLexerByFileName("", language)
		sb := &strings.Builder{}
		_, _ = sb.WriteString(`<pre><code class="chroma language-`)
		_, _ = sb.WriteString(strings.ToLower(language))
		_, _ = sb.WriteString(`">`)
		_, _ = sb.WriteString(string(highlight.RenderCodeByLexer(lexer, source)))
		_, _ = sb.WriteString("</code></pre>")
		_, _ = output.Write([]byte(sb.String()))

		_, _ = output.Write([]byte(`</div></div>`))

		// Render outputs
		if len(cell.Outputs) > 0 {
			hasExecutionResult := false
			for _, out := range cell.Outputs {
				if out.OutputType == "execute_result" {
					hasExecutionResult = true
					break
				}
			}

			_, _ = output.Write([]byte(`<div class="output-wrapper">`))
			if hasExecutionResult {
				_, _ = htmlutil.HTMLPrintf(output, `<div class="prompt output-prompt">Out[%d]:</div>`, execCount)
			} else {
				_, _ = output.Write([]byte(`<div class="prompt output-prompt"></div>`))
			}

			_, _ = output.Write([]byte(`<div class="output">`))
			for _, out := range cell.Outputs {
				renderOutput(output, out)
			}
			_, _ = output.Write([]byte(`</div></div>`))
		}

		_, _ = output.Write([]byte(`</div>`))
		*executionCount++
	}

	return nil
}

func renderMarkdown(_ *markup.RenderContext, output io.Writer, source string) error {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.Table,
			extension.Strikethrough,
			extension.Linkify,
			extension.TaskList,
			extension.DefinitionList,
		),
	)
	return md.Convert([]byte(source), output)
}

func renderOutput(output io.Writer, out Output) {
	if out.Data != nil {
		// Image outputs
		if pngData, ok := out.Data["image/png"]; ok {
			imgData := joinSource(pngData)
			_, _ = htmlutil.HTMLPrintf(output, `<img src="data:image/png;base64,%s" class="jupyter-output-image">`, imgData)
			return
		}
		if jpegData, ok := out.Data["image/jpeg"]; ok {
			imgData := joinSource(jpegData)
			_, _ = htmlutil.HTMLPrintf(output, `<img src="data:image/jpeg;base64,%s" class="jupyter-output-image">`, imgData)
			return
		}
		if svgData, ok := out.Data["image/svg+xml"]; ok {
			// Encode SVG as base64 data URI for safety
			svgContent := joinSource(svgData)
			svgBase64 := base64.StdEncoding.EncodeToString([]byte(svgContent))
			_, _ = htmlutil.HTMLPrintf(output, `<img src="data:image/svg+xml;base64,%s" class="jupyter-output-image">`, svgBase64)
			return
		}

		// HTML output
		if htmlData, ok := out.Data["text/html"]; ok {
			htmlContent := joinSource(htmlData)
			// Strip <style> tags as we handle DataFrame styles in CSS
			htmlContent = stripStyleTags(htmlContent)
			// Write raw HTML - sanitizer will clean it
			_, _ = output.Write([]byte(`<div class="jupyter-html-output">`))
			_, _ = output.Write([]byte(htmlContent))
			_, _ = output.Write([]byte(`</div>`))
			return
		}

		// LaTeX output
		if latexData, ok := out.Data["text/latex"]; ok {
			latex := joinSource(latexData)
			latex = strings.TrimPrefix(latex, "$$")
			latex = strings.TrimSuffix(latex, "$$")
			_, _ = htmlutil.HTMLPrintf(output, `<pre><code class="language-math display">%s</code></pre>`, html.EscapeString(latex))
			return
		}

		// Plain text output
		if plainData, ok := out.Data["text/plain"]; ok {
			_, _ = htmlutil.HTMLPrintf(output, `<pre>%s</pre>`, html.EscapeString(joinSource(plainData)))
			return
		}

		// Unsupported outputs
		if _, ok := out.Data["application/javascript"]; ok {
			_, _ = output.Write([]byte(`<div class="jupyter-unsupported-output">[JavaScript output - execution disabled for security]</div>`))
			return
		}
		if _, ok := out.Data["application/vnd.plotly.v1+json"]; ok {
			_, _ = output.Write([]byte(`<div class="jupyter-unsupported-output">[Plotly output - interactive plots not supported]</div>`))
			return
		}
		if _, ok := out.Data["application/vnd.jupyter.widget-view+json"]; ok {
			_, _ = output.Write([]byte(`<div class="jupyter-unsupported-output">[Jupyter widget - interactive widgets not supported]</div>`))
			return
		}
	}

	// Stream output
	if out.OutputType == "stream" && out.Text != nil {
		_, _ = htmlutil.HTMLPrintf(output, `<pre class="stream-%s">%s</pre>`, out.Name, html.EscapeString(joinSource(out.Text)))
		return
	}

	// Error output
	if out.OutputType == "error" {
		traceback := ""
		if out.Traceback != nil {
			if tb, ok := out.Traceback.([]any); ok {
				lines := make([]string, len(tb))
				for i, line := range tb {
					lines[i] = fmt.Sprint(line)
				}
				traceback = strings.Join(lines, "\n")
			}
		}
		if traceback == "" && out.Ename != "" {
			traceback = fmt.Sprintf("%s: %s", out.Ename, out.Evalue)
		}
		_, _ = htmlutil.HTMLPrintf(output, `<pre class="error-output">%s</pre>`, html.EscapeString(traceback))
		return
	}

	// Generic text output
	if out.Text != nil {
		_, _ = htmlutil.HTMLPrintf(output, `<pre>%s</pre>`, html.EscapeString(joinSource(out.Text)))
	}
}

func joinSource(source any) string {
	switch v := source.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, len(v))
		for i, part := range v {
			parts[i] = fmt.Sprint(part)
		}
		return strings.Join(parts, "")
	default:
		return fmt.Sprint(v)
	}
}

func stripStyleTags(html string) string {
	// Remove <style>...</style> tags (including scoped attribute)
	start := strings.Index(html, "<style")
	for start != -1 {
		end := strings.Index(html[start:], "</style>")
		if end == -1 {
			break
		}
		html = html[:start] + html[start+end+8:]
		start = strings.Index(html, "<style")
	}
	return html
}
