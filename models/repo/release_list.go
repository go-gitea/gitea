// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"sort"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"
	"xorm.io/builder"
)

// FindReleasesOptions describes the conditions to Find releases
type FindReleasesOptions struct {
	db.ListOptions
	IncludeDrafts bool
	IncludeTags   bool
	IsPreRelease  util.OptionalBool
	IsDraft       util.OptionalBool
	TagNames      []string
	HasSha1       util.OptionalBool // useful to find draft releases which are created with existing tags
}

func (opts *FindReleasesOptions) toConds(repoID int64) builder.Cond {
	cond := builder.NewCond()
	cond = cond.And(builder.Eq{"repo_id": repoID})

	if !opts.IncludeDrafts {
		cond = cond.And(builder.Eq{"is_draft": false})
	}
	if !opts.IncludeTags {
		cond = cond.And(builder.Eq{"is_tag": false})
	}
	if len(opts.TagNames) > 0 {
		cond = cond.And(builder.In("tag_name", opts.TagNames))
	}
	if !opts.IsPreRelease.IsNone() {
		cond = cond.And(builder.Eq{"is_prerelease": opts.IsPreRelease.IsTrue()})
	}
	if !opts.IsDraft.IsNone() {
		cond = cond.And(builder.Eq{"is_draft": opts.IsDraft.IsTrue()})
	}
	if !opts.HasSha1.IsNone() {
		if opts.HasSha1.IsTrue() {
			cond = cond.And(builder.Neq{"sha1": ""})
		} else {
			cond = cond.And(builder.Eq{"sha1": ""})
		}
	}
	return cond
}

// GetReleasesByRepoID returns a list of releases of repository.
func GetReleasesByRepoID(ctx context.Context, repoID int64, opts FindReleasesOptions) ([]*Release, error) {
	sess := db.GetEngine(ctx).
		Desc("created_unix", "id").
		Where(opts.toConds(repoID))

	if opts.PageSize != 0 {
		sess = db.SetSessionPagination(sess, &opts.ListOptions)
	}

	rels := make([]*Release, 0, opts.PageSize)
	return rels, sess.Find(&rels)
}

// CountReleasesByRepoID returns a number of releases matching FindReleaseOptions and RepoID.
func CountReleasesByRepoID(repoID int64, opts FindReleasesOptions) (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(opts.toConds(repoID)).Count(new(Release))
}

// GetReleaseCountByRepoID returns the count of releases of repository
func GetReleaseCountByRepoID(ctx context.Context, repoID int64, opts FindReleasesOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.toConds(repoID)).Count(&Release{})
}

type releaseMetaSearch struct {
	ID  []int64
	Rel []*Release
}

func (s releaseMetaSearch) Len() int {
	return len(s.ID)
}

func (s releaseMetaSearch) Swap(i, j int) {
	s.ID[i], s.ID[j] = s.ID[j], s.ID[i]
	s.Rel[i], s.Rel[j] = s.Rel[j], s.Rel[i]
}

func (s releaseMetaSearch) Less(i, j int) bool {
	return s.ID[i] < s.ID[j]
}

// GetTagNamesByRepoID returns a list of release tag names of repository.
func GetTagNamesByRepoID(ctx context.Context, repoID int64) ([]string, error) {
	listOptions := db.ListOptions{
		ListAll: true,
	}
	opts := FindReleasesOptions{
		ListOptions:   listOptions,
		IncludeDrafts: true,
		IncludeTags:   true,
		HasSha1:       util.OptionalBoolTrue,
	}

	tags := make([]string, 0)
	sess := db.GetEngine(ctx).
		Table("release").
		Desc("created_unix", "id").
		Where(opts.toConds(repoID)).
		Cols("tag_name")

	return tags, sess.Find(&tags)
}

// GetReleasesByRepoIDAndNames returns a list of releases of repository according repoID and tagNames.
func GetReleasesByRepoIDAndNames(ctx context.Context, repoID int64, tagNames []string) (rels []*Release, err error) {
	err = db.GetEngine(ctx).
		In("tag_name", tagNames).
		Desc("created_unix").
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
	return rs.rels[i].CreatedUnix > rs.rels[j].CreatedUnix
}

func (rs *releaseSorter) Swap(i, j int) {
	rs.rels[i], rs.rels[j] = rs.rels[j], rs.rels[i]
}

// SortReleases sorts releases by number of commits and created time.
func SortReleases(rels []*Release) {
	sorter := &releaseSorter{rels: rels}
	sort.Sort(sorter)
}

// FindReleasesByTagSha1s search a repository's releases
func FindReleasesByTagSha1s(ctx context.Context, repoID int64, sha1s []string) (map[string][]*Release, error) {
	releases := make([]*Release, 0, len(sha1s))
	if err := db.GetEngine(ctx).
		Where("repo_id=?", repoID).
		And("is_draft=?", false).
		In("sha1", sha1s). // TODO: sha1 should be indexed
		Find(&releases); err != nil {
		return nil, err
	}
	relMap := make(map[string][]*Release, len(releases))
	for _, rel := range releases {
		relMap[rel.Sha1] = append(relMap[rel.Sha1], rel)
	}
	return relMap, nil
}
