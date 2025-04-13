// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package references

import (
	"regexp"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

type testFixture struct {
	input    string
	expected []testResult
}

type testResult struct {
	Index          int64
	Owner          string
	Name           string
	Issue          string
	IsPull         bool
	Action         XRefAction
	RefLocation    *RefSpan
	ActionLocation *RefSpan
	TimeLog        string
}

func TestConvertFullHTMLReferencesToShortRefs(t *testing.T) {
	re := regexp.MustCompile(`(\s|^|\(|\[)` +
		regexp.QuoteMeta("https://ourgitea.com/git/") +
		`([0-9a-zA-Z-_\.]+/[0-9a-zA-Z-_\.]+)/` +
		`((?:issues)|(?:pulls))/([0-9]+)(?:\s|$|\)|\]|[:;,.?!]\s|[:;,.?!]$)`)
	test := `this is a https://ourgitea.com/git/owner/repo/issues/123456789, foo
https://ourgitea.com/git/owner/repo/pulls/123456789
  And https://ourgitea.com/git/owner/repo/pulls/123
`
	expect := `this is a owner/repo#123456789, foo
owner/repo!123456789
  And owner/repo!123
`

	contentBytes := []byte(test)
	convertFullHTMLReferencesToShortRefs(re, &contentBytes)
	result := string(contentBytes)
	assert.Equal(t, expect, result)
}

func TestFindAllIssueReferences(t *testing.T) {
	fixtures := []testFixture{
		{
			"Simply closes: #29 yes",
			[]testResult{
				{29, "", "", "29", false, XRefActionCloses, &RefSpan{Start: 15, End: 18}, &RefSpan{Start: 7, End: 13}, ""},
			},
		},
		{
			"Simply closes: !29 yes",
			[]testResult{
				{29, "", "", "29", true, XRefActionCloses, &RefSpan{Start: 15, End: 18}, &RefSpan{Start: 7, End: 13}, ""},
			},
		},
		{
			" #124 yes, this is a reference.",
			[]testResult{
				{124, "", "", "124", false, XRefActionNone, &RefSpan{Start: 0, End: 4}, nil, ""},
			},
		},
		{
			"```\nThis is a code block.\n#723 no, it's a code block.```",
			[]testResult{},
		},
		{
			"This `#724` no, it's inline code.",
			[]testResult{},
		},
		{
			"This org3/repo4#200 yes.",
			[]testResult{
				{200, "org3", "repo4", "200", false, XRefActionNone, &RefSpan{Start: 5, End: 19}, nil, ""},
			},
		},
		{
			"This org3/repo4!200 yes.",
			[]testResult{
				{200, "org3", "repo4", "200", true, XRefActionNone, &RefSpan{Start: 5, End: 19}, nil, ""},
			},
		},
		{
			"This [one](#919) no, this is a URL fragment.",
			[]testResult{},
		},
		{
			"This [two](/user2/repo1/issues/921) yes.",
			[]testResult{
				{921, "user2", "repo1", "921", false, XRefActionNone, nil, nil, ""},
			},
		},
		{
			"This [three](/user2/repo1/pulls/922) yes.",
			[]testResult{
				{922, "user2", "repo1", "922", true, XRefActionNone, nil, nil, ""},
			},
		},
		{
			"This [four](http://gitea.com:3000/org3/repo4/issues/203) yes.",
			[]testResult{
				{203, "org3", "repo4", "203", false, XRefActionNone, nil, nil, ""},
			},
		},
		{
			"This [five](http://github.com/org3/repo4/issues/204) no.",
			[]testResult{},
		},
		{
			"This http://gitea.com:3000/user4/repo5/201 no, bad URL.",
			[]testResult{},
		},
		{
			"This http://gitea.com:3000/user4/repo5/pulls/202 yes.",
			[]testResult{
				{202, "user4", "repo5", "202", true, XRefActionNone, nil, nil, ""},
			},
		},
		{
			"This http://gitea.com:3000/user4/repo5/pulls/202 yes. http://gitea.com:3000/user4/repo5/pulls/203 no",
			[]testResult{
				{202, "user4", "repo5", "202", true, XRefActionNone, nil, nil, ""},
				{203, "user4", "repo5", "203", true, XRefActionNone, nil, nil, ""},
			},
		},
		{
			"This http://GiTeA.COM:3000/user4/repo6/pulls/205 yes.",
			[]testResult{
				{205, "user4", "repo6", "205", true, XRefActionNone, nil, nil, ""},
			},
		},
		{
			"Reopens #15 yes",
			[]testResult{
				{15, "", "", "15", false, XRefActionReopens, &RefSpan{Start: 8, End: 11}, &RefSpan{Start: 0, End: 7}, ""},
			},
		},
		{
			"This closes #20 for you yes",
			[]testResult{
				{20, "", "", "20", false, XRefActionCloses, &RefSpan{Start: 12, End: 15}, &RefSpan{Start: 5, End: 11}, ""},
			},
		},
		{
			"Do you fix org6/repo6#300 ? yes",
			[]testResult{
				{300, "org6", "repo6", "300", false, XRefActionCloses, &RefSpan{Start: 11, End: 25}, &RefSpan{Start: 7, End: 10}, ""},
			},
		},
		{
			"For 999 #1235 no keyword, but yes",
			[]testResult{
				{1235, "", "", "1235", false, XRefActionNone, &RefSpan{Start: 8, End: 13}, nil, ""},
			},
		},
		{
			"For [!123] yes",
			[]testResult{
				{123, "", "", "123", true, XRefActionNone, &RefSpan{Start: 5, End: 9}, nil, ""},
			},
		},
		{
			"For (#345) yes",
			[]testResult{
				{345, "", "", "345", false, XRefActionNone, &RefSpan{Start: 5, End: 9}, nil, ""},
			},
		},
		{
			"For #22,#23 no, neither #28:#29 or !30!31#32;33 should",
			[]testResult{},
		},
		{
			"For #24, and #25. yes; also #26; #27? #28! and #29: should",
			[]testResult{
				{24, "", "", "24", false, XRefActionNone, &RefSpan{Start: 4, End: 7}, nil, ""},
				{25, "", "", "25", false, XRefActionNone, &RefSpan{Start: 13, End: 16}, nil, ""},
				{26, "", "", "26", false, XRefActionNone, &RefSpan{Start: 28, End: 31}, nil, ""},
				{27, "", "", "27", false, XRefActionNone, &RefSpan{Start: 33, End: 36}, nil, ""},
				{28, "", "", "28", false, XRefActionNone, &RefSpan{Start: 38, End: 41}, nil, ""},
				{29, "", "", "29", false, XRefActionNone, &RefSpan{Start: 47, End: 50}, nil, ""},
			},
		},
		{
			"This org3/repo4#200, yes.",
			[]testResult{
				{200, "org3", "repo4", "200", false, XRefActionNone, &RefSpan{Start: 5, End: 19}, nil, ""},
			},
		},
		{
			"Merge pull request '#12345 My fix for a bug' (!1337) from feature-branch into main",
			[]testResult{
				{12345, "", "", "12345", false, XRefActionNone, &RefSpan{Start: 20, End: 26}, nil, ""},
				{1337, "", "", "1337", true, XRefActionNone, &RefSpan{Start: 46, End: 51}, nil, ""},
			},
		},
		{
			"Which abc. #9434 same as above",
			[]testResult{
				{9434, "", "", "9434", false, XRefActionNone, &RefSpan{Start: 11, End: 16}, nil, ""},
			},
		},
		{
			"This closes #600 and reopens #599",
			[]testResult{
				{600, "", "", "600", false, XRefActionCloses, &RefSpan{Start: 12, End: 16}, &RefSpan{Start: 5, End: 11}, ""},
				{599, "", "", "599", false, XRefActionReopens, &RefSpan{Start: 29, End: 33}, &RefSpan{Start: 21, End: 28}, ""},
			},
		},
		{
			"This fixes #100 spent @40m and reopens #101, also fixes #102 spent @4h15m",
			[]testResult{
				{100, "", "", "100", false, XRefActionCloses, &RefSpan{Start: 11, End: 15}, &RefSpan{Start: 5, End: 10}, "40m"},
				{101, "", "", "101", false, XRefActionReopens, &RefSpan{Start: 39, End: 43}, &RefSpan{Start: 31, End: 38}, ""},
				{102, "", "", "102", false, XRefActionCloses, &RefSpan{Start: 56, End: 60}, &RefSpan{Start: 50, End: 55}, "4h15m"},
			},
		},
	}

	testFixtures(t, fixtures, "default")

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
		ref := FindRenderizableReferenceAlphanumeric(fixture.input)
		if fixture.issue == "" {
			assert.Nil(t, ref, "Failed to parse: {%s}", fixture.input)
		} else {
			assert.Equal(t, fixture.issue, ref.Issue, "Failed to parse: {%s}", fixture.input)
			assert.Equal(t, fixture.refLocation, ref.RefLocation, "Failed to parse: {%s}", fixture.input)
			assert.Equal(t, fixture.action, ref.Action, "Failed to parse: {%s}", fixture.input)
			assert.Equal(t, fixture.actionLocation, ref.ActionLocation, "Failed to parse: {%s}", fixture.input)
		}
	}
}

func testFixtures(t *testing.T, fixtures []testFixture, context string) {
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
				isPull:         e.IsPull,
				action:         e.Action,
				issue:          e.Issue,
				refLocation:    e.RefLocation,
				actionLocation: e.ActionLocation,
				timeLog:        e.TimeLog,
			}
		}
		expref := rawToIssueReferenceList(expraw)
		refs := FindAllIssueReferencesMarkdown(fixture.input)
		assert.Equal(t, expref, refs, "[%s] Failed to parse: {%s}", context, fixture.input)
		rawrefs := findAllIssueReferencesMarkdown(fixture.input)
		assert.Equal(t, expraw, rawrefs, "[%s] Failed to parse: {%s}", context, fixture.input)
	}

	// Restore for other tests that may rely on the original value
	setting.AppURL = prevURL
}

func TestFindAllMentions(t *testing.T) {
	res := FindAllMentionsBytes([]byte("@tasha, @mike; @lucy: @john"))
	assert.Equal(t, []RefSpan{
		{Start: 0, End: 6},
		{Start: 8, End: 13},
		{Start: 15, End: 20},
		{Start: 22, End: 27},
	}, res)
}

func TestFindRenderizableCommitCrossReference(t *testing.T) {
	cases := []struct {
		Input    string
		Expected *RenderizableReference
	}{
		{
			Input:    "",
			Expected: nil,
		},
		{
			Input:    "test",
			Expected: nil,
		},
		{
			Input:    "go-gitea/gitea@test",
			Expected: nil,
		},
		{
			Input:    "go-gitea/gitea@ab1234",
			Expected: nil,
		},
		{
			Input: "go-gitea/gitea@abcd1234",
			Expected: &RenderizableReference{
				Owner:       "go-gitea",
				Name:        "gitea",
				CommitSha:   "abcd1234",
				RefLocation: &RefSpan{Start: 0, End: 23},
			},
		},
		{
			Input: "go-gitea/gitea@abcd1234abcd1234abcd1234abcd1234abcd1234",
			Expected: &RenderizableReference{
				Owner:       "go-gitea",
				Name:        "gitea",
				CommitSha:   "abcd1234abcd1234abcd1234abcd1234abcd1234",
				RefLocation: &RefSpan{Start: 0, End: 55},
			},
		},
		{
			Input:    "go-gitea/gitea@abcd1234abcd1234abcd1234abcd1234abcd12341234512345123451234512345", // longer than 64 characters
			Expected: nil,
		},
		{
			Input: "test go-gitea/gitea@abcd1234 test",
			Expected: &RenderizableReference{
				Owner:       "go-gitea",
				Name:        "gitea",
				CommitSha:   "abcd1234",
				RefLocation: &RefSpan{Start: 5, End: 28},
			},
		},
	}

	for _, c := range cases {
		found, ref := FindRenderizableCommitCrossReference(c.Input)
		assert.Equal(t, ref != nil, found)
		assert.Equal(t, c.Expected, ref)
	}
}

func TestRegExp_mentionPattern(t *testing.T) {
	trueTestCases := []struct {
		pat string
		exp string
	}{
		{"@User", "@User"},
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
		{"@gitea/team1;", "@gitea/team1"},
		{"@user's idea", "@user"},
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
		"@gitea/team1/more",
	}

	for _, testCase := range trueTestCases {
		found := mentionPattern.FindStringSubmatch(testCase.pat)
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
		"\"#1234\"",
		"'#1234'",
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
		"\"ABC-123\"",
		"'ABC-123'",
		"ABC-123, unknown PR",
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

func TestCustomizeCloseKeywords(t *testing.T) {
	fixtures := []testFixture{
		{
			"Simplemente cierra: #29 yes",
			[]testResult{
				{29, "", "", "29", false, XRefActionCloses, &RefSpan{Start: 20, End: 23}, &RefSpan{Start: 12, End: 18}, ""},
			},
		},
		{
			"Closes: #123 no, this English.",
			[]testResult{
				{123, "", "", "123", false, XRefActionNone, &RefSpan{Start: 8, End: 12}, nil, ""},
			},
		},
		{
			"Cerró org6/repo6#300 yes",
			[]testResult{
				{300, "org6", "repo6", "300", false, XRefActionCloses, &RefSpan{Start: 7, End: 21}, &RefSpan{Start: 0, End: 6}, ""},
			},
		},
		{
			"Reabre org3/repo4#200 yes",
			[]testResult{
				{200, "org3", "repo4", "200", false, XRefActionReopens, &RefSpan{Start: 7, End: 21}, &RefSpan{Start: 0, End: 6}, ""},
			},
		},
	}

	issueKeywordsOnce.Do(func() {})

	doNewKeywords([]string{"cierra", "cerró"}, []string{"reabre"})
	testFixtures(t, fixtures, "spanish")

	// Restore default settings
	doNewKeywords(setting.Repository.PullRequest.CloseKeywords, setting.Repository.PullRequest.ReopenKeywords)
}

func TestParseCloseKeywords(t *testing.T) {
	// Test parsing of CloseKeywords and ReopenKeywords
	assert.Empty(t, parseKeywords([]string{""}))
	assert.Len(t, parseKeywords([]string{"  aa  ", " bb  ", "99", "#", "", "this is", "cc"}), 3)

	for _, test := range []struct {
		pattern  string
		match    string
		expected string
	}{
		{"close", "This PR will close ", "close"},
		{"cerró", "cerró ", "cerró"},
		{"cerró", "AQUÍ SE CERRÓ: ", "CERRÓ"},
		{"закрывается", "закрывается ", "закрывается"},
		{"κλείνει", "κλείνει: ", "κλείνει"},
		{"关闭", "关闭 ", "关闭"},
		{"閉じます", "閉じます ", "閉じます"},
		{",$!", "", ""},
		{"1234", "", ""},
	} {
		// The pattern only needs to match the part that precedes the reference.
		// getCrossReference() takes care of finding the reference itself.
		pat := makeKeywordsPat([]string{test.pattern})
		if test.expected == "" {
			assert.Nil(t, pat)
		} else {
			assert.NotNil(t, pat)
			res := pat.FindAllStringSubmatch(test.match, -1)
			assert.Len(t, res, 1)
			assert.Len(t, res[0], 2)
			assert.Equal(t, test.expected, res[0][1])
		}
	}
}
