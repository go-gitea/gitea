// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package references

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/mdstripper"
	"code.gitea.io/gitea/modules/setting"
)

var (
	// validNamePattern performs only the most basic validation for user or repository names
	// Repository name should contain only alphanumeric, dash ('-'), underscore ('_') and dot ('.') characters.
	validNamePattern = regexp.MustCompile(`^[a-z0-9_.-]+$`)

	// NOTE: All below regex matching do not perform any extra validation.
	// Thus a link is produced even if the linked entity does not exist.
	// While fast, this is also incorrect and lead to false positives.
	// TODO: fix invalid linking issue

	// mentionPattern matches all mentions in the form of "@user"
	mentionPattern = regexp.MustCompile(`(?:\s|^|\(|\[)(@[0-9a-zA-Z-_]+|@[0-9a-zA-Z-_][0-9a-zA-Z-_.]+[0-9a-zA-Z-_])(?:\s|[:,;.?!]\s|[:,;.?!]?$|\)|\])`)
	// issueNumericPattern matches string that references to a numeric issue, e.g. #1287
	issueNumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([#!][0-9]+)(?:\s|$|\)|\]|:|\.(\s|$))`)
	// issueAlphanumericPattern matches string that references to an alphanumeric issue, e.g. ABC-1234
	issueAlphanumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([A-Z]{1,10}-[1-9][0-9]*)(?:\s|$|\)|\]|:|\.(\s|$))`)
	// crossReferenceIssueNumericPattern matches string that references a numeric issue in a different repository
	// e.g. gogits/gogs#12345
	crossReferenceIssueNumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([0-9a-zA-Z-_\.]+/[0-9a-zA-Z-_\.]+[#!][0-9]+)(?:\s|$|\)|\]|\.(\s|$))`)

	issueCloseKeywordsPat, issueReopenKeywordsPat *regexp.Regexp
	issueKeywordsOnce                             sync.Once

	giteaHostInit sync.Once
	giteaHost     string
)

// XRefAction represents the kind of effect a cross reference has once is resolved
type XRefAction int64

const (
	// XRefActionNone means the cross-reference is simply a comment
	XRefActionNone XRefAction = iota // 0
	// XRefActionCloses means the cross-reference should close an issue if it is resolved
	XRefActionCloses // 1
	// XRefActionReopens means the cross-reference should reopen an issue if it is resolved
	XRefActionReopens // 2
	// XRefActionNeutered means the cross-reference will no longer affect the source
	XRefActionNeutered // 3
)

// IssueReference contains an unverified cross-reference to a local issue or pull request
type IssueReference struct {
	Index  int64
	Owner  string
	Name   string
	Action XRefAction
}

// RenderizableReference contains an unverified cross-reference to with rendering information
// The IsPull member means that a `!num` reference was used instead of `#num`.
// This kind of reference is used to make pulls available when an external issue tracker
// is used. Otherwise, `#` and `!` are completely interchangeable.
type RenderizableReference struct {
	Issue          string
	Owner          string
	Name           string
	IsPull         bool
	RefLocation    *RefSpan
	Action         XRefAction
	ActionLocation *RefSpan
}

type rawReference struct {
	index          int64
	owner          string
	name           string
	isPull         bool
	action         XRefAction
	issue          string
	refLocation    *RefSpan
	actionLocation *RefSpan
}

func rawToIssueReferenceList(reflist []*rawReference) []IssueReference {
	refarr := make([]IssueReference, len(reflist))
	for i, r := range reflist {
		refarr[i] = IssueReference{
			Index:  r.index,
			Owner:  r.owner,
			Name:   r.name,
			Action: r.action,
		}
	}
	return refarr
}

// RefSpan is the position where the reference was found within the parsed text
type RefSpan struct {
	Start int
	End   int
}

func makeKeywordsPat(words []string) *regexp.Regexp {
	acceptedWords := parseKeywords(words)
	if len(acceptedWords) == 0 {
		// Never match
		return nil
	}
	return regexp.MustCompile(`(?i)(?:\s|^|\(|\[)(` + strings.Join(acceptedWords, `|`) + `):? $`)
}

func parseKeywords(words []string) []string {
	acceptedWords := make([]string, 0, 5)
	wordPat := regexp.MustCompile(`^[\pL]+$`)
	for _, word := range words {
		word = strings.ToLower(strings.TrimSpace(word))
		// Accept Unicode letter class runes (a-z, á, à, ä, )
		if wordPat.MatchString(word) {
			acceptedWords = append(acceptedWords, word)
		} else {
			log.Info("Invalid keyword: %s", word)
		}
	}
	return acceptedWords
}

func newKeywords() {
	issueKeywordsOnce.Do(func() {
		// Delay initialization until after the settings module is initialized
		doNewKeywords(setting.Repository.PullRequest.CloseKeywords, setting.Repository.PullRequest.ReopenKeywords)
	})
}

func doNewKeywords(close []string, reopen []string) {
	issueCloseKeywordsPat = makeKeywordsPat(close)
	issueReopenKeywordsPat = makeKeywordsPat(reopen)
}

// getGiteaHostName returns a normalized string with the local host name, with no scheme or port information
func getGiteaHostName() string {
	giteaHostInit.Do(func() {
		if uapp, err := url.Parse(setting.AppURL); err == nil {
			giteaHost = strings.ToLower(uapp.Host)
		} else {
			giteaHost = ""
		}
	})
	return giteaHost
}

// FindAllMentionsMarkdown matches mention patterns in given content and
// returns a list of found unvalidated user names **not including** the @ prefix.
func FindAllMentionsMarkdown(content string) []string {
	bcontent, _ := mdstripper.StripMarkdownBytes([]byte(content))
	locations := FindAllMentionsBytes(bcontent)
	mentions := make([]string, len(locations))
	for i, val := range locations {
		mentions[i] = string(bcontent[val.Start+1 : val.End])
	}
	return mentions
}

// FindAllMentionsBytes matches mention patterns in given content
// and returns a list of locations for the unvalidated user names, including the @ prefix.
func FindAllMentionsBytes(content []byte) []RefSpan {
	mentions := mentionPattern.FindAllSubmatchIndex(content, -1)
	ret := make([]RefSpan, len(mentions))
	for i, val := range mentions {
		ret[i] = RefSpan{Start: val[2], End: val[3]}
	}
	return ret
}

// FindFirstMentionBytes matches the first mention in then given content
// and returns the location of the unvalidated user name, including the @ prefix.
func FindFirstMentionBytes(content []byte) (bool, RefSpan) {
	mention := mentionPattern.FindSubmatchIndex(content)
	if mention == nil {
		return false, RefSpan{}
	}
	return true, RefSpan{Start: mention[2], End: mention[3]}
}

// FindAllIssueReferencesMarkdown strips content from markdown markup
// and returns a list of unvalidated references found in it.
func FindAllIssueReferencesMarkdown(content string) []IssueReference {
	return rawToIssueReferenceList(findAllIssueReferencesMarkdown(content))
}

func findAllIssueReferencesMarkdown(content string) []*rawReference {
	bcontent, links := mdstripper.StripMarkdownBytes([]byte(content))
	return findAllIssueReferencesBytes(bcontent, links)
}

// FindAllIssueReferences returns a list of unvalidated references found in a string.
func FindAllIssueReferences(content string) []IssueReference {
	return rawToIssueReferenceList(findAllIssueReferencesBytes([]byte(content), []string{}))
}

// FindRenderizableReferenceNumeric returns the first unvalidated reference found in a string.
func FindRenderizableReferenceNumeric(content string, prOnly bool) (bool, *RenderizableReference) {
	match := issueNumericPattern.FindStringSubmatchIndex(content)
	if match == nil {
		if match = crossReferenceIssueNumericPattern.FindStringSubmatchIndex(content); match == nil {
			return false, nil
		}
	}
	r := getCrossReference([]byte(content), match[2], match[3], false, prOnly)
	if r == nil {
		return false, nil
	}

	return true, &RenderizableReference{
		Issue:          r.issue,
		Owner:          r.owner,
		Name:           r.name,
		IsPull:         r.isPull,
		RefLocation:    r.refLocation,
		Action:         r.action,
		ActionLocation: r.actionLocation,
	}
}

// FindRenderizableReferenceAlphanumeric returns the first alphanumeric unvalidated references found in a string.
func FindRenderizableReferenceAlphanumeric(content string) (bool, *RenderizableReference) {
	match := issueAlphanumericPattern.FindStringSubmatchIndex(content)
	if match == nil {
		return false, nil
	}

	action, location := findActionKeywords([]byte(content), match[2])

	return true, &RenderizableReference{
		Issue:          string(content[match[2]:match[3]]),
		RefLocation:    &RefSpan{Start: match[2], End: match[3]},
		Action:         action,
		ActionLocation: location,
		IsPull:         false,
	}
}

// FindAllIssueReferencesBytes returns a list of unvalidated references found in a byte slice.
func findAllIssueReferencesBytes(content []byte, links []string) []*rawReference {

	ret := make([]*rawReference, 0, 10)

	matches := issueNumericPattern.FindAllSubmatchIndex(content, -1)
	for _, match := range matches {
		if ref := getCrossReference(content, match[2], match[3], false, false); ref != nil {
			ret = append(ret, ref)
		}
	}

	matches = crossReferenceIssueNumericPattern.FindAllSubmatchIndex(content, -1)
	for _, match := range matches {
		if ref := getCrossReference(content, match[2], match[3], false, false); ref != nil {
			ret = append(ret, ref)
		}
	}

	localhost := getGiteaHostName()
	for _, link := range links {
		if u, err := url.Parse(link); err == nil {
			// Note: we're not attempting to match the URL scheme (http/https)
			host := strings.ToLower(u.Host)
			if host != "" && host != localhost {
				continue
			}
			parts := strings.Split(u.EscapedPath(), "/")
			// /user/repo/issues/3
			if len(parts) != 5 || parts[0] != "" {
				continue
			}
			var sep string
			if parts[3] == "issues" {
				sep = "#"
			} else if parts[3] == "pulls" {
				sep = "!"
			} else {
				continue
			}
			// Note: closing/reopening keywords not supported with URLs
			bytes := []byte(parts[1] + "/" + parts[2] + sep + parts[4])
			if ref := getCrossReference(bytes, 0, len(bytes), true, false); ref != nil {
				ref.refLocation = nil
				ret = append(ret, ref)
			}
		}
	}

	return ret
}

func getCrossReference(content []byte, start, end int, fromLink bool, prOnly bool) *rawReference {
	refid := string(content[start:end])
	sep := strings.IndexAny(refid, "#!")
	if sep < 0 {
		return nil
	}
	isPull := refid[sep] == '!'
	if prOnly && !isPull {
		return nil
	}
	repo := refid[:sep]
	issue := refid[sep+1:]
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
		return &rawReference{
			index:          index,
			action:         action,
			issue:          issue,
			isPull:         isPull,
			refLocation:    &RefSpan{Start: start, End: end},
			actionLocation: location,
		}
	}
	parts := strings.Split(strings.ToLower(repo), "/")
	if len(parts) != 2 {
		return nil
	}
	owner, name := parts[0], parts[1]
	if !validNamePattern.MatchString(owner) || !validNamePattern.MatchString(name) {
		return nil
	}
	action, location := findActionKeywords(content, start)
	return &rawReference{
		index:          index,
		owner:          owner,
		name:           name,
		action:         action,
		issue:          issue,
		isPull:         isPull,
		refLocation:    &RefSpan{Start: start, End: end},
		actionLocation: location,
	}
}

func findActionKeywords(content []byte, start int) (XRefAction, *RefSpan) {
	newKeywords()
	var m []int
	if issueCloseKeywordsPat != nil {
		m = issueCloseKeywordsPat.FindSubmatchIndex(content[:start])
		if m != nil {
			return XRefActionCloses, &RefSpan{Start: m[2], End: m[3]}
		}
	}
	if issueReopenKeywordsPat != nil {
		m = issueReopenKeywordsPat.FindSubmatchIndex(content[:start])
		if m != nil {
			return XRefActionReopens, &RefSpan{Start: m[2], End: m[3]}
		}
	}
	return XRefActionNone, nil
}

// IsXrefActionable returns true if the xref action is actionable (i.e. produces a result when resolved)
func IsXrefActionable(ref *RenderizableReference, extTracker bool, alphaNum bool) bool {
	if extTracker {
		// External issues cannot be automatically closed
		return false
	}
	return ref.Action == XRefActionCloses || ref.Action == XRefActionReopens
}
