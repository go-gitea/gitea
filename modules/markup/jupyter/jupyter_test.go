// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jupyter

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/markup"

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
		assert.Contains(t, result, `<div class="cell code">`)
		assert.Contains(t, result, `In [1]:`)
		assert.Contains(t, result, `print`)
		assert.Contains(t, result, `hello`)
		assert.Contains(t, result, `stream-stdout`)
	})

	t.Run("Markdown cell", func(t *testing.T) {
		input := `{
			"cells": [
				{
					"cell_type": "markdown",
					"source": ["# Title\n", "Some text"]
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
		assert.Contains(t, result, `<div class="cell markdown">`)
		assert.Contains(t, result, `<h1`)
		assert.Contains(t, result, `Title`)
		assert.Contains(t, result, `Some text`)
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
		ctx := &markup.RenderContext{}
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
		ctx := &markup.RenderContext{}
		err := r.Render(ctx, strings.NewReader(input), &output)

		assert.NoError(t, err)
		result := output.String()
		assert.NotContains(t, result, `<style scoped>`)
		assert.Contains(t, result, `<table class="dataframe">`)
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
		ctx := &markup.RenderContext{}
		err := r.Render(ctx, strings.NewReader(input), &output)

		assert.NoError(t, err)
		result := output.String()
		assert.Contains(t, result, `ValueError: test error`)
		assert.Contains(t, result, `error-output`)
	})

	t.Run("Old nbformat version", func(t *testing.T) {
		input := `{
			"cells": [],
			"metadata": {},
			"nbformat": 3
		}`

		var output strings.Builder
		ctx := &markup.RenderContext{}
		err := r.Render(ctx, strings.NewReader(input), &output)

		assert.NoError(t, err)
		result := output.String()
		assert.Contains(t, result, `jupyter-notebook-message`)
		assert.Contains(t, result, `nbformat 3`)
	})
}

func TestStripStyleTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Single style tag",
			input:    `<style scoped>.test { color: red; }</style><div>content</div>`,
			expected: `<div>content</div>`,
		},
		{
			name:     "Multiple style tags",
			input:    `<style>.a{}</style><div>text</div><style>.b{}</style>`,
			expected: `<div>text</div>`,
		},
		{
			name:     "No style tags",
			input:    `<div>content</div>`,
			expected: `<div>content</div>`,
		},
		{
			name:     "Style tag with attributes",
			input:    `<style type="text/css" scoped>.test{}</style><p>text</p>`,
			expected: `<p>text</p>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripStyleTags(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJoinSource(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "String input",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "Array input",
			input:    []interface{}{"line1\n", "line2\n", "line3"},
			expected: "line1\nline2\nline3",
		},
		{
			name:     "Empty array",
			input:    []interface{}{},
			expected: "",
		},
		{
			name:     "Single element array",
			input:    []interface{}{"single"},
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
