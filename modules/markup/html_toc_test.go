// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup_test

import (
	"regexp"
	"testing"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestToCWithHTML(t *testing.T) {
	defer test.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()

	t1 := `tag <a href="link">link</a> and <b>Bold</b>`
	t2 := "code block `<a>`"
	t3 := "markdown **bold**"
	input := `---
include_toc: true
---

# ` + t1 + `
## ` + t2 + `
#### ` + t3 + `
## last
`

	renderCtx := markup.NewTestRenderContext().WithEnableHeadingIDGeneration(true)
	resultHTML, err := markdown.RenderString(renderCtx, input)
	assert.NoError(t, err)
	result := string(resultHTML)
	re := regexp.MustCompile(`(?s)<details class="frontmatter-content">.*?</details>`)
	result = re.ReplaceAllString(result, "\n")
	expected := `<details><summary>toc</summary>
<ul>
  <li><a href="#user-content-tag-link-and-bold" rel="nofollow">tag link and Bold</a></li>
  <ul>
    <li><a href="#user-content-code-block-a" rel="nofollow">code block &lt;a&gt;</a></li>
    <ul>
      <ul>
        <li><a href="#user-content-markdown-bold" rel="nofollow">markdown bold</a></li>
      </ul>
    </ul>
    <li><a href="#user-content-last" rel="nofollow">last</a></li>
  </ul>
</ul>
</details>

<h1 id="user-content-tag-link-and-bold">tag <a href="/link" rel="nofollow">link</a> and <b>Bold</b></h1>
<h2 id="user-content-code-block-a">code block <code>&lt;a&gt;</code></h2>
<h4 id="user-content-markdown-bold">markdown <strong>bold</strong></h4>
<h2 id="user-content-last">last</h2>
`
	assert.Equal(t, expected, result)
}
