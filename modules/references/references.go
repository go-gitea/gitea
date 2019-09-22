// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package references

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
)

var (
	// validNamePattern performs only the most basic validation for user or repository names
	// Repository name should contain only alphanumeric, dash ('-'), underscore ('_') and dot ('.') characters.
	validNamePattern = regexp.MustCompile(`^[a-z0-9_.-]+$`)

	// mentionPattern matches all mentions in the form of "@user"
	mentionPattern = regexp.MustCompile(`(?:\s|^|\(|\[)(@[0-9a-zA-Z-_\.]+)(?:\s|$|\)|\])`)
	// issueNumericPattern matches string that references to a numeric issue, e.g. #1287
	issueNumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)(#[0-9]+)(?:\s|$|\)|\]|:|\.(\s|$))`)
	// issueNumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([\pL]+ )?(#[0-9]+)(?:\s|$|\)|\]|:|\.(\s|$))`)
	// issueNumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([\pL]+ )?(#[0-9]+)(?:\s|$|\)|\]|:|\.(\s|$))`)
	// crossReferenceIssueNumericPattern matches string that references a numeric issue in a different repository
	// e.g. gogits/gogs#12345
	crossReferenceIssueNumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([0-9a-zA-Z-_\.]+/[0-9a-zA-Z-_\.]+#[0-9]+)(?:\s|$|\)|\]|\.(\s|$))`)
	// crossReferenceIssueNumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([\pL]+ )?([0-9a-zA-Z-_\.]+/[0-9a-zA-Z-_\.]+#[0-9]+)(?:\s|$|\)|\]|\.(\s|$))`)

	// Same as GitHub. See
	// https://help.github.com/articles/closing-issues-via-commit-messages
	issueCloseKeywords  = []string{"close", "closes", "closed", "fix", "fixes", "fixed", "resolve", "resolves", "resolved"}
	issueReopenKeywords = []string{"reopen", "reopens", "reopened"}

	issueCloseKeywordsPat, issueReopenKeywordsPat *regexp.Regexp
)

// XRefAction represents the kind of effect a cross reference has once is resolved
type XRefAction int64

const (
	// XRefActionNone means the cross-reference is simply a comment
	XRefActionNone XRefAction = iota	// 0
	// XRefActionCloses means the cross-reference should close an issue if it is resolved
	XRefActionCloses					// 1
	// XRefActionReopens means the cross-reference should reopen an issue if it is resolved
	XRefActionReopens					// 2
	// XRefActionNeutered means the cross-reference will no longer affect the source
	XRefActionNeutered					// 3
)

// RawIssueReference contains information about a cross-reference in the text
type RawIssueReference struct {
	Index           int64
	Owner           string
	Name            string
	Action          XRefAction
	RefLocation     ReferenceLocation
	ActionLocation  ReferenceLocation
}

type ReferenceLocation struct {
	Start	int
	End		int
}

func makeKeywordsPat(keywords []string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(?:\s|^|\(|\[)(` + strings.Join(keywords, `|`) + `):? $`)
}

func init() {
	issueCloseKeywordsPat = makeKeywordsPat(issueCloseKeywords)
	issueReopenKeywordsPat = makeKeywordsPat(issueReopenKeywords)
}

// FindAllMentions matches mention patterns in given content
// and returns a list of found unvalidated user names without @ prefix.
func FindAllMentions(content string) []string {
	bcontent := []byte(content)
	locations := FindAllMentionLocations(bcontent)
	mentions := make([]string, len(locations))
	for i, val := range locations {
		mentions[i] = string(bcontent[val.Start:val.End])
	}
	return mentions
}

// FindAllMentionLocations matches mention patterns in given content
// and returns a list of found unvalidated user names without @ prefix.
func FindAllMentionLocations(content []byte) []ReferenceLocation {
	content, _ = markup.StripMarkdownBytes([]byte(content))
	mentions := mentionPattern.FindAllSubmatchIndex(content, -1)
	ret := make([]ReferenceLocation, len(mentions))
	for i, val := range mentions {
		ret[i] = ReferenceLocation{Start: val[2]+1, End: val[3]}
	}
	return ret
}

// FindAllIssueReferencesMarkdown strips content from markdown markup
// and returns a list of unvalidated references found in it.
func FindAllIssueReferencesMarkdown(content string) []*RawIssueReference {
	bcontent, links := markup.StripMarkdownBytes([]byte(content))
	return FindAllIssueReferencesBytes(bcontent, links)
}

// FindAllIssueReferences returns a list of unvalidated references found in a string.
func FindAllIssueReferences(content string) []*RawIssueReference {
	return FindAllIssueReferencesBytes([]byte(content), []string{})
}

// FindAllIssueReferencesBytes returns a list of unvalidated references found in a byte slice.
func FindAllIssueReferencesBytes(content []byte, links []string) []*RawIssueReference {

	ret := make([]*RawIssueReference, 0, 10)

	matches := issueNumericPattern.FindAllSubmatchIndex(content, -1)
	for _, match := range matches {
		if ref := getCrossReference(content, match[2], match[3], false); ref != nil {
			ret = append(ret, ref)
		}
	}

	matches = crossReferenceIssueNumericPattern.FindAllSubmatchIndex(content, -1)
	for _, match := range matches {
		if ref := getCrossReference(content, match[2], match[3], false); ref != nil {
			ret = append(ret, ref)
		}
	}

	var giteahost string
	if uapp, err := url.Parse(setting.AppURL); err == nil {
		giteahost = strings.ToLower(uapp.Host)
	}

	for _, link := range links {
		if u, err := url.Parse(link); err == nil {
			// Note: we're not attempting to match the URL scheme (http/https)
			host := strings.ToLower(u.Host)
			if host != "" && host != giteahost {
				continue
			}
			parts := strings.Split(u.EscapedPath(), "/")
			// /user/repo/issues/3
			if len(parts) != 5 || parts[0] != "" {
				continue
			}
			if parts[3] != "issues" && parts[3] != "pulls" {
				continue
			}
			// Note: closing/reopening keywords not supported with URLs
			bytes := []byte(parts[1]+"/"+parts[2]+"#"+parts[4])
			if ref := getCrossReference(bytes, 0, len(bytes), true); ref != nil {
				ref.RefLocation = ReferenceLocation{}
				ret = append(ret, ref)
			}
		}
	}

	return ret
}

func getCrossReference(content []byte, start, end int, fromLink bool) *RawIssueReference {
	s := string(content[start:end])
	parts := strings.Split(s, "#")
	if len(parts) != 2 {
		return nil
	}
	repo, issue := parts[0], parts[1]
	index, err := strconv.ParseInt(issue, 10, 64)
	if err != nil {
		return nil
	}
	if repo == "" {
		if fromLink {
			// Markdown links must specify owner/repo
			return nil
		}
		action, location := findActionKeywords(content, start)
		return &RawIssueReference{Index: index, RefLocation: ReferenceLocation{Start: start, End: end},
								  Action: action, ActionLocation: location}
	}
	parts = strings.Split(strings.ToLower(repo), "/")
	if len(parts) != 2 {
		return nil
	}
	owner, name := parts[0], parts[1]
	if !validNamePattern.MatchString(owner) || !validNamePattern.MatchString(name) {
		return nil
	}
	action, location := findActionKeywords(content, start)
	return &RawIssueReference{Index: index, Owner: owner, Name: name,
							  RefLocation: ReferenceLocation{Start: start, End: end},
							  Action: action, ActionLocation: location}
}

func findActionKeywords(content []byte, start int) (XRefAction, ReferenceLocation) {
	m := issueCloseKeywordsPat.FindSubmatchIndex(content[:start]);
	if m != nil {
		return XRefActionCloses, ReferenceLocation{Start: m[2], End: m[3]}
	}
	m = issueReopenKeywordsPat.FindSubmatchIndex(content[:start]);
	if m != nil {
		return XRefActionReopens, ReferenceLocation{Start: m[2], End: m[3]}
	}
	return XRefActionNone, ReferenceLocation{}
}
