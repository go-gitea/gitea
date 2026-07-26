// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jupyter

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"strings"
	"sync"

	"gitea.dev/modules/highlight"
	"gitea.dev/modules/htmlutil"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

func init() {
	markup.RegisterRenderer(renderer{})
}

// Renderer implements markup.Renderer for Jupyter notebooks
type renderer struct{}

var (
	_ markup.Renderer            = (*renderer)(nil)
	_ markup.PostProcessRenderer = (*renderer)(nil)
	_ markup.ExternalRenderer    = (*renderer)(nil) // FIXME: this is not an external render, need to refactor the framework in the future
)

type mimeHandler struct {
	Mime string
	Fn   func(w htmlutil.HTMLWriter, data string) error
}

func renderCellCodeOutputTextPlain(w htmlutil.HTMLWriter, text string) error {
	w.WriteFormat(`<div class="cell-output-text"><pre>%s</pre></div>`, text)
	return w.Err()
}

func renderCellCodeOutputUnsupported(w htmlutil.HTMLWriter, message string) error {
	w.WriteFormat(`<div class="cell-output-unsupported">%s</div>`, message)
	return w.Err()
}

var dataMimeHandlers = sync.OnceValue(func() []mimeHandler {
	renderImage := func(w htmlutil.HTMLWriter, subtype, payload string) error {
		w.WriteFormat(`<div class="cell-output-image"><img src="data:image/%s;base64,%s"></div>`, subtype, payload)
		return w.Err()
	}
	renderUnsupportedOutput := func(message string) func(htmlutil.HTMLWriter, string) error {
		return func(w htmlutil.HTMLWriter, _ string) error {
			return renderCellCodeOutputUnsupported(w, message)
		}
	}
	return []mimeHandler{
		// Images (PNG, JPEG, SVG)
		{"image/png", func(w htmlutil.HTMLWriter, d string) error {
			return renderImage(w, "png", d)
		}},
		{"image/jpeg", func(w htmlutil.HTMLWriter, d string) error {
			return renderImage(w, "jpeg", d)
		}},
		{"image/svg+xml", func(w htmlutil.HTMLWriter, d string) error {
			return renderImage(w, "svg+xml", base64.StdEncoding.EncodeToString(util.UnsafeStringToBytes(d)))
		}},

		// Rich & Math Layouts
		{"text/html", func(w htmlutil.HTMLWriter, d string) error {
			// To future developers:  don't allow custom CSS classes or attributes,
			// because ".link-action" or "data-fetch-xxx" can send POST requests and lead to XSS.
			// If you'd really like to support more, do remember to correctly sanitize the values.
			w.WriteFormat(`<div class="cell-output-html">%s</div>`, markup.Sanitize(d))
			return w.Err()
		}},
		{"text/latex", func(w htmlutil.HTMLWriter, d string) error {
			w.WriteFormat(`<div class="cell-output-latex"><pre><code class="language-math display">%s</code></pre></div>`, trimMathDelimiters(d))
			return w.Err()
		}},
		{"text/plain", renderCellCodeOutputTextPlain},

		// Security Placeholders
		{"application/javascript", renderUnsupportedOutput("[JavaScript output - execution disabled for security]")},
		{"application/vnd.plotly.v1+json", renderUnsupportedOutput("[Plotly output - interactive plots not supported]")},
		{"application/vnd.jupyter.widget-view+json", renderUnsupportedOutput("[Jupyter widget - interactive widgets not supported]")},
	}
})

func (renderer) Name() string {
	return "jupyter-render"
}

func (renderer) NeedPostProcess() bool { return true }

func (renderer) GetExternalRendererOptions() markup.ExternalRendererOptions {
	return markup.ExternalRendererOptions{
		// HINT: no need to let markup render sanitize the output because there are many special CSS class names, inline attributes.
		// This render must guarantee that the output is safe and no XSS
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
func (renderer) Render(ctx *markup.RenderContext, input io.Reader, outputWriter io.Writer) error {
	htmlWriter := htmlutil.NewHTMLWriter(outputWriter)
	// the size is (should be) checked and/or limited by the caller to avoid OOM
	var notebook Notebook
	if err := json.NewDecoder(input).Decode(&notebook); err != nil {
		htmlWriter.WriteFormat(`<div class="ui error message">Failed to parse notebook JSON: %v</div>`, err)
		return htmlWriter.Err()
	}

	// Check nbformat version
	if notebook.Nbformat < 4 {
		msg := htmlutil.HTMLFormat("This notebook uses an older format (nbformat %d). Only nbformat 4+ is supported for rendering. Please upgrade the notebook in Jupyter or view the raw JSON.", notebook.Nbformat)
		htmlWriter.WriteFormat(`<div class="file-not-rendered-prompt">%s</div>`, msg)
		return htmlWriter.Err()
	}

	// Detect language
	language := "python" // default
	if metadata, ok := notebook.Metadata["language_info"].(map[string]any); ok {
		if name, ok := metadata["name"].(string); ok {
			language = name
		}
	} else if kernelSpec, ok := notebook.Metadata["kernelspec"].(map[string]any); ok {
		if lang, ok := kernelSpec["language"].(string); ok {
			language = lang
		}
	}

	// Start rendering
	htmlWriter.WriteHTML(`<div class="jupyter-notebook">`)

	// limiting the cell rendering to 100 cells
	cells := notebook.Cells
	truncated := false
	const maxRenderedCells = 100

	if len(cells) > maxRenderedCells {
		cells = cells[:maxRenderedCells] // Slice down to exactly 100 elements instantly at the pointer layer
		truncated = true
	}

	for _, cell := range cells {
		if err := renderCell(ctx, htmlWriter, cell, language); err != nil {
			log.Warn("Failed to render cell: %v", err) // TODO: RENDER-LOG-HANDLING: see other comments
			continue
		}
	}

	if truncated {
		renderCellPrompt(htmlWriter, "Warning:", "Output truncated. This notebook contains too many cells to display efficiently.")
	}

	htmlWriter.WriteHTML(`</div>`)
	return htmlWriter.Err()
}

func renderCellCode(output htmlutil.HTMLWriter, cell Cell, language string) error {
	source := joinSource(cell.Source)
	var executionCount *int64
	if cell.ExecutionCount != nil {
		if count, err := util.ToInt64(cell.ExecutionCount); err == nil {
			executionCount = &count
		}
	}

	output.WriteHTML(`<div class="cell-line">`)
	{
		if executionCount != nil {
			output.WriteFormat(`<div class="cell-left cell-prompt">In [%d]:</div>`, *executionCount)
		} else {
			output.WriteHTML(`<div class="cell-left cell-prompt">In [ ]:</div>`)
		}

		// Highlight code
		lexer := highlight.DetectChromaLexerByFileName("", language)
		output.WriteFormat(`<div class="cell-right cell-input"><pre><code class="chroma language-%s">`, strings.ToLower(language))
		output.WriteHTML(highlight.RenderCodeByLexer(lexer, source))
		output.WriteHTML("</code></pre></div>")
	}
	output.WriteHTML(`</div>`)

	// Render outputs
	if len(cell.Outputs) > 0 {
		hasExecutionResult := false
		for _, out := range cell.Outputs {
			if out.OutputType == "execute_result" {
				hasExecutionResult = true
				break
			}
		}

		output.WriteHTML(`<div class="cell-line">`)
		{
			if hasExecutionResult && executionCount != nil {
				output.WriteFormat(`<div class="cell-left cell-prompt">Out [%d]:</div>`, *executionCount)
			} else {
				output.WriteHTML(`<div class="cell-left cell-prompt"></div>`)
			}

			output.WriteHTML(`<div class="cell-right cell-output">`)
			for _, out := range cell.Outputs {
				renderCellCodeOutput(output, out)
			}
			output.WriteHTML(`</div>`)
		}
		output.WriteHTML(`</div>`)
	}

	return output.Err()
}

func renderCellPrompt(output htmlutil.HTMLWriter, left, right template.HTML) {
	output.WriteFormat(`
<div class="notebook-cell">
	<div class="cell-line">
		<div class="cell-left cell-prompt">%s</div>
		<div class="cell-right cell-prompt">%s</div>
	</div>
</div>`, left, right)
}

func renderCell(ctx *markup.RenderContext, output htmlutil.HTMLWriter, cell Cell, language string) error {
	switch cell.CellType {
	case "markdown":
		output.WriteHTML(`
<div class="notebook-cell cell-type-markdown">
	<div class="cell-line">
		<div class="cell-left cell-prompt"></div>
		<div class="cell-right">`)
		if err := renderCellMarkdown(ctx, output, joinSource(cell.Source)); err != nil {
			return err
		}
		output.WriteHTML(`
		</div>
	</div>
</div>`)
	case "code":
		output.WriteHTML(`<div class="notebook-cell cell-type-code">`)
		if err := renderCellCode(output, cell, language); err != nil {
			return err
		}
		output.WriteHTML(`</div>`)
	default:
		renderCellPrompt(output, "Cell:", htmlutil.HTMLFormat("[Cell type %s - unsupported, skipped]", cell.CellType))
	}
	return output.Err()
}

func renderCellMarkdown(rctx *markup.RenderContext, output htmlutil.HTMLWriter, source string) error {
	markdownCtx := markup.NewRenderContext(rctx)
	// make sure the markdown render use the same options and helper to generate correct contents (e.g.: links)
	markdownCtx.RenderOptions = rctx.RenderOptions
	markdownCtx.RenderHelper = rctx.RenderHelper
	output.WriteHTML(`<div class="embedded-markdown">`)
	if err := markdown.Render(markdownCtx, strings.NewReader(source), output.OriginWriter()); err != nil {
		return err
	}
	output.WriteHTML(`</div>`)
	return output.Err()
}

func renderCellCodeOutput(output htmlutil.HTMLWriter, out Output) {
	if out.Data != nil {
		// Iterate through our priority list to find the best matching MIME handler available
		for _, h := range dataMimeHandlers() {
			if rawPayload, exists := out.Data[h.Mime]; exists {
				var stringPayload string

				// Flatten the polymorphic JSON input (string or []any) into a single clean string
				switch v := rawPayload.(type) {
				case string:
					stringPayload = v
				case []any:
					stringPayload = joinSource(v)
				default:
					_ = renderCellCodeOutputUnsupported(output, fmt.Sprintf("[Data output - unsupported data type %T for mime type %s]", rawPayload, h.Mime))
					continue
				}

				if err := h.Fn(output, stringPayload); err != nil {
					// TODO: RENDER-LOG-HANDLING: outputting render's error to sever's log is not a proper approach
					// The errors can be:
					// * unsupported element (cell, data, etc): it should render the message on the UI to tell users that the content is not supported, or ignore them if they are ignore-able
					// * logic error: it should report to server logs
					// * network error: io.Writer tries to write to the HTTP connection, so the error can also be a network error, such error should be ignored
					log.Error("Jupyter rendering engine failed for MIME type %s: %v", h.Mime, err)
				}

				// Return immediately after rendering the top matching priority format
				return
			}
		}
	}

	// Stream output
	if out.OutputType == "stream" && out.Text != nil {
		streamName := util.Iif(out.Name == "stderr", "stderr", "stdout")
		output.WriteFormat(`<pre class="cell-output-stream stream-%s">%s</pre>`, streamName, joinSource(out.Text))
		return
	}

	// Error output
	if out.OutputType == "error" {
		traceback := ""
		if tb, ok := out.Traceback.([]any); ok {
			lines := make([]string, len(tb))
			for i, line := range tb {
				lines[i] = fmt.Sprint(line)
			}
			traceback = strings.Join(lines, "\n")
		}
		if traceback == "" && out.Ename != "" {
			traceback = fmt.Sprintf("%s: %s", out.Ename, out.Evalue)
		}
		output.WriteFormat(`<pre class="cell-output-error">%s</pre>`, traceback)
		return
	}

	// Generic text output
	if out.Text != nil {
		_ = renderCellCodeOutputTextPlain(output, joinSource(out.Text))
	}
}

func joinSource(source any) string {
	switch v := source.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		// the "source slice item" has EOL ("\n"), so just join them together
		parts := make([]string, len(v))
		for i, part := range v {
			parts[i] = fmt.Sprint(part)
		}
		return strings.Join(parts, "")
	default:
		return fmt.Sprint(v)
	}
}

// trimMathDelimiters strips a single pair of surrounding math delimiters ("$$...$$" or "$...$"),
// so the inner expression is handled by the math post-processor. Unlike strings.Trim, it does not
// eat unrelated "$" characters elsewhere in multi-expression content.
func trimMathDelimiters(s string) string {
	s = strings.TrimSpace(s)
	if t, ok := strings.CutPrefix(s, "$$"); ok {
		return strings.TrimSuffix(t, "$$")
	}
	if t, ok := strings.CutPrefix(s, "$"); ok {
		return strings.TrimSuffix(t, "$")
	}
	return s
}
