// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestMarkdownStripper(t *testing.T) {
	type testItem struct {
		markdown      string
		expectedText  []string
		expectedLinks []string
	}

	list := []testItem{
		{
			`
## This is a title

This is [one](link) to paradise.
This **is emphasized**.

` + "```" + `
This is a code block.
This should not appear in the output at all.
` + "```" + `

* Bullet 1
* Bullet 2

A HIDDEN ` + "`" + `GHOST` + "`" + ` IN THIS LINE.
		`,
			[]string{
				"This is a title",
				"This is",
				"to paradise.",
				"This",
				"is emphasized",
				".",
				"Bullet 1",
				"Bullet 2",
				"A HIDDEN",
				"IN THIS LINE.",
			},
			[]string{
				"link",
			}},
	}

	for _, test := range list {
		text, links := StripMarkdown([]byte(test.markdown))
		rawlines := strings.Split(text, "\n")
		lines := make([]string, 0, len(rawlines))
		for _, line := range rawlines {
			line := strings.TrimSpace(line)
			if line != "" {
				lines = append(lines, line)
			}
		}
		assert.EqualValues(t, test.expectedText, lines)
		assert.EqualValues(t, test.expectedLinks, links)
	}
}

func TestFindAllIssueReferences(t *testing.T) {
	text := `
#123 no, this is a title.
 #124 yes, this is a reference.
This [one](#919) no, this is a URL fragment.
This [two](/user2/repo1/issues/921) yes.
This [three](/user2/repo1/pulls/922) yes.
This [four](http://gitea.com:3000/user3/repo4/issues/203) yes.
This [five](http://github.com/user3/repo4/issues/204) no.

` + "```" + `
This is a code block.
#723 no, it's a code block.
` + "```" + `

This ` + "`" + `#724` + "`" + ` no, it's inline code.
This user3/repo4#200 yes.
This http://gitea.com:3000/user4/repo5/201 no, bad URL.
This http://gitea.com:3000/user4/repo5/pulls/202 yes.
This http://GiTeA.COM:3000/user4/repo6/pulls/205 yes.

		`
	// Note, FindAllIssueReferences() processes inline
	// references first, then link references.
	expected := []*RawIssueReference{
		// Inline references
		{124, "", ""},
		{200, "user3", "repo4"},
		// Link references
		{921, "user2", "repo1"},
		{922, "user2", "repo1"},
		{203, "user3", "repo4"},
		{202, "user4", "repo5"},
		{205, "user4", "repo6"},
	}

	// Save original value for other tests that may rely on it
	prevURL := setting.AppURL
	setting.AppURL = "https://gitea.com:3000/"

	refs := FindAllIssueReferences(text)
	assert.EqualValues(t, expected, refs)

	// Restore for other tests that may rely on the original value
	setting.AppURL = prevURL
}
