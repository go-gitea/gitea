// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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
	return RefName(ref.Name).ShortName()
}

// RefGroup returns the group type of the reference
func (ref *Reference) RefGroup() string {
	return RefName(ref.Name).RefGroup()
}

// RefName represents a git reference name
type RefName string

func (ref RefName) IsBranch() bool {
	return strings.HasPrefix(string(ref), BranchPrefix)
}

func (ref RefName) IsTag() bool {
	return strings.HasPrefix(string(ref), TagPrefix)
}

// ShortName returns the short name of the reference name
func (ref RefName) ShortName() string {
	refName := string(ref)
	if strings.HasPrefix(refName, BranchPrefix) {
		return strings.TrimPrefix(refName, BranchPrefix)
	}
	if strings.HasPrefix(refName, TagPrefix) {
		return strings.TrimPrefix(refName, TagPrefix)
	}
	if strings.HasPrefix(refName, RemotePrefix) {
		return strings.TrimPrefix(refName, RemotePrefix)
	}
	if strings.HasPrefix(refName, PullPrefix) && strings.IndexByte(refName[pullLen:], '/') > -1 {
		return refName[pullLen : strings.IndexByte(refName[pullLen:], '/')+pullLen]
	}

	return refName
}

// RefGroup returns the group type of the reference
func (ref RefName) RefGroup() string {
	refName := string(ref)
	if strings.HasPrefix(refName, BranchPrefix) {
		return "heads"
	}
	if strings.HasPrefix(refName, TagPrefix) {
		return "tags"
	}
	if strings.HasPrefix(refName, RemotePrefix) {
		return "remotes"
	}
	if strings.HasPrefix(refName, PullPrefix) && strings.IndexByte(refName[pullLen:], '/') > -1 {
		return "pull"
	}
	return ""
}
