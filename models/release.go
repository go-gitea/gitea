// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-xorm/xorm"

	"github.com/go-gitea/git"

	"github.com/go-gitea/gitea/modules/process"
)

// Release represents a release of repository.
type Release struct {
	ID               int64 `xorm:"pk autoincr"`
	RepoID           int64
	PublisherID      int64
	Publisher        *User `xorm:"-"`
	TagName          string
	LowerTagName     string
	Target           string
	Title            string
	Sha1             string `xorm:"VARCHAR(40)"`
	NumCommits       int64
	NumCommitsBehind int64  `xorm:"-"`
	Note             string `xorm:"TEXT"`
	IsDraft          bool   `xorm:"NOT NULL DEFAULT false"`
	IsPrerelease     bool

	Created     time.Time `xorm:"-"`
	CreatedUnix int64
}

func (r *Release) BeforeInsert() {
	if r.CreatedUnix == 0 {
		r.CreatedUnix = time.Now().Unix()
	}
}

func (r *Release) AfterSet(colName string, _ xorm.Cell) {
	switch colName {
	case "created_unix":
		r.Created = time.Unix(r.CreatedUnix, 0).Local()
	}
}

// IsReleaseExist returns true if release with given tag name already exists.
func IsReleaseExist(repoID int64, tagName string) (bool, error) {
	if len(tagName) == 0 {
		return false, nil
	}

	return x.Get(&Release{RepoID: repoID, LowerTagName: strings.ToLower(tagName)})
}

func createTag(gitRepo *git.Repository, rel *Release) error {
	// Only actual create when publish.
	if !rel.IsDraft {
		if !gitRepo.IsTagExist(rel.TagName) {
			commit, err := gitRepo.GetBranchCommit(rel.Target)
			if err != nil {
				return fmt.Errorf("GetBranchCommit: %v", err)
			}

			// Trim '--' prefix to prevent command line argument vulnerability.
			rel.TagName = strings.TrimPrefix(rel.TagName, "--")
			if err = gitRepo.CreateTag(rel.TagName, commit.ID.String()); err != nil {
				if strings.Contains(err.Error(), "is not a valid tag name") {
					return ErrInvalidTagName{rel.TagName}
				}
				return err
			}
		} else {
			commit, err := gitRepo.GetTagCommit(rel.TagName)
			if err != nil {
				return fmt.Errorf("GetTagCommit: %v", err)
			}

			rel.Sha1 = commit.ID.String()
			rel.NumCommits, err = commit.CommitsCount()
			if err != nil {
				return fmt.Errorf("CommitsCount: %v", err)
			}
		}
	}
	return nil
}

// CreateRelease creates a new release of repository.
func CreateRelease(gitRepo *git.Repository, rel *Release) error {
	isExist, err := IsReleaseExist(rel.RepoID, rel.TagName)
	if err != nil {
		return err
	} else if isExist {
		return ErrReleaseAlreadyExist{rel.TagName}
	}

	if err = createTag(gitRepo, rel); err != nil {
		return err
	}
	rel.LowerTagName = strings.ToLower(rel.TagName)
	_, err = x.InsertOne(rel)
	return err
}

// GetRelease returns release by given ID.
func GetRelease(repoID int64, tagName string) (*Release, error) {
	isExist, err := IsReleaseExist(repoID, tagName)
	if err != nil {
		return nil, err
	} else if !isExist {
		return nil, ErrReleaseNotExist{0, tagName}
	}

	rel := &Release{RepoID: repoID, LowerTagName: strings.ToLower(tagName)}
	_, err = x.Get(rel)
	return rel, err
}

// GetReleaseByID returns release with given ID.
func GetReleaseByID(id int64) (*Release, error) {
	rel := new(Release)
	has, err := x.Id(id).Get(rel)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrReleaseNotExist{id, ""}
	}

	return rel, nil
}

// GetReleasesByRepoID returns a list of releases of repository.
func GetReleasesByRepoID(repoID int64) (rels []*Release, err error) {
	err = x.Desc("created_unix").Find(&rels, Release{RepoID: repoID})
	return rels, err
}

type ReleaseSorter struct {
	rels []*Release
}

func (rs *ReleaseSorter) Len() int {
	return len(rs.rels)
}

func (rs *ReleaseSorter) Less(i, j int) bool {
	diffNum := rs.rels[i].NumCommits - rs.rels[j].NumCommits
	if diffNum != 0 {
		return diffNum > 0
	}
	return rs.rels[i].Created.After(rs.rels[j].Created)
}

func (rs *ReleaseSorter) Swap(i, j int) {
	rs.rels[i], rs.rels[j] = rs.rels[j], rs.rels[i]
}

// SortReleases sorts releases by number of commits and created time.
func SortReleases(rels []*Release) {
	sorter := &ReleaseSorter{rels: rels}
	sort.Sort(sorter)
}

// UpdateRelease updates information of a release.
func UpdateRelease(gitRepo *git.Repository, rel *Release) (err error) {
	if err = createTag(gitRepo, rel); err != nil {
		return err
	}
	_, err = x.Id(rel.ID).AllCols().Update(rel)
	return err
}

// DeleteReleaseByID deletes a release and corresponding Git tag by given ID.
func DeleteReleaseByID(id int64) error {
	rel, err := GetReleaseByID(id)
	if err != nil {
		return fmt.Errorf("GetReleaseByID: %v", err)
	}

	repo, err := GetRepositoryByID(rel.RepoID)
	if err != nil {
		return fmt.Errorf("GetRepositoryByID: %v", err)
	}

	_, stderr, err := process.ExecDir(-1, repo.RepoPath(),
		fmt.Sprintf("DeleteReleaseByID (git tag -d): %d", rel.ID),
		"git", "tag", "-d", rel.TagName)
	if err != nil && !strings.Contains(stderr, "not found") {
		return fmt.Errorf("git tag -d: %v - %s", err, stderr)
	}

	if _, err = x.Id(rel.ID).Delete(new(Release)); err != nil {
		return fmt.Errorf("Delete: %v", err)
	}

	return nil
}
