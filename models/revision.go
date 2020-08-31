// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"sort"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
)

// Revision Tracking information about PR revisions.
type Revision struct {
	ID                       int64                   `xorm:"pk autoincr"`
	Commit                   string                  `xorm:"NOT NULL"`
	SignedCommitWithStatuses *SignCommitWithStatuses `xorm:"-"`
	NumberOfCommits          int64                   `xorm:"NOT NULL"`
	ChangesetHeadRef         string                  `xorm:"-"`
	PrevChangesetHeadRef     string                  `xorm:"-"`
	UserID                   int64                   `xorm:"NOT NULL"`
	User                     *User                   `xorm:"-"`
	PRID                     int64                   `xorm:"pr_id NOT NULL"`
	PR                       *PullRequest            `xorm:"-"`
	Index                    int64                   `xorm:"NOT NULL"`
	Created                  timeutil.TimeStamp      `xorm:"NOT NULL"`
}

// GetRevision get the revision by pr and revision index.
func GetRevision(pr *PullRequest, index int64) (*Revision, error) {
	rev := &Revision{PRID: pr.ID, Index: index}
	has, e := x.Get(rev)

	if e != nil {
		return nil, e
	}

	if has {
		return rev, nil
	}
	return nil, nil
}

// LoadPrevChangesetHeadRef load the previous revision's git ref.
func (rev *Revision) LoadPrevChangesetHeadRef() error {
	if rev.PrevChangesetHeadRef != "" {
		return nil
	}

	err := rev.LoadPullRequest()
	if err != nil {
		return err
	}

	if rev.Index > 1 {
		rev.PrevChangesetHeadRef = git.GetRevisionRef(rev.PR.Index, rev.Index-1)
	}

	return nil
}

// LoadChangesetHeadRef load the git ref of the revision.
func (rev *Revision) LoadChangesetHeadRef() error {
	if rev.PrevChangesetHeadRef != "" {
		return nil
	}

	err := rev.LoadPullRequest()
	if err != nil {
		return err
	}

	rev.ChangesetHeadRef = git.GetRevisionRef(rev.PR.Index, rev.Index)

	return nil
}

// LoadUser load the user field
func (rev *Revision) LoadUser() error {
	if rev.User != nil {
		return nil
	}

	user, err := GetUserByID(rev.UserID)
	if err != nil {
		if IsErrUserNotExist(err) {
			rev.User = NewGhostUser()
		} else {
			return err
		}
	} else {
		rev.User = user
	}

	return nil
}

// LoadPullRequest load the PR field
func (rev *Revision) LoadPullRequest() error {
	if rev.PR != nil {
		return nil
	}

	pr, err := GetPullRequestByID(rev.PRID)
	if err != nil {
		return err
	}
	rev.PR = pr

	return nil
}

// LoadCommit loads information about the signatures and statuses of the revision head commit
func (rev *Revision) LoadCommit() error {
	err := rev.LoadPullRequest()
	if err != nil {
		return err
	}
	if rev.SignedCommitWithStatuses != nil {
		return nil
	}

	err = rev.PR.LoadBaseRepo()
	if err != nil {
		return err
	}

	gitRepo, err := git.OpenRepository(rev.PR.BaseRepo.RepoPath())
	if err != nil {
		return err
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(rev.Commit)
	if err != nil {
		return err
	}

	c := ParseGitCommitWithStatus(commit, rev.PR.BaseRepo, nil, nil)
	rev.SignedCommitWithStatuses = &c

	return nil
}

// GetRevisions Gets the list of revisions of a PR.
func GetRevisions(pr *PullRequest) ([]*Revision, error) {
	err := pr.LoadBaseRepo()
	if err != nil {
		return nil, err
	}

	baseRepo, err := git.OpenRepository(pr.BaseRepo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer baseRepo.Close()

	var refs, error = baseRepo.GetRevisionRefs(pr.Index)
	if error != nil {
		return nil, error
	}

	var revisions []*Revision

	for _, ref := range refs {
		var index = git.GetRevisionIndexFromRef(ref.Name)

		if index == nil {
			continue
		}

		rev, err := GetRevision(pr, *index)

		if err != nil {
			log.Error("GetRevisions, GetRevision: %v", err)
			continue
		}

		err = rev.LoadUser()

		if err != nil {
			log.Error("GetRevisions, LoadUser: %v", err)
			continue
		}

		err = rev.LoadPullRequest()

		if err != nil {
			log.Error("GetRevisions, LoadPullRequest: %v", err)
			continue
		}

		err = rev.LoadCommit()

		if err != nil {
			log.Error("GetRevisions, LoadCommit: %v", err)
			continue
		}

		err = rev.LoadChangesetHeadRef()

		if err != nil {
			log.Error("GetRevisions, LoadChangesetHeadRef: %v", err)
			continue
		}

		err = rev.LoadPrevChangesetHeadRef()

		if err != nil {
			log.Error("GetRevisions, LoadPrevChangesetHeadRef: %v", err)
			continue
		}

		revisions = append(revisions, rev)
	}

	sort.Slice(revisions, func(i, j int) bool {
		return revisions[j].Index < revisions[i].Index
	})

	return revisions, nil
}

// InitializeRevisions Enables revisioning and creates the first revision for a PR.
func InitializeRevisions(pr *PullRequest) (*Revision, error) {
	err := pr.LoadBaseRepo()
	if err != nil {
		return nil, err
	}

	repo, err := git.OpenRepository(pr.BaseRepo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer repo.Close()

	ref := pr.GetGitRefName()
	commit, err := repo.GetRefCommitID(ref)
	if err != nil {
		return nil, err
	}

	index, err := repo.InitializeRevisions(pr.Index, commit)
	if err != nil {
		return nil, err
	}

	count, err := repo.CommitsCountBetween(pr.MergeBase, commit)
	if err != nil {
		return nil, err
	}

	return createMetadata(pr, pr.Issue.Poster, commit, count, index)
}

// CreateNewRevision Creates a new revision entry for the PR.
func CreateNewRevision(pr *PullRequest, user *User, commit string) (*Revision, error) {
	err := pr.LoadBaseRepo()
	if err != nil {
		return nil, err
	}

	repo, err := git.OpenRepository(pr.BaseRepo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer repo.Close()

	var refs, error = repo.GetRevisionRefs(pr.Index)
	if error != nil {
		return nil, error
	}

	for _, ref := range refs {
		if ref.Object.String() == commit {
			index := git.GetRevisionIndexFromRef(ref.Name)

			if index == nil {
				continue
			}

			count, err := repo.CommitsCountBetween(pr.MergeBase, commit)
			if err != nil {
				return nil, err
			}

			return createMetadata(pr, user, commit, count, *index)
		}
	}

	// this can only happen if this is a PR between repos. In that case the push doesn't create the git ref for the revisions
	// Only later will we "force-pull" in the PushToBase func which is called from AddTestPullRequestTask.
	// If this is the case then it is not surprising that we didn't find a ref for the commit. Lets create it now
	index, err := repo.CreateNewRevision(pr.Index, commit)

	if err != nil {
		return nil, err
	}

	count, err := repo.CommitsCountBetween(pr.MergeBase, commit)
	if err != nil {
		return nil, err
	}

	return createMetadata(pr, user, commit, count, index)
}

func createMetadata(pr *PullRequest, user *User, commit string, count int64, index int64) (*Revision, error) {
	rev := &Revision{
		PRID:            pr.ID,
		Index:           index,
		Commit:          commit,
		NumberOfCommits: count,
		UserID:          user.ID,
		Created:         timeutil.TimeStampNow(),
	}
	_, err := x.Insert(rev)
	if err != nil {
		return nil, err
	}
	_, err = x.Get(rev)
	if err != nil {
		return nil, err
	}
	return rev, nil
}
