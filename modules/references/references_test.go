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

	type result struct {
		Index          int64
		Owner          string
		Name           string
		Issue          string
		Action         XRefAction
		RefLocation    *RefSpan
		ActionLocation *RefSpan
	}

	type testFixture struct {
		input    string
		expected []result
	}

	fixtures := []testFixture{
		{
			"Simply closes: #29 yes",
			[]result{
				{29, "", "", "29", XRefActionCloses, &RefSpan{Start: 15, End: 18}, &RefSpan{Start: 7, End: 13}},
			},
		},
		{
			"#123 no, this is a title.",
			[]result{},
		},
		{
			" #124 yes, this is a reference.",
			[]result{
				{124, "", "", "124", XRefActionNone, &RefSpan{Start: 0, End: 4}, nil},
			},
		},
		{
			"```\nThis is a code block.\n#723 no, it's a code block.```",
			[]result{},
		},
		{
			"This `#724` no, it's inline code.",
			[]result{},
		},
		{
			"This user3/repo4#200 yes.",
			[]result{
				{200, "user3", "repo4", "200", XRefActionNone, &RefSpan{Start: 5, End: 20}, nil},
			},
		},
		{
			"This [one](#919) no, this is a URL fragment.",
			[]result{},
		},
		{
			"This [two](/user2/repo1/issues/921) yes.",
			[]result{
				{921, "user2", "repo1", "921", XRefActionNone, nil, nil},
			},
		},
		{
			"This [three](/user2/repo1/pulls/922) yes.",
			[]result{
				{922, "user2", "repo1", "922", XRefActionNone, nil, nil},
			},
		},
		{
			"This [four](http://gitea.com:3000/user3/repo4/issues/203) yes.",
			[]result{
				{203, "user3", "repo4", "203", XRefActionNone, nil, nil},
			},
		},
		{
			"This [five](http://github.com/user3/repo4/issues/204) no.",
			[]result{},
		},
		{
			"This http://gitea.com:3000/user4/repo5/201 no, bad URL.",
			[]result{},
		},
		{
			"This http://gitea.com:3000/user4/repo5/pulls/202 yes.",
			[]result{
				{202, "user4", "repo5", "202", XRefActionNone, nil, nil},
			},
		},
		{
			"This http://GiTeA.COM:3000/user4/repo6/pulls/205 yes.",
			[]result{
				{205, "user4", "repo6", "205", XRefActionNone, nil, nil},
			},
		},
		{
			"Reopens #15 yes",
			[]result{
				{15, "", "", "15", XRefActionReopens, &RefSpan{Start: 8, End: 11}, &RefSpan{Start: 0, End: 7}},
			},
		},
		{
			"This closes #20 for you yes",
			[]result{
				{20, "", "", "20", XRefActionCloses, &RefSpan{Start: 12, End: 15}, &RefSpan{Start: 5, End: 11}},
			},
		},
		{
			"Do you fix user6/repo6#300 ? yes",
			[]result{
				{300, "user6", "repo6", "300", XRefActionCloses, &RefSpan{Start: 11, End: 26}, &RefSpan{Start: 7, End: 10}},
			},
		},
		{
			"For 999 #1235 no keyword, but yes",
			[]result{
				{1235, "", "", "1235", XRefActionNone, &RefSpan{Start: 8, End: 13}, nil},
			},
		},
		{
			"Which abc. #9434 same as above",
			[]result{
				{9434, "", "", "9434", XRefActionNone, &RefSpan{Start: 11, End: 16}, nil},
			},
		},
		{
			"This closes #600 and reopens #599",
			[]result{
				{600, "", "", "600", XRefActionCloses, &RefSpan{Start: 12, End: 16}, &RefSpan{Start: 5, End: 11}},
				{599, "", "", "599", XRefActionReopens, &RefSpan{Start: 29, End: 33}, &RefSpan{Start: 21, End: 28}},
			},
		},
	}

	// Save original value for other tests that may rely on it
	prevURL := setting.AppURL
	setting.AppURL = "https://gitea.com:3000/"

	for _, fixture := range fixtures {
		expraw := make([]*rawReference, len(fixture.expected))
		for i, e := range fixture.expected {
			expraw[i] = &rawReference{
				index:          e.Index,
				owner:          e.Owner,
				name:           e.Name,
				action:         e.Action,
				issue:          e.Issue,
				refLocation:    e.RefLocation,
				actionLocation: e.ActionLocation,
			}
		}
		expref := rawToIssueReferenceList(expraw)
		refs := FindAllIssueReferencesMarkdown(fixture.input)
		assert.EqualValues(t, expref, refs, "Failed to parse: {%s}", fixture.input)
		rawrefs := findAllIssueReferencesMarkdown(fixture.input)
		assert.EqualValues(t, expraw, rawrefs, "Failed to parse: {%s}", fixture.input)
	}

	// Restore for other tests that may rely on the original value
	setting.AppURL = prevURL

	type alnumFixture struct {
		input          string
		issue          string
		refLocation    *RefSpan
		action         XRefAction
		actionLocation *RefSpan
	}

	alnumFixtures := []alnumFixture{
		{
			"This ref ABC-123 is alphanumeric",
			"ABC-123", &RefSpan{Start: 9, End: 16},
			XRefActionNone, nil,
		},
		{
			"This closes ABCD-1234 alphanumeric",
			"ABCD-1234", &RefSpan{Start: 12, End: 21},
			XRefActionCloses, &RefSpan{Start: 5, End: 11},
		},
	}

	for _, fixture := range alnumFixtures {
		found, ref := FindRenderizableReferenceAlphanumeric(fixture.input)
		if fixture.issue == "" {
			assert.False(t, found, "Failed to parse: {%s}", fixture.input)
		} else {
			assert.True(t, found, "Failed to parse: {%s}", fixture.input)
			assert.Equal(t, fixture.issue, ref.Issue, "Failed to parse: {%s}", fixture.input)
			assert.Equal(t, fixture.refLocation, ref.RefLocation, "Failed to parse: {%s}", fixture.input)
			assert.Equal(t, fixture.action, ref.Action, "Failed to parse: {%s}", fixture.input)
			assert.Equal(t, fixture.actionLocation, ref.ActionLocation, "Failed to parse: {%s}", fixture.input)
		}
	}
}

func TestRegExp_mentionPattern(t *testing.T) {
	trueTestCases := []struct {
		pat string
		exp string
	}{
		{"@Unknwon", "@Unknwon"},
		{"@ANT_123", "@ANT_123"},
		{"@xxx-DiN0-z-A..uru..s-xxx", "@xxx-DiN0-z-A..uru..s-xxx"},
		{"   @lol   ", "@lol"},
		{" @Te-st", "@Te-st"},
		{"(@gitea)", "@gitea"},
		{"[@gitea]", "@gitea"},
		{"@gitea! this", "@gitea"},
		{"@gitea? this", "@gitea"},
		{"@gitea. this", "@gitea"},
		{"@gitea, this", "@gitea"},
		{"@gitea; this", "@gitea"},
		{"@gitea!\nthis", "@gitea"},
		{"\n@gitea?\nthis", "@gitea"},
		{"\t@gitea.\nthis", "@gitea"},
		{"@gitea,\nthis", "@gitea"},
		{"@gitea;\nthis", "@gitea"},
		{"@gitea!", "@gitea"},
		{"@gitea?", "@gitea"},
		{"@gitea.", "@gitea"},
		{"@gitea,", "@gitea"},
		{"@gitea;", "@gitea"},
	}
	falseTestCases := []string{
		"@ 0",
		"@ ",
		"@",
		"",
		"ABC",
		"@.ABC",
		"/home/gitea/@gitea",
		"\"@gitea\"",
		"@@gitea",
		"@gitea!this",
		"@gitea?this",
		"@gitea,this",
		"@gitea;this",
	}

	for _, testCase := range trueTestCases {
		found := mentionPattern.FindStringSubmatch(testCase.pat)
		assert.Len(t, found, 2)
		assert.Equal(t, testCase.exp, found[1])
	}
	for _, testCase := range falseTestCases {
		res := mentionPattern.MatchString(testCase)
		assert.False(t, res, "[%s] should be false", testCase)
	}
}

func TestRegExp_issueNumericPattern(t *testing.T) {
	trueTestCases := []string{
		"#1234",
		"#0",
		"#1234567890987654321",
		"  #12",
		"#12:",
		"ref: #12: msg",
	}
	falseTestCases := []string{
		"# 1234",
		"# 0",
		"# ",
		"#",
		"#ABC",
		"#1A2B",
		"",
		"ABC",
	}

	for _, testCase := range trueTestCases {
		assert.True(t, issueNumericPattern.MatchString(testCase))
	}
	for _, testCase := range falseTestCases {
		assert.False(t, issueNumericPattern.MatchString(testCase))
	}
}

func TestRegExp_issueAlphanumericPattern(t *testing.T) {
	trueTestCases := []string{
		"ABC-1234",
		"A-1",
		"RC-80",
		"ABCDEFGHIJ-1234567890987654321234567890",
		"ABC-123.",
		"(ABC-123)",
		"[ABC-123]",
		"ABC-123:",
	}
	falseTestCases := []string{
		"RC-08",
		"PR-0",
		"ABCDEFGHIJK-1",
		"PR_1",
		"",
		"#ABC",
		"",
		"ABC",
		"GG-",
		"rm-1",
		"/home/gitea/ABC-1234",
		"MY-STRING-ABC-123",
	}

	for _, testCase := range trueTestCases {
		assert.True(t, issueAlphanumericPattern.MatchString(testCase))
	}
	for _, testCase := range falseTestCases {
		assert.False(t, issueAlphanumericPattern.MatchString(testCase))
	}
}
