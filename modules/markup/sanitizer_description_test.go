// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDescriptionSanitizer(t *testing.T) {
	testCases := []string{
		`<h1>Title</h1>`, `Title`,
		`<img src='img.png' alt='image'>`, ``,
		`<span class="emoji" aria-label="thumbs up">THUMBS UP</span>`, `<span class="emoji" aria-label="thumbs up">THUMBS UP</span>`,
		`<span style="color: red">Hello World</span>`, `<span>Hello World</span>`,
		`<br>`, ``,
		`<a href="https://example.com" target="_blank" rel="noopener noreferrer">https://example.com</a>`, `<a href="https://example.com" target="_blank" rel="noopener noreferrer nofollow">https://example.com</a>`,
		`<a href="data:1234">data</a>`, `data`,
		`<mark>Important!</mark>`, `Important!`,
		`<details>Click me! <summary>Nothing to see here.</summary></details>`, `Click me! Nothing to see here.`,
		`<input type="hidden">`, ``,
		`<b>I</b> have a <i>strong</i> <strong>opinion</strong> about <em>this</em>.`, `<b>I</b> have a <i>strong</i> <strong>opinion</strong> about <em>this</em>.`,
		`Provides alternative <code>wg(8)</code> tool`, `Provides alternative <code>wg(8)</code> tool`,
	}

	for i := 0; i < len(testCases); i += 2 {
		assert.Equal(t, testCases[i+1], SanitizeDescription(testCases[i]))
	}
}
