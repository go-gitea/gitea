// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jupyter

import (
	"fmt"
	"strings"
	"testing"

	"gitea.dev/modules/markup"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestRender(t *testing.T) {
	r := renderer{}

	t.Run("Basic notebook", func(t *testing.T) {
		input := `{
			"cells": [
				{
					"cell_type": "code",
					"execution_count": 1,
					"source": ["print('hello')"],
					"outputs": [
						{
							"output_type": "stream",
							"name": "stdout",
							"text": ["hello\n"]
						}
					]
				}
			],
			"metadata": {},
			"nbformat": 4
		}`

		var output strings.Builder
		ctx := &markup.RenderContext{}
		err := r.Render(ctx, strings.NewReader(input), &output)

		assert.NoError(t, err)
		result := output.String()
		assert.Contains(t, result, `<div class="jupyter-notebook">`)
		assert.Contains(t, result, `<div class="notebook-cell cell-type-code">`)
		assert.Contains(t, result, `In [1]:`)
		assert.Contains(t, result, `print`)
		assert.Contains(t, result, `hello`)
		assert.Contains(t, result, `stream-stdout`)
	})

	t.Run("Markdown cell with XSS Protection", func(t *testing.T) {
		input := `{
			"cells": [
				{
					"cell_type": "markdown",
					"source": [
						"# Title\n",
						"Some text\n",
						"[click me](javascript:alert(1))\n",
						"<script>alert('dangerous')</script>"
					]
				}
			],
			"metadata": {},
			"nbformat": 4
		}`

		var output strings.Builder
		ctx := markup.NewRenderContext(t.Context())
		err := r.Render(ctx, strings.NewReader(input), &output)

		assert.NoError(t, err)
		result := output.String()

		// Assert normal markup still renders correctly
		assert.Contains(t, result, `<div class="notebook-cell cell-type-markdown">`)
		assert.Contains(t, result, `Title`)
		assert.Contains(t, result, `Some text`)
		assert.Contains(t, result, `click me`)

		// CRITICAL SECURITY ASSERTIONS: Ensure XSS vectors are completely stripped
		assert.NotContains(t, result, `javascript:alert`)
		assert.NotContains(t, result, `<script>`)
	})

	t.Run("Cell limit truncation guardrail", func(t *testing.T) {
		// Generate an oversized notebook containing 105 cells dynamically
		var cellBlocks []string
		for range 105 {
			cellBlocks = append(cellBlocks, `{"cell_type": "markdown", "source": ["cell text"]}`)
		}
		input := fmt.Sprintf(`{"cells": [%s], "metadata": {}, "nbformat": 4}`, strings.Join(cellBlocks, ","))

		var output strings.Builder
		ctx := markup.NewRenderContext(t.Context())
		err := r.Render(ctx, strings.NewReader(input), &output)

		assert.NoError(t, err)
		result := output.String()

		// Verify it halts rendering gracefully and shows the truncation warning
		assert.Contains(t, result, "Output truncated.")
		assert.Contains(t, result, "This notebook contains too many cells to display efficiently.")

		// Count occurrences of the rendered cells to ensure it sliced down to exactly 100 elements
		assert.Equal(t, 100, strings.Count(result, `class="notebook-cell cell-type-markdown"`))
	})

	t.Run("Image output", func(t *testing.T) {
		input := `{
			"cells": [
				{
					"cell_type": "code",
					"execution_count": 1,
					"source": ["import matplotlib.pyplot as plt"],
					"outputs": [
						{
							"output_type": "display_data",
							"data": {
								"image/png": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
							}
						}
					]
				}
			],
			"metadata": {},
			"nbformat": 4
		}`

		var output strings.Builder
		ctx := markup.NewRenderContext(t.Context())
		err := r.Render(ctx, strings.NewReader(input), &output)

		assert.NoError(t, err)
		result := output.String()
		assert.Contains(t, result, `<img src="data:image/png;base64,`)
		assert.Contains(t, result, `iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==`)
	})

	t.Run("HTML output with style tag", func(t *testing.T) {
		input := `{
			"cells": [
				{
					"cell_type": "code",
					"execution_count": 1,
					"source": ["import pandas as pd"],
					"outputs": [
						{
							"output_type": "execute_result",
							"data": {
								"text/html": ["<style scoped>.dataframe tbody tr th { vertical-align: top; }</style><table class=\"dataframe\"><tr><td>1</td></tr></table>"]
							}
						}
					]
				}
			],
			"metadata": {},
			"nbformat": 4
		}`

		var output strings.Builder
		ctx := markup.NewRenderContext(t.Context())
		err := r.Render(ctx, strings.NewReader(input), &output)

		assert.NoError(t, err)
		result := output.String()
		assert.NotContains(t, result, `<style scoped>`)
		assert.Contains(t, result, `<table><tr><td>1</td></tr></table>`)
		assert.Contains(t, result, `<td>1</td>`)
	})

	t.Run("Error output", func(t *testing.T) {
		input := `{
			"cells": [
				{
					"cell_type": "code",
					"execution_count": 1,
					"source": ["raise ValueError('test error')"],
					"outputs": [
						{
							"output_type": "error",
							"ename": "ValueError",
							"evalue": "test error",
							"traceback": ["ValueError: test error"]
						}
					]
				}
			],
			"metadata": {},
			"nbformat": 4
		}`

		var output strings.Builder
		ctx := markup.NewRenderContext(t.Context())
		err := r.Render(ctx, strings.NewReader(input), &output)

		assert.NoError(t, err)
		result := output.String()
		assert.Contains(t, result, `ValueError: test error`)
		assert.Contains(t, result, `cell-output-error`)
	})

	t.Run("Old nbformat version", func(t *testing.T) {
		input := `{
			"cells": [],
			"metadata": {},
			"nbformat": 3
		}`

		var output strings.Builder
		ctx := markup.NewRenderContext(t.Context())
		err := r.Render(ctx, strings.NewReader(input), &output)

		assert.NoError(t, err)
		assert.Regexp(t, `<div class="file-not-rendered-prompt">This notebook uses an older format.*</div>`, output.String())
	})
}

func TestJoinSource(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "String input",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "Array input",
			input:    []any{"line1\n", "line2\n", "line3"},
			expected: "line1\nline2\nline3",
		},
		{
			name:     "Empty array",
			input:    []any{},
			expected: "",
		},
		{
			name:     "Single element array",
			input:    []any{"single"},
			expected: "single",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinSource(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIntegrationAndSanitization(t *testing.T) {
	// A mock malicious Jupyter notebook containing an XSS injection attempt
	// inside a text/html output cell (e.g., pretending to be a poisoned Pandas DataFrame).
	maliciousNotebook := `{
		"nbformat": 4,
		"nbformat_minor": 2,
		"metadata": {},
		"cells": [
			{
				"cell_type": "code",
				"execution_count": 1,
				"metadata": {},
				"source": ["a=1"],
				"outputs": [
					{
						"output_type": "execute_result",
						"execution_count": 1,
						"data": {
							"text/html": [
								"<div><script>alert('XSS Vector')</script><table class=\"dataframe\"><tr><td>Safe Content</td></tr></table></div>"
							]
						},
						"metadata": {}
					}
				]
			}
		]
	}`

	var output strings.Builder
	ctx := markup.NewRenderContext(t.Context())
	ctx.RenderOptions.MarkupType = "jupyter-render"
	err := markup.Render(ctx, strings.NewReader(maliciousNotebook), &output)
	assert.NoError(t, err)
	const expected = `
<div class="jupyter-notebook">
	<div class="notebook-cell cell-type-code">
		<div class="cell-line">
			<div class="cell-left cell-prompt">In [1]:</div>
			<div class="cell-right cell-input">
				<pre><code class="chroma language-python">
					<span class="n">a</span><span class="o">=</span><span class="mi">1</span>
				</code></pre>
			</div>
		</div>
		<div class="cell-line">
			<div class="cell-left cell-prompt">Out [1]:</div>
			<div class="cell-right cell-output">
				<div class="cell-output-html">
					<div><table><tbody><tr><td>Safe Content</td></tr></tbody></table></div>
				</div>
			</div>
		</div>
	</div>
</div>`
	assert.Equal(t, test.NormalizeHTMLSpaces(expected), test.NormalizeHTMLSpaces(output.String()))
}
