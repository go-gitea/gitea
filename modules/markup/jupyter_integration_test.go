// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup_test

import (
	"context"
	"strings"
	"testing"

	"gitea.dev/modules/markup"

	_ "gitea.dev/modules/markup/jupyter"

	"github.com/stretchr/testify/assert"
)

func TestJupyterPipelineIntegrationAndSanitization(t *testing.T) {
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
				"source": ["df.head()"],
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

	// 1. Use the public constructor function to create a fully initialized RenderContext
	ctx := markup.NewRenderContext(context.Background())

	// 2. Explicitly assign the rendering engine to target your jupyter module
	ctx.RenderOptions.MarkupType = "jupyter"

	// 3. Execute the render using Gitea's global pipeline entrypoint
	err := markup.Render(ctx, strings.NewReader(maliciousNotebook), &output)

	// Assertions
	assert.NoError(t, err)
	result := output.String()

	// Verify that the legitimate, safe layout content made it through
	assert.Contains(t, result, `class="jupyter-html-output"`)
	assert.Contains(t, result, `<table class="dataframe">`)
	assert.Contains(t, result, `Safe Content`)

	// Verify that the internal bluemonday UGCPolicy intercepted and neutralized the exploit
	assert.NotContains(t, result, `<script>`)
	assert.NotContains(t, result, `alert('XSS Vector')`)
}
