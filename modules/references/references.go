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
	issueNumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([\pL]+ )?(#[0-9]+)(?:\s|$|\)|\]|:|\.(\s|$))`)
	// crossReferenceIssueNumericPattern matches string that references a numeric issue in a different repository
	// e.g. gogits/gogs#12345
	crossReferenceIssueNumericPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([\pL]+ )?([0-9a-zA-Z-_\.]+/[0-9a-zA-Z-_\.]+#[0-9]+)(?:\s|$|\)|\]|\.(\s|$))`)
)

// RawIssueReference contains information about a cross-reference in the text
type RawIssueReference struct {
	Index   int64
	Owner   string
	Name    string
	Keyword string
}

// FindAllMentions matches mention patterns in given content
// and returns a list of found unvalidated user names without @ prefix.
func FindAllMentions(content string) []string {
	content, _ = markup.StripMarkdown([]byte(content))
	mentions := mentionPattern.FindAllStringSubmatch(content, -1)
	ret := make([]string, len(mentions))
	for i, val := range mentions {
		ret[i] = val[1][1:]
	}
	return ret
}

// FindAllIssueReferences matches issue reference patterns in given content
// and returns a list of unvalidated references.
func FindAllIssueReferences(content string) []*RawIssueReference {

	content, links := markup.StripMarkdown([]byte(content))
	ret := make([]*RawIssueReference, 0, 10)

	matches := issueNumericPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if ref := getCrossReference(match[2], strings.TrimSuffix(match[1], " "), false); ref != nil {
			ret = append(ret, ref)
		}
	}

	matches = crossReferenceIssueNumericPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if ref := getCrossReference(match[2], strings.TrimSuffix(match[1], " "), false); ref != nil {
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
			if ref := getCrossReference(parts[1]+"/"+parts[2]+"#"+parts[4], "", true); ref != nil {
				ret = append(ret, ref)
			}
		}
	}
	return ret
}

func getCrossReference(s string, keyword string, fromLink bool) *RawIssueReference {
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
		return &RawIssueReference{Index: index, Keyword: keyword}
	}
	parts = strings.Split(strings.ToLower(repo), "/")
	if len(parts) != 2 {
		return nil
	}
	owner, name := parts[0], parts[1]
	if !validNamePattern.MatchString(owner) || !validNamePattern.MatchString(name) {
		return nil
	}
	return &RawIssueReference{Index: index, Owner: owner, Name: name, Keyword: keyword}
}
