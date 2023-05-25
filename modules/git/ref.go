// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/util"
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

// RefName represents a full git reference name
type RefName string

func RefNameFromBranch(shortName string) RefName {
	return RefName(BranchPrefix + shortName)
}

func RefNameFromTag(shortName string) RefName {
	return RefName(TagPrefix + shortName)
}

func (ref RefName) String() string {
	return string(ref)
}

func (ref RefName) IsBranch() bool {
	return strings.HasPrefix(string(ref), BranchPrefix)
}

func (ref RefName) IsTag() bool {
	return strings.HasPrefix(string(ref), TagPrefix)
}

func (ref RefName) IsRemote() bool {
	return strings.HasPrefix(string(ref), RemotePrefix)
}

func (ref RefName) IsPull() bool {
	return strings.HasPrefix(string(ref), PullPrefix)
}

func (ref RefName) IsFor() bool {
	return strings.HasPrefix(string(ref), ForPrefix)
}

// TagName returns simple tag name if it's an operation to a tag
func (ref RefName) TagName() string {
	if ref.IsTag() {
		return ref.ShortName()
	}
	return ""
}

// BranchName returns simple branch name if it's an operation to branch
func (ref RefName) BranchName() string {
	if ref.IsBranch() {
		return ref.ShortName()
	}
	return ""
}

func (ref RefName) ForBranchName() string {
	refName := string(ref)
	if strings.HasPrefix(refName, ForPrefix) {
		return strings.TrimPrefix(refName, ForPrefix)
	}
	return ""
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
	if strings.HasPrefix(refName, ForPrefix) {
		return strings.TrimPrefix(refName, ForPrefix)
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
	if strings.HasPrefix(refName, ForPrefix) {
		return "for"
	}
	return ""
}

// RefURL returns the absolute URL for a ref in a repository
func RefURL(repoURL, ref string) string {
	refFullName := RefName(ref)
	refName := util.PathEscapeSegments(refFullName.ShortName())
	switch {
	case refFullName.IsBranch():
		return repoURL + "/src/branch/" + refName
	case refFullName.IsTag():
		return repoURL + "/src/tag/" + refName
	case !IsValidSHAPattern(ref):
		// assume they mean a branch
		return repoURL + "/src/branch/" + refName
	default:
		return repoURL + "/src/commit/" + refName
	}
}
