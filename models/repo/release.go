// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrReleaseAlreadyExist represents a "ReleaseAlreadyExist" kind of error.
type ErrReleaseAlreadyExist struct {
	TagName string
}

// IsErrReleaseAlreadyExist checks if an error is a ErrReleaseAlreadyExist.
func IsErrReleaseAlreadyExist(err error) bool {
	_, ok := err.(ErrReleaseAlreadyExist)
	return ok
}

func (err ErrReleaseAlreadyExist) Error() string {
	return fmt.Sprintf("release tag already exist [tag_name: %s]", err.TagName)
}

func (err ErrReleaseAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrReleaseNotExist represents a "ReleaseNotExist" kind of error.
type ErrReleaseNotExist struct {
	ID      int64
	TagName string
}

// IsErrReleaseNotExist checks if an error is a ErrReleaseNotExist.
func IsErrReleaseNotExist(err error) bool {
	_, ok := err.(ErrReleaseNotExist)
	return ok
}

func (err ErrReleaseNotExist) Error() string {
	return fmt.Sprintf("release tag does not exist [id: %d, tag_name: %s]", err.ID, err.TagName)
}

func (err ErrReleaseNotExist) Unwrap() error {
	return util.ErrNotExist
}

// Release represents a release of repository.
type Release struct {
	ID               int64            `xorm:"pk autoincr"`
	RepoID           int64            `xorm:"INDEX UNIQUE(n)"`
	Repo             *Repository      `xorm:"-"`
	PublisherID      int64            `xorm:"INDEX"`
	Publisher        *user_model.User `xorm:"-"`
	TagName          string           `xorm:"INDEX UNIQUE(n)"`
	OriginalAuthor   string
	OriginalAuthorID int64 `xorm:"index"`
	LowerTagName     string
	Target           string
	TargetBehind     string `xorm:"-"` // to handle non-existing or empty target
	Title            string
	Sha1             string `xorm:"VARCHAR(40)"`
	NumCommits       int64
	NumCommitsBehind int64              `xorm:"-"`
	Note             string             `xorm:"TEXT"`
	RenderedNote     string             `xorm:"-"`
	IsDraft          bool               `xorm:"NOT NULL DEFAULT false"`
	IsPrerelease     bool               `xorm:"NOT NULL DEFAULT false"`
	IsTag            bool               `xorm:"NOT NULL DEFAULT false"` // will be true only if the record is a tag and has no related releases
	Attachments      []*Attachment      `xorm:"-"`
	CreatedUnix      timeutil.TimeStamp `xorm:"INDEX"`
}

func init() {
	db.RegisterModel(new(Release))
}

// LoadAttributes load repo and publisher attributes for a release
func (r *Release) LoadAttributes(ctx context.Context) error {
	var err error
	if r.Repo == nil {
		r.Repo, err = GetRepositoryByID(ctx, r.RepoID)
		if err != nil {
			return err
		}
	}
	if r.Publisher == nil {
		r.Publisher, err = user_model.GetUserByID(ctx, r.PublisherID)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				r.Publisher = user_model.NewGhostUser()
			} else {
				return err
			}
		}
	}
	return GetReleaseAttachments(ctx, r)
}

// APIURL the api url for a release. release must have attributes loaded
func (r *Release) APIURL() string {
	return r.Repo.APIURL() + "/releases/" + strconv.FormatInt(r.ID, 10)
}

// ZipURL the zip url for a release. release must have attributes loaded
func (r *Release) ZipURL() string {
	return r.Repo.HTMLURL() + "/archive/" + util.PathEscapeSegments(r.TagName) + ".zip"
}

// TarURL the tar.gz url for a release. release must have attributes loaded
func (r *Release) TarURL() string {
	return r.Repo.HTMLURL() + "/archive/" + util.PathEscapeSegments(r.TagName) + ".tar.gz"
}

// HTMLURL the url for a release on the web UI. release must have attributes loaded
func (r *Release) HTMLURL() string {
	return r.Repo.HTMLURL() + "/releases/tag/" + util.PathEscapeSegments(r.TagName)
}

// Link the relative url for a release on the web UI. release must have attributes loaded
func (r *Release) Link() string {
	return r.Repo.Link() + "/releases/tag/" + util.PathEscapeSegments(r.TagName)
}

// IsReleaseExist returns true if release with given tag name already exists.
func IsReleaseExist(ctx context.Context, repoID int64, tagName string) (bool, error) {
	if len(tagName) == 0 {
		return false, nil
	}

	return db.GetEngine(ctx).Exist(&Release{RepoID: repoID, LowerTagName: strings.ToLower(tagName)})
}

// UpdateRelease updates all columns of a release
func UpdateRelease(ctx context.Context, rel *Release) error {
	_, err := db.GetEngine(ctx).ID(rel.ID).AllCols().Update(rel)
	return err
}

// AddReleaseAttachments adds a release attachments
func AddReleaseAttachments(ctx context.Context, releaseID int64, attachmentUUIDs []string) (err error) {
	// Check attachments
	attachments, err := GetAttachmentsByUUIDs(ctx, attachmentUUIDs)
	if err != nil {
		return fmt.Errorf("GetAttachmentsByUUIDs [uuids: %v]: %w", attachmentUUIDs, err)
	}

	for i := range attachments {
		if attachments[i].ReleaseID != 0 {
			return util.NewPermissionDeniedErrorf("release permission denied")
		}
		attachments[i].ReleaseID = releaseID
		// No assign value could be 0, so ignore AllCols().
		if _, err = db.GetEngine(ctx).ID(attachments[i].ID).Update(attachments[i]); err != nil {
			return fmt.Errorf("update attachment [%d]: %w", attachments[i].ID, err)
		}
	}

	return err
}

// GetRelease returns release by given ID.
func GetRelease(repoID int64, tagName string) (*Release, error) {
	rel := &Release{RepoID: repoID, LowerTagName: strings.ToLower(tagName)}
	has, err := db.GetEngine(db.DefaultContext).Get(rel)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrReleaseNotExist{0, tagName}
	}
	return rel, nil
}

// GetReleaseByID returns release with given ID.
func GetReleaseByID(ctx context.Context, id int64) (*Release, error) {
	rel := new(Release)
	has, err := db.GetEngine(ctx).
		ID(id).
		Get(rel)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrReleaseNotExist{id, ""}
	}

	return rel, nil
}

// GetReleaseForRepoByID returns release with given ID.
func GetReleaseForRepoByID(ctx context.Context, repoID, id int64) (*Release, error) {
	rel := new(Release)
	has, err := db.GetEngine(ctx).
		Where("id=? AND repo_id=?", id, repoID).
		Get(rel)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrReleaseNotExist{id, ""}
	}

	return rel, nil
}

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

// CountReleasesByRepoID returns a number of releases matching FindReleaseOptions and RepoID.
func CountReleasesByRepoID(repoID int64, opts FindReleasesOptions) (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(opts.toConds(repoID)).Count(new(Release))
}

// GetLatestReleaseByRepoID returns the latest release for a repository
func GetLatestReleaseByRepoID(repoID int64) (*Release, error) {
	cond := builder.NewCond().
		And(builder.Eq{"repo_id": repoID}).
		And(builder.Eq{"is_draft": false}).
		And(builder.Eq{"is_prerelease": false}).
		And(builder.Eq{"is_tag": false})

	rel := new(Release)
	has, err := db.GetEngine(db.DefaultContext).
		Desc("created_unix", "id").
		Where(cond).
		Get(rel)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrReleaseNotExist{0, "latest"}
	}

	return rel, nil
}

// GetReleasesByRepoIDAndNames returns a list of releases of repository according repoID and tagNames.
func GetReleasesByRepoIDAndNames(ctx context.Context, repoID int64, tagNames []string) (rels []*Release, err error) {
	err = db.GetEngine(ctx).
		In("tag_name", tagNames).
		Desc("created_unix").
		Find(&rels, Release{RepoID: repoID})
	return rels, err
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

// GetReleaseAttachments retrieves the attachments for releases
func GetReleaseAttachments(ctx context.Context, rels ...*Release) (err error) {
	if len(rels) == 0 {
		return
	}

	// To keep this efficient as possible sort all releases by id,
	//    select attachments by release id,
	//    then merge join them

	// Sort
	sortedRels := releaseMetaSearch{ID: make([]int64, len(rels)), Rel: make([]*Release, len(rels))}
	var attachments []*Attachment
	for index, element := range rels {
		element.Attachments = []*Attachment{}
		sortedRels.ID[index] = element.ID
		sortedRels.Rel[index] = element
	}
	sort.Sort(sortedRels)

	// Select attachments
	err = db.GetEngine(ctx).
		Asc("release_id", "name").
		In("release_id", sortedRels.ID).
		Find(&attachments, Attachment{})
	if err != nil {
		return err
	}

	// merge join
	currentIndex := 0
	for _, attachment := range attachments {
		for sortedRels.ID[currentIndex] < attachment.ReleaseID {
			currentIndex++
		}
		sortedRels.Rel[currentIndex].Attachments = append(sortedRels.Rel[currentIndex].Attachments, attachment)
	}

	// Makes URL's predictable
	for _, release := range rels {
		// If we have no Repo, we don't need to execute this loop
		if release.Repo == nil {
			continue
		}

		// Check if there are two or more attachments with the same name
		hasDuplicates := false
		foundNames := make(map[string]bool)
		for _, attachment := range release.Attachments {
			_, found := foundNames[attachment.Name]
			if found {
				hasDuplicates = true
				break
			} else {
				foundNames[attachment.Name] = true
			}
		}

		// If the names unique, use the URL with the Name instead of the UUID
		if !hasDuplicates {
			for _, attachment := range release.Attachments {
				attachment.CustomDownloadURL = release.Repo.HTMLURL() + "/releases/download/" + url.PathEscape(release.TagName) + "/" + url.PathEscape(attachment.Name)
			}
		}
	}

	return err
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

// DeleteReleaseByID deletes a release from database by given ID.
func DeleteReleaseByID(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).ID(id).Delete(new(Release))
	return err
}

// UpdateReleasesMigrationsByType updates all migrated repositories' releases from gitServiceType to replace originalAuthorID to posterID
func UpdateReleasesMigrationsByType(gitServiceType structs.GitServiceType, originalAuthorID string, posterID int64) error {
	_, err := db.GetEngine(db.DefaultContext).Table("release").
		Where("repo_id IN (SELECT id FROM repository WHERE original_service_type = ?)", gitServiceType).
		And("original_author_id = ?", originalAuthorID).
		Update(map[string]any{
			"publisher_id":       posterID,
			"original_author":    "",
			"original_author_id": 0,
		})
	return err
}

// PushUpdateDeleteTagsContext updates a number of delete tags with context
func PushUpdateDeleteTagsContext(ctx context.Context, repo *Repository, tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	lowerTags := make([]string, 0, len(tags))
	for _, tag := range tags {
		lowerTags = append(lowerTags, strings.ToLower(tag))
	}

	if _, err := db.GetEngine(ctx).
		Where("repo_id = ? AND is_tag = ?", repo.ID, true).
		In("lower_tag_name", lowerTags).
		Delete(new(Release)); err != nil {
		return fmt.Errorf("Delete: %w", err)
	}

	if _, err := db.GetEngine(ctx).
		Where("repo_id = ? AND is_tag = ?", repo.ID, false).
		In("lower_tag_name", lowerTags).
		Cols("is_draft", "num_commits", "sha1").
		Update(&Release{
			IsDraft: true,
		}); err != nil {
		return fmt.Errorf("Update: %w", err)
	}

	return nil
}

// PushUpdateDeleteTag must be called for any push actions to delete tag
func PushUpdateDeleteTag(repo *Repository, tagName string) error {
	rel, err := GetRelease(repo.ID, tagName)
	if err != nil {
		if IsErrReleaseNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetRelease: %w", err)
	}
	if rel.IsTag {
		if _, err = db.GetEngine(db.DefaultContext).ID(rel.ID).Delete(new(Release)); err != nil {
			return fmt.Errorf("Delete: %w", err)
		}
	} else {
		rel.IsDraft = true
		rel.NumCommits = 0
		rel.Sha1 = ""
		if _, err = db.GetEngine(db.DefaultContext).ID(rel.ID).AllCols().Update(rel); err != nil {
			return fmt.Errorf("Update: %w", err)
		}
	}

	return nil
}

// SaveOrUpdateTag must be called for any push actions to add tag
func SaveOrUpdateTag(repo *Repository, newRel *Release) error {
	rel, err := GetRelease(repo.ID, newRel.TagName)
	if err != nil && !IsErrReleaseNotExist(err) {
		return fmt.Errorf("GetRelease: %w", err)
	}

	if rel == nil {
		rel = newRel
		if _, err = db.GetEngine(db.DefaultContext).Insert(rel); err != nil {
			return fmt.Errorf("InsertOne: %w", err)
		}
	} else {
		rel.Sha1 = newRel.Sha1
		rel.CreatedUnix = newRel.CreatedUnix
		rel.NumCommits = newRel.NumCommits
		rel.IsDraft = false
		if rel.IsTag && newRel.PublisherID > 0 {
			rel.PublisherID = newRel.PublisherID
		}
		if _, err = db.GetEngine(db.DefaultContext).ID(rel.ID).AllCols().Update(rel); err != nil {
			return fmt.Errorf("Update: %w", err)
		}
	}
	return nil
}

// RemapExternalUser ExternalUserRemappable interface
func (r *Release) RemapExternalUser(externalName string, externalID, userID int64) error {
	r.OriginalAuthor = externalName
	r.OriginalAuthorID = externalID
	r.PublisherID = userID
	return nil
}

// UserID ExternalUserRemappable interface
func (r *Release) GetUserID() int64 { return r.PublisherID }

// ExternalName ExternalUserRemappable interface
func (r *Release) GetExternalName() string { return r.OriginalAuthor }

// ExternalID ExternalUserRemappable interface
func (r *Release) GetExternalID() int64 { return r.OriginalAuthorID }
