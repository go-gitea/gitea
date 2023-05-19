// Copyright 2017 The Gitea Authors. All rights reserved.
// Copyright 2017 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"html/template"
	"strings"
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

		// Code highlight injection
		`<code class="language-random&#32;ui&#32;tab&#32;active&#32;menu&#32;attached&#32;animating&#32;sidebar&#32;following&#32;bar&#32;center"></code>`, `<code></code>`,
		`<code class="language-lol&#32;ui&#32;tab&#32;active&#32;menu&#32;attached&#32;animating&#32;sidebar&#32;following&#32;bar&#32;center">
<code class="language-lol&#32;ui&#32;container&#32;input&#32;huge&#32;basic&#32;segment&#32;center">&nbsp;</code>
<img src="https://try.gogs.io/img/favicon.png" width="200" height="200">
<code class="language-lol&#32;ui&#32;container&#32;input&#32;massive&#32;basic&#32;segment">Hello there! Something has gone wrong, we are working on it.</code>
<code class="language-lol&#32;ui&#32;container&#32;input&#32;huge&#32;basic&#32;segment">In the meantime, play a game with us at&nbsp;<a href="http://example.com/">example.com</a>.</code>
</code>`, "<code>\n<code>\u00a0</code>\n<img src=\"https://try.gogs.io/img/favicon.png\" width=\"200\" height=\"200\">\n<code>Hello there! Something has gone wrong, we are working on it.</code>\n<code>In the meantime, play a game with us at\u00a0<a href=\"http://example.com/\" rel=\"nofollow\">example.com</a>.</code>\n</code>",

		// <kbd> tags
		`<kbd>Ctrl + C</kbd>`, `<kbd>Ctrl + C</kbd>`,
		`<i class="dropdown icon">NAUGHTY</i>`, `<i>NAUGHTY</i>`,
		`<i class="icon dropdown"></i>`, `<i class="icon dropdown"></i>`,
		`<input type="checkbox" disabled=""/>unchecked`, `<input type="checkbox" disabled=""/>unchecked`,
		`<span class="emoji dropdown">NAUGHTY</span>`, `<span>NAUGHTY</span>`,
		`<span class="emoji">contents</span>`, `<span class="emoji">contents</span>`,

		// Color property
		`<span style="color: red">Hello World</span>`, `<span style="color: red">Hello World</span>`,
		`<p style="color: red">Hello World</p>`, `<p style="color: red">Hello World</p>`,
		`<code style="color: red">Hello World</code>`, `<code>Hello World</code>`,
		`<span style="bad-color: red">Hello World</span>`, `<span>Hello World</span>`,
		`<p style="bad-color: red">Hello World</p>`, `<p>Hello World</p>`,
		`<code style="bad-color: red">Hello World</code>`, `<code>Hello World</code>`,

		// URLs
		`[my custom URL scheme](cbthunderlink://somebase64string)`, `[my custom URL scheme](cbthunderlink://somebase64string)`,
		`[my custom URL scheme](matrix:roomid/psumPMeAfzgAeQpXMG:feneas.org?action=join)`, `[my custom URL scheme](matrix:roomid/psumPMeAfzgAeQpXMG:feneas.org?action=join)`,
	}

	for i := 0; i < len(testCases); i += 2 {
		assert.Equal(t, testCases[i+1], Sanitize(testCases[i]))
	}
}

func TestSanitizeNonEscape(t *testing.T) {
	descStr := "<scrİpt>&lt;script&gt;alert(document.domain)&lt;/script&gt;</scrİpt>"

	output := template.HTML(Sanitize(descStr))
	if strings.Contains(string(output), "<script>") {
		t.Errorf("un-escaped <script> in output: %q", output)
	}
}
