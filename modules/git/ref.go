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
	Object ObjectID // The id of this commit object
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

// ForPrefix special ref to create a pull request: refs/for/<target-branch>/<topic-branch>
// or refs/for/<targe-branch> -o topic='<topic-branch>'
const ForPrefix = "refs/for/"

// TODO: /refs/for-review for suggest change interface

// RefName represents a full git reference name
type RefName string

func RefNameFromBranch(shortName string) RefName {
	return RefName(BranchPrefix + shortName)
}

func RefNameFromTag(shortName string) RefName {
	return RefName(TagPrefix + shortName)
}

func RefNameFromCommit(shortName string) RefName {
	return RefName(shortName)
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
	return strings.HasPrefix(string(ref), PullPrefix) && strings.IndexByte(string(ref)[len(PullPrefix):], '/') > -1
}

func (ref RefName) IsFor() bool {
	return strings.HasPrefix(string(ref), ForPrefix)
}

func (ref RefName) nameWithoutPrefix(prefix string) string {
	if strings.HasPrefix(string(ref), prefix) {
		return strings.TrimPrefix(string(ref), prefix)
	}
	return ""
}

// TagName returns simple tag name if it's an operation to a tag
func (ref RefName) TagName() string {
	return ref.nameWithoutPrefix(TagPrefix)
}

// BranchName returns simple branch name if it's an operation to branch
func (ref RefName) BranchName() string {
	return ref.nameWithoutPrefix(BranchPrefix)
}

// PullName returns the pull request name part of refs like refs/pull/<pull_name>/head
func (ref RefName) PullName() string {
	refName := string(ref)
	lastIdx := strings.LastIndexByte(refName[len(PullPrefix):], '/')
	if strings.HasPrefix(refName, PullPrefix) && lastIdx > -1 {
		return refName[len(PullPrefix) : lastIdx+len(PullPrefix)]
	}
	return ""
}

// ForBranchName returns the branch name part of refs like refs/for/<branch_name>
func (ref RefName) ForBranchName() string {
	return ref.nameWithoutPrefix(ForPrefix)
}

func (ref RefName) RemoteName() string {
	return ref.nameWithoutPrefix(RemotePrefix)
}

// ShortName returns the short name of the reference name
func (ref RefName) ShortName() string {
	if ref.IsBranch() {
		return ref.BranchName()
	}
	if ref.IsTag() {
		return ref.TagName()
	}
	if ref.IsRemote() {
		return ref.RemoteName()
	}
	if ref.IsPull() {
		return ref.PullName()
	}
	if ref.IsFor() {
		return ref.ForBranchName()
	}
	return string(ref) // usually it is a commit ID
}

// RefGroup returns the group type of the reference
// Using the name of the directory under .git/refs
func (ref RefName) RefGroup() string {
	if ref.IsBranch() {
		return "heads"
	}
	if ref.IsTag() {
		return "tags"
	}
	if ref.IsRemote() {
		return "remotes"
	}
	if ref.IsPull() {
		return "pull"
	}
	if ref.IsFor() {
		return "for"
	}
	return ""
}

// RefType is a simple ref type of the reference, it is used for UI and webhooks
type RefType string

const (
	RefTypeBranch RefType = "branch"
	RefTypeTag    RefType = "tag"
	RefTypeCommit RefType = "commit"
)

// RefType returns the simple ref type of the reference, e.g. branch, tag
// It's different from RefGroup, which is using the name of the directory under .git/refs
func (ref RefName) RefType() RefType {
	switch {
	case ref.IsBranch():
		return RefTypeBranch
	case ref.IsTag():
		return RefTypeTag
	case IsStringLikelyCommitID(nil, string(ref), 6):
		return RefTypeCommit
	}
	return ""
}

// RefWebLinkPath returns a path for the reference that can be used in a web link:
// * "branch/<branch_name>"
// * "tag/<tag_name>"
// * "commit/<commit_id>"
// It returns an empty string if the reference is not a branch, tag or commit.
func (ref RefName) RefWebLinkPath() string {
	refType := ref.RefType()
	if refType == "" {
		return ""
	}
	return string(refType) + "/" + util.PathEscapeSegments(ref.ShortName())
}
