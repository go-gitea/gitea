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
	type testFixture struct {
		input		string
		expected	[]*RawIssueReference
	}

	fixtures := []testFixture {
		{
			"Simply closes: #29 yes",
			[]*RawIssueReference {
				&RawIssueReference{ 29, "", "", XRefActionCloses,
									ReferenceLocation{Start:15, End:18},
									ReferenceLocation{Start:7, End:13},
								 },
			},
		},
		{
			"#123 no, this is a title.",
			[]*RawIssueReference{
			},
		},
		{
			" #124 yes, this is a reference.",
			[]*RawIssueReference {
				&RawIssueReference{ 124, "", "", XRefActionNone,
					ReferenceLocation{Start:0, End:4},
					ReferenceLocation{Start:0, End:0},
				},
			},
		},
		{
			"```\nThis is a code block.\n#723 no, it's a code block.```",
			[]*RawIssueReference{
			},
		},
		{
			"This `#724` no, it's inline code.",
			[]*RawIssueReference{
			},
		},
		{
			"This user3/repo4#200 yes.",
			[]*RawIssueReference {
				&RawIssueReference{ 200, "user3", "repo4", /* This */ XRefActionNone,
					ReferenceLocation{Start:5, End:20},
					ReferenceLocation{Start:0, End:0},
				},
			},
		},
		{
			"This [one](#919) no, this is a URL fragment.",
			[]*RawIssueReference{
			},
		},
		{
			"This [two](/user2/repo1/issues/921) yes.",
			[]*RawIssueReference {
				&RawIssueReference{ 921, "user2", "repo1", XRefActionNone,
					ReferenceLocation{Start:0, End:0},
					ReferenceLocation{Start:0, End:0},
				},
			},
		},
		{
			"This [three](/user2/repo1/pulls/922) yes.",
			[]*RawIssueReference {
				&RawIssueReference{ 922, "user2", "repo1", XRefActionNone,
					ReferenceLocation{Start:0, End:0},
					ReferenceLocation{Start:0, End:0},
				},
			},
		},
		{
			"This [four](http://gitea.com:3000/user3/repo4/issues/203) yes.",
			[]*RawIssueReference {
				&RawIssueReference{ 203, "user3", "repo4", XRefActionNone,
					ReferenceLocation{Start:0, End:0},
					ReferenceLocation{Start:0, End:0},
				},
			},
		},
		{
			"This [five](http://github.com/user3/repo4/issues/204) no.",
			[]*RawIssueReference{
			},
		},
		{
			"This http://gitea.com:3000/user4/repo5/201 no, bad URL.",
			[]*RawIssueReference{
			},
		},
		{
			"This http://gitea.com:3000/user4/repo5/pulls/202 yes.",
			[]*RawIssueReference {
				&RawIssueReference{ 202, "user4", "repo5", XRefActionNone,
					ReferenceLocation{Start:0, End:0},
					ReferenceLocation{Start:0, End:0},
				},
			},
		},
		{
			"This http://GiTeA.COM:3000/user4/repo6/pulls/205 yes.",
			[]*RawIssueReference {
				&RawIssueReference{ 205, "user4", "repo6", XRefActionNone,
					ReferenceLocation{Start:0, End:0},
					ReferenceLocation{Start:0, End:0},
				},
			},
		},
		{
			"Reopens #15 yes",
			[]*RawIssueReference {
				&RawIssueReference{ 15, "", "", XRefActionReopens,
					ReferenceLocation{Start:8, End:11},
					ReferenceLocation{Start:0, End:7},
				},
			},
		},
		{
			"This closes #20 for you yes",
			[]*RawIssueReference {
				&RawIssueReference{ 20, "", "", XRefActionCloses,
					ReferenceLocation{Start:12, End:15},
					ReferenceLocation{Start:5, End:11},
				},
			},
		},
		{
			"Do you fix user6/repo6#300 ? yes",
			[]*RawIssueReference {
				&RawIssueReference{ 300, "user6", "repo6", XRefActionCloses,
					ReferenceLocation{Start:11, End:26},
					ReferenceLocation{Start:7, End:10},
				},
			},
		},
		{
			"For 999 #1235 no keyword, but yes",
			[]*RawIssueReference {
				&RawIssueReference{ 1235, "", "", XRefActionNone,
					ReferenceLocation{Start:8, End:13},
					ReferenceLocation{Start:0, End:0},
				},
			},
		},
		{
			"Which abc. #9434 same as above",
			[]*RawIssueReference {
				&RawIssueReference{ 9434, "", "", XRefActionNone,
					ReferenceLocation{Start:11, End:16},
					ReferenceLocation{Start:0, End:0},
				},
			},
		},
		{
			"This closes #600 and reopens #599",
			[]*RawIssueReference {
				&RawIssueReference{ 600, "", "", XRefActionCloses,
					ReferenceLocation{Start:12, End:16},
					ReferenceLocation{Start:5, End:11},
				},
				&RawIssueReference{ 599, "", "", XRefActionReopens,
					ReferenceLocation{Start:29, End:33},
					ReferenceLocation{Start:21, End:28},
				},
			},
		},
	}

	// Save original value for other tests that may rely on it
	prevURL := setting.AppURL
	setting.AppURL = "https://gitea.com:3000/"

	for _, fixture := range fixtures {
		refs := FindAllIssueReferencesMarkdown(fixture.input)
		assert.EqualValues(t, fixture.expected, refs, "Failed to parse: {%s}", fixture.input)
	}

	// Restore for other tests that may rely on the original value
	setting.AppURL = prevURL
}
