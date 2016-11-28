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

	"code.gitea.io/git"

	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"

	api "code.gitea.io/sdk/gitea"
)

// Release represents a release of repository.
type Release struct {
	ID               int64       `xorm:"pk autoincr"`
	RepoID           int64       `xorm:"INDEX UNIQUE(n)"`
	Repo             *Repository `xorm:"-"`
	PublisherID      int64       `xorm:"INDEX"`
	Publisher        *User       `xorm:"-"`
	TagName          string      `xorm:"INDEX UNIQUE(n)"`
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
	CreatedUnix int64     `xorm:"INDEX"`
}

// BeforeInsert is invoked from XORM before inserting an object of this type.
func (r *Release) BeforeInsert() {
	if r.CreatedUnix == 0 {
		r.CreatedUnix = time.Now().Unix()
	}
}

// AfterSet is invoked from XORM after setting the value of a field of this object.
func (r *Release) AfterSet(colName string, _ xorm.Cell) {
	switch colName {
	case "created_unix":
		r.Created = time.Unix(r.CreatedUnix, 0).Local()
	}
}

func (r *Release) loadAttributes(e Engine) error {
	var err error
	if r.Repo == nil {
		r.Repo, err = GetRepositoryByID(r.RepoID)
		if err != nil {
			return err
		}
	}
	if r.Publisher == nil {
		r.Publisher, err = GetUserByID(r.PublisherID)
		if err != nil {
			return err
		}
	}
	return nil
}

// LoadAttributes load repo and publisher attributes for a realease
func (r *Release) LoadAttributes() error {
	return r.loadAttributes(x)
}

// APIURL the api url for a release. release must have attributes loaded
func (r *Release) APIURL() string {
	return fmt.Sprintf("%sapi/v1/%s/releases/%d",
		setting.AppURL, r.Repo.FullName(), r.ID)
}

// ZipURL the zip url for a release. release must have attributes loaded
func (r *Release) ZipURL() string {
	return fmt.Sprintf("%s/archive/%s.zip", r.Repo.HTMLURL(), r.TagName)
}

// TarURL the tar.gz url for a release. release must have attributes loaded
func (r *Release) TarURL() string {
	return fmt.Sprintf("%s/archive/%s.tar.gz", r.Repo.HTMLURL(), r.TagName)
}

// APIFormat convert a Release to api.Release
func (r *Release) APIFormat() *api.Release {
	return &api.Release{
		ID:           r.ID,
		TagName:      r.TagName,
		Target:       r.Target,
		Note:         r.Note,
		URL:          r.APIURL(),
		TarURL:       r.TarURL(),
		ZipURL:       r.ZipURL(),
		IsDraft:      r.IsDraft,
		IsPrerelease: r.IsPrerelease,
		CreatedAt:    r.Created,
		PublishedAt:  r.Created,
		Publisher:    r.Publisher.APIFormat(),
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
	has, err := x.
		Id(id).
		Get(rel)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrReleaseNotExist{id, ""}
	}

	return rel, nil
}

// GetReleasesByRepoID returns a list of releases of repository.
func GetReleasesByRepoID(repoID int64, page, pageSize int) (rels []*Release, err error) {
	if page <= 0 {
		page = 1
	}
	err = x.
		Desc("created_unix").
		Limit(pageSize, (page-1)*pageSize).
		Find(&rels, Release{RepoID: repoID})
	return rels, err
}

// GetReleasesByRepoIDAndNames returns a list of releases of repository accroding repoID and tagNames.
func GetReleasesByRepoIDAndNames(repoID int64, tagNames []string) (rels []*Release, err error) {
	err = x.
		Desc("created_unix").
		In("tag_name", tagNames).
		Find(&rels, Release{RepoID: repoID})
	return rels, err
}

type releaseSorter struct {
	rels []*Release
}

func (rs *releaseSorter) Len() int {
	return len(rs.rels)
}

func (rs *releaseSorter) Less(i, j int) bool {
	diffNum := rs.rels[i].NumCommits - rs.rels[j].NumCommits
	if diffNum != 0 {
		return diffNum > 0
	}
	return rs.rels[i].Created.After(rs.rels[j].Created)
}

func (rs *releaseSorter) Swap(i, j int) {
	rs.rels[i], rs.rels[j] = rs.rels[j], rs.rels[i]
}

// SortReleases sorts releases by number of commits and created time.
func SortReleases(rels []*Release) {
	sorter := &releaseSorter{rels: rels}
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
func DeleteReleaseByID(id int64, u *User, delTag bool) error {
	rel, err := GetReleaseByID(id)
	if err != nil {
		return fmt.Errorf("GetReleaseByID: %v", err)
	}

	repo, err := GetRepositoryByID(rel.RepoID)
	if err != nil {
		return fmt.Errorf("GetRepositoryByID: %v", err)
	}

	has, err := HasAccess(u, repo, AccessModeWrite)
	if err != nil {
		return fmt.Errorf("HasAccess: %v", err)
	} else if !has {
		return fmt.Errorf("DeleteReleaseByID: permission denied")
	}

	if delTag {
		_, stderr, err := process.ExecDir(-1, repo.RepoPath(),
			fmt.Sprintf("DeleteReleaseByID (git tag -d): %d", rel.ID),
			"git", "tag", "-d", rel.TagName)
		if err != nil && !strings.Contains(stderr, "not found") {
			return fmt.Errorf("git tag -d: %v - %s", err, stderr)
		}
	}

	if _, err = x.Id(rel.ID).Delete(new(Release)); err != nil {
		return fmt.Errorf("Delete: %v", err)
	}

	return nil
}
