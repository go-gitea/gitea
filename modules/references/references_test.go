// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package references

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

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
Closing #15
I am opening #20 for you
Do you process user6/repo6#300 ?
For 999 #1235 no keyword.
Which abc. #9434 nk either.
		`
	// Note, FindAllIssueReferences() processes references in this order:
	// * Issue number: #123
	// * Repository/issue number: user/repo#123
	// * URL: http:// ....
	expected := []*RawIssueReference{
		// Numeric references
		{124, "", "", ""},
		{15, "", "", "Closing"},
		{20, "", "", "opening"},
		{1235, "", "", ""},
		{9434, "", "", ""},
		// Repository/issue references
		{200, "user3", "repo4", "This"},
		{300, "user6", "repo6", "process"},
		// Link references
		{921, "user2", "repo1", ""},
		{922, "user2", "repo1", ""},
		{203, "user3", "repo4", ""},
		{202, "user4", "repo5", ""},
		{205, "user4", "repo6", ""},
	}

	// Save original value for other tests that may rely on it
	prevURL := setting.AppURL
	setting.AppURL = "https://gitea.com:3000/"

	refs := FindAllIssueReferences(text)
	assert.EqualValues(t, expected, refs)

	// Restore for other tests that may rely on the original value
	setting.AppURL = prevURL
}
