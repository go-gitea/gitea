// Copyright 2017 The Gitea Authors. All rights reserved.
// Copyright 2017 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Sanitizer(t *testing.T) {
	NewSanitizer()
	testCases := []string{
		// Regular
		`<a onblur="alert(secret)" href="http://www.google.com">Google</a>`, `<a href="http://www.google.com" rel="nofollow">Google</a>`,

		// Code highlighting class
		`<code class="random string"></code>`, `<code></code>`,
		`<code class="language-random ui tab active menu attached animating sidebar following bar center"></code>`, `<code></code>`,
		`<code class="language-go"></code>`, `<code class="language-go"></code>`,

		// Input checkbox
		`<input type="hidden">`, ``,
		`<input type="checkbox">`, `<input type="checkbox">`,
		`<input checked disabled autofocus>`, `<input checked="" disabled="">`,
	}

	for i := 0; i < len(testCases); i += 2 {
		assert.Equal(t, testCases[i+1], Sanitize(testCases[i]))
		assert.Equal(t, testCases[i+1], string(SanitizeBytes([]byte(testCases[i]))))
	}
}
