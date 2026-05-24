// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jupyter

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"

	"github.com/yuin/goldmark"
	goldmarkHTML "github.com/yuin/goldmark/renderer/html"
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
		SanitizerDisabled: true,
	}
}

func (renderer) FileNamePatterns() []string {
	return []string{"*.ipynb"}
}

func (renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return nil
}

// Notebook structures
type Notebook struct {
	Cells    []Cell                 `json:"cells"`
	Metadata map[string]interface{} `json:"metadata"`
	Nbformat int                    `json:"nbformat"`
}

type Cell struct {
	CellType       string                 `json:"cell_type"`
	Source         interface{}            `json:"source"` // string or []string
	Outputs        []Output               `json:"outputs,omitempty"`
	ExecutionCount interface{}            `json:"execution_count,omitempty"` // int or null
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type Output struct {
	OutputType string                 `json:"output_type"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Text       interface{}            `json:"text,omitempty"` // string or []string
	Name       string                 `json:"name,omitempty"`
	Traceback  interface{}            `json:"traceback,omitempty"` // []string
	Ename      string                 `json:"ename,omitempty"`
	Evalue     string                 `json:"evalue,omitempty"`
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
		_, _ = output.Write([]byte(fmt.Sprintf(
			`<div class="jupyter-notebook-message">This notebook uses an older format (nbformat %d). Only nbformat 4+ is supported for rendering. Please upgrade the notebook in Jupyter or view the raw JSON.</div>`,
			notebook.Nbformat,
		)))
		return nil
	}

	// Detect language
	language := "python" // default
	if metadata, ok := notebook.Metadata["language_info"].(map[string]interface{}); ok {
		if name, ok := metadata["name"].(string); ok {
			language = name
		}
	} else if kernelspec, ok := notebook.Metadata["kernelspec"].(map[string]interface{}); ok {
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
		_, _ = output.Write([]byte(fmt.Sprintf(`<div class="prompt input-prompt">In [%d]:</div>`, execCount)))
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
				_, _ = output.Write([]byte(fmt.Sprintf(`<div class="prompt output-prompt">Out[%d]:</div>`, execCount)))
			} else {
				_, _ = output.Write([]byte(`<div class="prompt output-prompt"></div>`))
			}

			_, _ = output.Write([]byte(`<div class="output">`))
			for _, out := range cell.Outputs {
				if err := renderOutput(output, out); err != nil {
					log.Warn("Failed to render output: %v", err)
				}
			}
			_, _ = output.Write([]byte(`</div></div>`))
		}

		_, _ = output.Write([]byte(`</div>`))
		*executionCount++
	}

	return nil
}

func renderMarkdown(ctx *markup.RenderContext, output io.Writer, source string) error {
	md := goldmark.New(
		goldmark.WithRendererOptions(
			goldmarkHTML.WithUnsafe(), // Allow raw HTML in markdown
		),
	)
	return md.Convert([]byte(source), output)
}

func renderOutput(output io.Writer, out Output) error {
	if out.Data != nil {
		// Image outputs
		if pngData, ok := out.Data["image/png"]; ok {
			imgData := joinSource(pngData)
			_, _ = output.Write([]byte(fmt.Sprintf(`<img src="data:image/png;base64,%s" style="max-width: 100%%;">`, imgData)))
			return nil
		}
		if jpegData, ok := out.Data["image/jpeg"]; ok {
			imgData := joinSource(jpegData)
			_, _ = output.Write([]byte(fmt.Sprintf(`<img src="data:image/jpeg;base64,%s" style="max-width: 100%%;">`, imgData)))
			return nil
		}
		if svgData, ok := out.Data["image/svg+xml"]; ok {
			_, _ = output.Write([]byte(fmt.Sprintf(`<div>%s</div>`, joinSource(svgData))))
			return nil
		}

		// HTML output
		if htmlData, ok := out.Data["text/html"]; ok {
			htmlContent := joinSource(htmlData)
			// Strip <style> tags as we handle DataFrame styles in CSS
			htmlContent = stripStyleTags(htmlContent)
			_, _ = output.Write([]byte(fmt.Sprintf(`<div style="overflow-x: auto; max-width: 100%%;">%s</div>`, htmlContent)))
			return nil
		}

		// LaTeX output
		if latexData, ok := out.Data["text/latex"]; ok {
			latex := joinSource(latexData)
			latex = strings.TrimPrefix(latex, "$$")
			latex = strings.TrimSuffix(latex, "$$")
			_, _ = output.Write([]byte(fmt.Sprintf(`<pre><code class="language-math display">%s</code></pre>`, html.EscapeString(latex))))
			return nil
		}

		// Plain text output
		if plainData, ok := out.Data["text/plain"]; ok {
			_, _ = output.Write([]byte(fmt.Sprintf(`<pre>%s</pre>`, html.EscapeString(joinSource(plainData)))))
			return nil
		}

		// Unsupported outputs
		if _, ok := out.Data["application/javascript"]; ok {
			_, _ = output.Write([]byte(`<div style="color: var(--color-text-light-2); font-style: italic;">[JavaScript output - execution disabled for security]</div>`))
			return nil
		}
		if _, ok := out.Data["application/vnd.plotly.v1+json"]; ok {
			_, _ = output.Write([]byte(`<div style="color: var(--color-text-light-2); font-style: italic;">[Plotly output - interactive plots not supported]</div>`))
			return nil
		}
		if _, ok := out.Data["application/vnd.jupyter.widget-view+json"]; ok {
			_, _ = output.Write([]byte(`<div style="color: var(--color-text-light-2); font-style: italic;">[Jupyter widget - interactive widgets not supported]</div>`))
			return nil
		}
	}

	// Stream output
	if out.OutputType == "stream" && out.Text != nil {
		_, _ = output.Write([]byte(fmt.Sprintf(`<pre class="stream-%s">%s</pre>`, out.Name, html.EscapeString(joinSource(out.Text)))))
		return nil
	}

	// Error output
	if out.OutputType == "error" {
		traceback := ""
		if out.Traceback != nil {
			if tb, ok := out.Traceback.([]interface{}); ok {
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
		_, _ = output.Write([]byte(fmt.Sprintf(`<pre class="error-output" style="color: var(--color-red);">%s</pre>`, html.EscapeString(traceback))))
		return nil
	}

	// Generic text output
	if out.Text != nil {
		_, _ = output.Write([]byte(fmt.Sprintf(`<pre>%s</pre>`, html.EscapeString(joinSource(out.Text)))))
		return nil
	}

	return nil
}

func joinSource(source interface{}) string {
	switch v := source.(type) {
	case string:
		return v
	case []interface{}:
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
