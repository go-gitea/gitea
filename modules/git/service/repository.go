// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import (
	"io"
	"time"
)

// Repository represents a git repository
type Repository interface {
	io.Closer

	// Path is the filesystem path for the repository
	Path() string

	//  _
	// |_) |  _  |_
	// |_) | (_) |_)
	//

	// GetBlob finds the blob object in the repository.
	GetBlob(idStr string) (Blob, error)

	//  _
	// |_) ._  _. ._   _ |_
	// |_) |  (_| | | (_ | |
	//

	// IsBranchExist returns true if given branch exists in current repository.
	IsBranchExist(name string) bool

	// GetBranches returns all branches of the repository.
	GetBranches() ([]string, error)

	// SetDefaultBranch sets default branch of repository.
	SetDefaultBranch(name string) error

	// GetDefaultBranch gets default branch of repository.
	GetDefaultBranch() (string, error)

	// GetHEADBranch returns corresponding branch of HEAD.
	GetHEADBranch() (Branch, error)

	// GetBranch returns a branch by it's name
	GetBranch(branch string) (Branch, error)

	// DeleteBranch delete a branch by name on repository.
	DeleteBranch(name string, opts DeleteBranchOptions) error

	// CreateBranch create a new branch
	CreateBranch(branch, oldbranchOrCommit string) error

	//  _
	// |_)  _  ._ _   _  _|_  _
	// | \ (/_ | | | (_)  |_ (/_
	//

	// AddRemote adds a new remote to repository.
	AddRemote(name, url string, fetch bool) error

	// RemoveRemote removes a remote from repository.
	RemoveRemote(name string) error

	//	_
	// /   _  ._ _  ._ _  o _|_
	// \_ (_) | | | | | | |  |_
	//

	// GetRefCommitID returns the last commit ID string of given reference (branch or tag).
	GetRefCommitID(name string) (string, error)

	// IsCommitExist returns true if given commit exists in current repository.
	IsCommitExist(name string) bool

	// GetBranchCommitID returns last commit ID string of given branch.
	GetBranchCommitID(name string) (string, error)

	// GetTagCommitID returns last commit ID string of given tag.
	GetTagCommitID(name string) (string, error)

	// ConvertToSHA1 returns a Hash object from a potential ID string
	ConvertToSHA1(commitID string) (Hash, error)

	// GetCommit returns commit object of by ID string.
	GetCommit(commitID string) (Commit, error)

	// GetBranchCommit returns the last commit of given branch.
	GetBranchCommit(name string) (Commit, error)

	// GetTagCommit get the commit of the specific tag via name
	GetTagCommit(name string) (Commit, error)

	// IsEmpty Check if repository is empty.
	IsEmpty() (bool, error)

	//  _
	// /   _  ._ _  ._   _. ._  _
	// \_ (_) | | | |_) (_| |  (/_
	//              |

	// GetMergeBase checks and returns merge base of two branches and the reference used as base.
	GetMergeBase(tmpRemote string, base, head string) (string, string, error)

	// GetCompareInfo generates and returns compare information between base and head branches of repositories.
	GetCompareInfo(basePath, baseBranch, headBranch string) (_ *CompareInfo, err error)

	// GetDiffNumChangedFiles counts the number of changed files
	// This is substantially quicker than shortstat but...
	GetDiffNumChangedFiles(base, head string) (int, error)

	// GetDiffShortStat counts number of changed files, number of additions and deletions
	GetDiffShortStat(base, head string) (numFiles, totalAdditions, totalDeletions int, err error)

	// GetDiffOrPatch generates either diff or formatted patch data between given revisions
	GetDiffOrPatch(base, head string, w io.Writer, formatted bool) error

	// GetDiff generates and returns patch data between given revisions.
	GetDiff(base, head string, w io.Writer) error

	// GetPatch generates and returns format-patch data between given revisions.
	GetPatch(base, head string, w io.Writer) error

	// GetDiffFromMergeBase generates and return patch data from merge base to head
	GetDiffFromMergeBase(base, head string, w io.Writer) error

	//  __  _   __
	// /__ |_) /__
	// \_| |   \_|
	//

	// GetDefaultPublicGPGKey will return and cache the default public GPG settings for this repository
	GetDefaultPublicGPGKey(forceUpdate bool) (*GPGSettings, error)

	//                                 __
	// |   _. ._   _       _.  _   _  (_  _|_  _. _|_  _
	// |_ (_| | | (_| |_| (_| (_| (/_ __)  |_ (_|  |_ _>
	// 					   _|          _|

	// GetLanguageStats calculates language stats for git repository at specified commit
	GetLanguageStats(commitID string) (map[string]int64, error)

	// GetCodeActivityStats returns code statistics for acitivity page
	GetCodeActivityStats(fromTime time.Time, branch string) (*CodeActivityStats, error)

	//
	// |_|  _.  _ |_
	// | | (_| _> | |
	//

	// HashObject takes a reader and returns SHA1 hash for that reader
	HashObject(reader io.Reader) (Hash, error)

	// GetRefType gets the type of the ref based on the string
	GetRefType(ref string) ObjectType

	// GetRefsFiltered returns all references of the repository that matches patterm exactly or starting with.
	GetRefsFiltered(pattern string) ([]Reference, error)

	// GetRefs returns all references of the repository.
	GetRefs() ([]Reference, error)

	// ___
	//  |   _.  _
	//  |  (_| (_|
	//          _|
	// IsTagExist returns true if given tag exists in the repository.
	IsTagExist(name string) bool

	// GetTags returns all tags of the repository.
	GetTags() ([]string, error)

	// CreateTag create one tag in the repository
	CreateTag(name, revision string) error

	// CreateAnnotatedTag create one annotated tag in the repository
	CreateAnnotatedTag(name, message, revision string) error

	// GetTagNameBySHA returns the name of a tag from its tag object SHA or commit SHA
	GetTagNameBySHA(sha string) (string, error)

	// GetTagID returns the object ID for a tag (annotated tags have both an object SHA AND a commit SHA)
	GetTagID(name string) (string, error)

	// GetTag returns a Git tag by given name.
	GetTag(name string) (Tag, error)

	// GetTagInfos returns all tag infos of the repository.
	GetTagInfos(page, pageSize int) ([]Tag, error)

	// GetTagType gets the type of the tag, either commit (simple) or tag (annotated)
	GetTagType(id Hash) (string, error)

	// GetAnnotatedTag returns a Git tag by its SHA, must be an annotated tag
	GetAnnotatedTag(sha string) (Tag, error)

	// ___
	//  |  ._  _   _
	//  |  |  (/_ (/_
	//

	// GetTree find the tree object in the repository.
	GetTree(idStr string) (Tree, error)

	// CommitTree creates a commit from a given tree id for the user with provided message
	CommitTree(author *Signature, committer *Signature, tree Tree, opts CommitTreeOpts) (Hash, error)

	//  _
	// |_) |  _. ._ _   _
	// |_) | (_| | | | (/_
	//

	// LineBlame returns the latest commit at the given line
	LineBlame(revision, path, file string, line uint) (Commit, error)

	//  __
	// (_   _  ._    o  _  _
	// 	__) (/_ |  \/ | (_ (/_
	//

	// Service returns this repositories preferred service
	Service() GitService
}

// CommitTreeOpts represents the possible options to CommitTree
type CommitTreeOpts struct {
	Parents    []string
	Message    string
	KeyID      string
	NoGPGSign  bool
	AlwaysSign bool
}

// CodeActivityStats represents git statistics data
type CodeActivityStats struct {
	AuthorCount              int64
	CommitCount              int64
	ChangedFiles             int64
	Additions                int64
	Deletions                int64
	CommitCountInAllBranches int64
	Authors                  []*CodeActivityAuthor
}

// CodeActivityAuthor represents git statistics data for commit authors
type CodeActivityAuthor struct {
	Name    string
	Email   string
	Commits int64
}
