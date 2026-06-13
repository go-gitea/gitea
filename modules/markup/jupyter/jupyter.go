// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jupyter

import (
	"encoding/base64"
	"fmt"
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

	jupyterGoldmark = goldmark.New(
		goldmark.WithExtensions(
			extension.Table,
			extension.Strikethrough,
			extension.Linkify,
			extension.TaskList,
			extension.DefinitionList,
		),
	)
}

// Renderer implements markup.Renderer for Jupyter notebooks
type renderer struct{}

var (
	_ markup.Renderer            = (*renderer)(nil)
	_ markup.PostProcessRenderer = (*renderer)(nil)
	_ markup.ExternalRenderer    = (*renderer)(nil)

	jupyterGoldmark goldmark.Markdown
)

type mimeHandler struct {
	Mime string
	Fn   func(w io.Writer, data string) error
}

var dataMimeHandlers = []mimeHandler{
	// Images (PNG, JPEG, SVG)
	{"image/png", func(w io.Writer, d string) error { return renderJupyterImg(w, "png", d) }},
	{"image/jpeg", func(w io.Writer, d string) error { return renderJupyterImg(w, "jpeg", d) }},
	{"image/svg+xml", func(w io.Writer, d string) error {
		return renderJupyterImg(w, "svg+xml", base64.StdEncoding.EncodeToString([]byte(d)))
	}},

	// Rich & Math Layouts
	{"text/html", func(w io.Writer, d string) error {
		_, err := w.Write([]byte(`<div class="jupyter-html-output">` + markup.Sanitize(stripStyleTags(d)) + `</div>`))
		return err
	}},
	{"text/latex", func(w io.Writer, d string) error {
		_, err := htmlutil.HTMLPrintf(w, `<pre><code class="language-math display">%s</code></pre>`, strings.Trim(d, "$"))
		return err
	}},
	{"text/plain", func(w io.Writer, d string) error {
		_, err := htmlutil.HTMLPrintf(w, `<pre>%s</pre>`, d)
		return err
	}},

	// Security Placeholders
	{"application/javascript", renderUnsupported("[JavaScript output - execution disabled for security]")},
	{"application/vnd.plotly.v1+json", renderUnsupported("[Plotly output - interactive plots not supported]")},
	{"application/vnd.jupyter.widget-view+json", renderUnsupported("[Jupyter widget - interactive widgets not supported]")},
}

func renderJupyterImg(w io.Writer, subtype, payload string) error {
	_, err := htmlutil.HTMLPrintf(w, `<img src="data:image/%s;base64,%s" class="jupyter-output-image">`, subtype, payload)
	return err
}

func renderUnsupported(message string) func(io.Writer, string) error {
	return func(w io.Writer, _ string) error {
		_, err := w.Write([]byte(`<div class="jupyter-unsupported-output">` + message + `</div>`))
		return err
	}
}

func (renderer) Name() string {
	return "jupyter"
}

func (renderer) NeedPostProcess() bool { return true }

func (renderer) GetExternalRendererOptions() markup.ExternalRendererOptions {
	return markup.ExternalRendererOptions{
		// CRITICAL SECURITY NOTE: Gitea's root global HTML sanitizer is disabled HERE
		// because Jupyter Notebooks rely heavily on custom inline layout attributes,
		// multi-layer tables (DataFrames), and embedded base64 image data URIs that
		// the global sanitizer would aggressively strip out, ruining the frontend layout.
		//
		// To maintain strict defense-in-depth and completely close execution vectors (Stored XSS):
		// 1. Markdown cells are forcefully buffered and passed through markup.Sanitize() inside renderMarkdown().
		// 2. HTML output blocks are explicitly sanitized via markup.Sanitize() inside renderOutput().
		//
		// DO NOT flip this flag to false without refactoring the granular cell parsing loops.
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
		_, _ = htmlutil.HTMLPrintf(output, `<div class="jupyter-notebook-error">Failed to parse notebook JSON: %v</div>`, err)
		return nil
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

	// limiting the cell rendering to 100 cells
	cells := notebook.Cells
	truncated := false
	const maxRenderedCells = 100

	if len(cells) > maxRenderedCells {
		cells = cells[:maxRenderedCells] // Slice down to exactly 100 elements instantly at the pointer layer
		truncated = true
	}

	for _, cell := range cells {
		if err := renderCell(ctx, output, cell, language); err != nil {
			log.Warn("Failed to render cell: %v", err)
			continue
		}
	}

	if truncated {
		_, _ = output.Write([]byte(`<div class="ui warning message jupyter-notebook-message">`))
		_, _ = output.Write([]byte(`<strong>Output truncated.</strong> This notebook contains too many cells to display efficiently.`))
		_, _ = output.Write([]byte(`</div>`))
	}

	_, _ = output.Write([]byte(`</div>`))
	return nil
}

func renderCell(ctx *markup.RenderContext, output io.Writer, cell Cell, language string) error {
	source := joinSource(cell.Source)

	switch cell.CellType {
	case "markdown":
		_, _ = output.Write([]byte(`<div class="cell markdown"><div class="input markup">`))
		if err := renderMarkdown(ctx, output, source); err != nil {
			return err
		}
		_, _ = output.Write([]byte(`</div></div>`))

	case "code":
		hasCount := false
		countVal := 0
		if cell.ExecutionCount != nil {
			if count, ok := cell.ExecutionCount.(float64); ok {
				hasCount = true
				countVal = int(count)
			}
		}

		_, _ = output.Write([]byte(`<div class="cell code">`))
		_, _ = output.Write([]byte(`<div class="input-wrapper">`))
		if hasCount {
			_, _ = htmlutil.HTMLPrintf(output, `<div class="prompt input-prompt">In [%d]:</div>`, countVal)
		} else {
			_, _ = output.Write([]byte(`<div class="prompt input-prompt">In [ ]:</div>`))
		}
		_, _ = output.Write([]byte(`<div class="input">`))

		// Highlight code
		lexer := highlight.DetectChromaLexerByFileName("", language)
		if lexer == nil {
			lexer = highlight.DetectChromaLexerByFileName("", "plaintext")
		}
		_, _ = htmlutil.HTMLPrintf(output, `<pre><code class="chroma language-%s">`, strings.ToLower(language))
		_, _ = output.Write([]byte(highlight.RenderCodeByLexer(lexer, source)))
		_, _ = output.Write([]byte("</code></pre>"))

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
			if hasExecutionResult && hasCount {
				_, _ = htmlutil.HTMLPrintf(output, `<div class="prompt output-prompt">Out[%d]:</div>`, countVal)
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

	default:
		log.Debug("Jupyter markup: unknown cell type %q encountered in notebook, skipping", cell.CellType)
	}

	return nil
}

func renderMarkdown(_ *markup.RenderContext, output io.Writer, source string) error {
	var buf strings.Builder
	if err := jupyterGoldmark.Convert([]byte(source), &buf); err != nil {
		return err
	}

	// Sanitize the generated markdown HTML before sending it to the DOM
	safeHTML := markup.Sanitize(buf.String())
	_, _ = output.Write([]byte(safeHTML))
	return nil
}

func renderOutput(output io.Writer, out Output) {
	if out.Data != nil {
		// Iterate through our priority list to find the best matching MIME handler available
		for _, h := range dataMimeHandlers {
			if rawPayload, exists := out.Data[h.Mime]; exists {
				var stringPayload string

				// Flatten the polymorphic JSON input (string or []any) into a single clean string
				switch v := rawPayload.(type) {
				case string:
					stringPayload = v
				case []any:
					stringPayload = joinSource(v)
				default:
					log.Debug("Jupyter markup: unexpected format variant type for MIME key %s, skipping", h.Mime)
					continue
				}

				if err := h.Fn(output, stringPayload); err != nil {
					log.Error("Jupyter rendering engine failed for MIME type %s: %v", h.Mime, err)
				}

				// Return immediately after rendering the top matching priority format
				return
			}
		}
	}

	// Stream output
	if out.OutputType == "stream" && out.Text != nil {
		streamName := "stdout"
		if out.Name == "stderr" {
			streamName = "stderr"
		}
		_, _ = htmlutil.HTMLPrintf(output, `<pre class="stream-%s">%s</pre>`, streamName, joinSource(out.Text))
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
		_, _ = htmlutil.HTMLPrintf(output, `<pre class="error-output">%s</pre>`, traceback)
		return
	}

	// Generic text output
	if out.Text != nil {
		_, _ = htmlutil.HTMLPrintf(output, `<pre>%s</pre>`, joinSource(out.Text))
	}
}

func joinSource(source any) string {
	switch v := source.(type) {
	case nil:
		return ""
	case string:
		return v
	case []string:
		return strings.Join(v, "")
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
