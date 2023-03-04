// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"regexp"
	"strings"
)

const (
	// RemotePrefix is the base directory of the remotes information of git.
	RemotePrefix = "refs/remotes/"
	// PullPrefix is the base directory of the pull information of git.
	PullPrefix = "refs/pull/"

	pullLen = len(PullPrefix)
)

// refNamePatternInvalid is regular expression with unallowed characters in git reference name
// They cannot have ASCII control characters (i.e. bytes whose values are lower than \040, or \177 DEL), space, tilde ~, caret ^, or colon : anywhere.
// They cannot have question-mark ?, asterisk *, or open bracket [ anywhere
var refNamePatternInvalid = regexp.MustCompile(
	`[\000-\037\177 \\~^:?*[]|` + // No absolutely invalid characters
		`(?:^[/.])|` + // Not HasPrefix("/") or "."
		`(?:/\.)|` + // no "/."
		`(?:\.lock$)|(?:\.lock/)|` + // No ".lock/"" or ".lock" at the end
		`(?:\.\.)|` + // no ".." anywhere
		`(?://)|` + // no "//" anywhere
		`(?:@{)|` + // no "@{"
		`(?:[/.]$)|` + // no terminal '/' or '.'
		`(?:^@$)`) // Not "@"

// IsValidRefPattern ensures that the provided string could be a valid reference
func IsValidRefPattern(name string) bool {
	return !refNamePatternInvalid.MatchString(name)
}

func SanitizeRefPattern(name string) string {
	return refNamePatternInvalid.ReplaceAllString(name, "_")
}

// Reference represents a Git ref.
type Reference struct {
	Name   string
	repo   *Repository
	Object SHA1 // The id of this commit object
	Type   string
}

// Commit return the commit of the reference
func (ref *Reference) Commit() (*Commit, error) {
	return ref.repo.getCommit(ref.Object)
}

// ShortName returns the short name of the reference
func (ref *Reference) ShortName() string {
	if ref == nil {
		return ""
	}
	if strings.HasPrefix(ref.Name, BranchPrefix) {
		return strings.TrimPrefix(ref.Name, BranchPrefix)
	}
	if strings.HasPrefix(ref.Name, TagPrefix) {
		return strings.TrimPrefix(ref.Name, TagPrefix)
	}
	if strings.HasPrefix(ref.Name, RemotePrefix) {
		return strings.TrimPrefix(ref.Name, RemotePrefix)
	}
	if strings.HasPrefix(ref.Name, PullPrefix) && strings.IndexByte(ref.Name[pullLen:], '/') > -1 {
		return ref.Name[pullLen : strings.IndexByte(ref.Name[pullLen:], '/')+pullLen]
	}

	return ref.Name
}

// RefGroup returns the group type of the reference
func (ref *Reference) RefGroup() string {
	if ref == nil {
		return ""
	}
	if strings.HasPrefix(ref.Name, BranchPrefix) {
		return "heads"
	}
	if strings.HasPrefix(ref.Name, TagPrefix) {
		return "tags"
	}
	if strings.HasPrefix(ref.Name, RemotePrefix) {
		return "remotes"
	}
	if strings.HasPrefix(ref.Name, PullPrefix) && strings.IndexByte(ref.Name[pullLen:], '/') > -1 {
		return "pull"
	}
	return ""
}
