// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
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
	NumCommitsBehind int64          `xorm:"-"`
	Note             string         `xorm:"TEXT"`
	IsDraft          bool           `xorm:"NOT NULL DEFAULT false"`
	IsPrerelease     bool           `xorm:"NOT NULL DEFAULT false"`
	IsTag            bool           `xorm:"NOT NULL DEFAULT false"`
	Attachments      []*Attachment  `xorm:"-"`
	CreatedUnix      util.TimeStamp `xorm:"INDEX"`
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
		r.Publisher, err = getUserByID(e, r.PublisherID)
		if err != nil {
			return err
		}
	}
	return getReleaseAttachments(e, r)
}

// LoadAttributes load repo and publisher attributes for a release
func (r *Release) LoadAttributes() error {
	return r.loadAttributes(x)
}

// APIURL the api url for a release. release must have attributes loaded
func (r *Release) APIURL() string {
	return fmt.Sprintf("%sapi/v1/repos/%s/releases/%d",
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
	assets := make([]*api.Attachment, 0)
	for _, att := range r.Attachments {
		assets = append(assets, att.APIFormat())
	}
	return &api.Release{
		ID:           r.ID,
		TagName:      r.TagName,
		Target:       r.Target,
		Title:        r.Title,
		Note:         r.Note,
		URL:          r.APIURL(),
		TarURL:       r.TarURL(),
		ZipURL:       r.ZipURL(),
		IsDraft:      r.IsDraft,
		IsPrerelease: r.IsPrerelease,
		CreatedAt:    r.CreatedUnix.AsTime(),
		PublishedAt:  r.CreatedUnix.AsTime(),
		Publisher:    r.Publisher.APIFormat(),
		Attachments:  assets,
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
			commit, err := gitRepo.GetCommit(rel.Target)
			if err != nil {
				return fmt.Errorf("GetCommit: %v", err)
			}

			// Trim '--' prefix to prevent command line argument vulnerability.
			rel.TagName = strings.TrimPrefix(rel.TagName, "--")
			if err = gitRepo.CreateTag(rel.TagName, commit.ID.String()); err != nil {
				if strings.Contains(err.Error(), "is not a valid tag name") {
					return ErrInvalidTagName{rel.TagName}
				}
				return err
			}
			rel.LowerTagName = strings.ToLower(rel.TagName)
		}
		commit, err := gitRepo.GetTagCommit(rel.TagName)
		if err != nil {
			return fmt.Errorf("GetTagCommit: %v", err)
		}

		rel.Sha1 = commit.ID.String()
		rel.CreatedUnix = util.TimeStamp(commit.Author.When.Unix())
		rel.NumCommits, err = commit.CommitsCount()
		if err != nil {
			return fmt.Errorf("CommitsCount: %v", err)
		}
	} else {
		rel.CreatedUnix = util.TimeStampNow()
	}
	return nil
}

func addReleaseAttachments(releaseID int64, attachmentUUIDs []string) (err error) {
	// Check attachments
	var attachments = make([]*Attachment, 0)
	for _, uuid := range attachmentUUIDs {
		attach, err := getAttachmentByUUID(x, uuid)
		if err != nil {
			if IsErrAttachmentNotExist(err) {
				continue
			}
			return fmt.Errorf("getAttachmentByUUID [%s]: %v", uuid, err)
		}
		attachments = append(attachments, attach)
	}

	for i := range attachments {
		attachments[i].ReleaseID = releaseID
		// No assign value could be 0, so ignore AllCols().
		if _, err = x.ID(attachments[i].ID).Update(attachments[i]); err != nil {
			return fmt.Errorf("update attachment [%d]: %v", attachments[i].ID, err)
		}
	}

	return
}

// CreateRelease creates a new release of repository.
func CreateRelease(gitRepo *git.Repository, rel *Release, attachmentUUIDs []string) error {
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
	if err != nil {
		return err
	}

	err = addReleaseAttachments(rel.ID, attachmentUUIDs)
	if err != nil {
		return err
	}

	if !rel.IsDraft {
		if err := rel.LoadAttributes(); err != nil {
			log.Error("LoadAttributes: %v", err)
		} else {
			mode, _ := AccessLevel(rel.Publisher, rel.Repo)
			if err := PrepareWebhooks(rel.Repo, HookEventRelease, &api.ReleasePayload{
				Action:     api.HookReleasePublished,
				Release:    rel.APIFormat(),
				Repository: rel.Repo.APIFormat(mode),
				Sender:     rel.Publisher.APIFormat(),
			}); err != nil {
				log.Error("PrepareWebhooks: %v", err)
			} else {
				go HookQueue.Add(rel.Repo.ID)
			}
		}
	}

	return nil
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
		ID(id).
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
	IncludeDrafts bool
	IncludeTags   bool
	TagNames      []string
}

func (opts *FindReleasesOptions) toConds(repoID int64) builder.Cond {
	var cond = builder.NewCond()
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
	return cond
}

// GetReleasesByRepoID returns a list of releases of repository.
func GetReleasesByRepoID(repoID int64, opts FindReleasesOptions, page, pageSize int) (rels []*Release, err error) {
	if page <= 0 {
		page = 1
	}

	err = x.
		Desc("created_unix", "id").
		Limit(pageSize, (page-1)*pageSize).
		Where(opts.toConds(repoID)).
		Find(&rels)
	return rels, err
}

// GetReleasesByRepoIDAndNames returns a list of releases of repository according repoID and tagNames.
func GetReleasesByRepoIDAndNames(repoID int64, tagNames []string) (rels []*Release, err error) {
	err = x.
		Desc("created_unix").
		In("tag_name", tagNames).
		Find(&rels, Release{RepoID: repoID})
	return rels, err
}

// GetReleaseCountByRepoID returns the count of releases of repository
func GetReleaseCountByRepoID(repoID int64, opts FindReleasesOptions) (int64, error) {
	return x.Where(opts.toConds(repoID)).Count(&Release{})
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
func GetReleaseAttachments(rels ...*Release) (err error) {
	return getReleaseAttachments(x, rels...)
}

func getReleaseAttachments(e Engine, rels ...*Release) (err error) {
	if len(rels) == 0 {
		return
	}

	// To keep this efficient as possible sort all releases by id,
	//    select attachments by release id,
	//    then merge join them

	// Sort
	var sortedRels = releaseMetaSearch{ID: make([]int64, len(rels)), Rel: make([]*Release, len(rels))}
	var attachments []*Attachment
	for index, element := range rels {
		element.Attachments = []*Attachment{}
		sortedRels.ID[index] = element.ID
		sortedRels.Rel[index] = element
	}
	sort.Sort(sortedRels)

	// Select attachments
	err = e.
		Asc("release_id").
		In("release_id", sortedRels.ID).
		Find(&attachments, Attachment{})
	if err != nil {
		return err
	}

	// merge join
	var currentIndex = 0
	for _, attachment := range attachments {
		for sortedRels.ID[currentIndex] < attachment.ReleaseID {
			currentIndex++
		}
		sortedRels.Rel[currentIndex].Attachments = append(sortedRels.Rel[currentIndex].Attachments, attachment)
	}

	return
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

// UpdateRelease updates information of a release.
func UpdateRelease(doer *User, gitRepo *git.Repository, rel *Release, attachmentUUIDs []string) (err error) {
	if err = createTag(gitRepo, rel); err != nil {
		return err
	}
	rel.LowerTagName = strings.ToLower(rel.TagName)

	_, err = x.ID(rel.ID).AllCols().Update(rel)
	if err != nil {
		return err
	}

	if err = addReleaseAttachments(rel.ID, attachmentUUIDs); err != nil {
		log.Error("addReleaseAttachments: %v", err)
	}

	err = rel.loadAttributes(x)
	if err != nil {
		return err
	}

	mode, _ := AccessLevel(doer, rel.Repo)
	if err1 := PrepareWebhooks(rel.Repo, HookEventRelease, &api.ReleasePayload{
		Action:     api.HookReleaseUpdated,
		Release:    rel.APIFormat(),
		Repository: rel.Repo.APIFormat(mode),
		Sender:     doer.APIFormat(),
	}); err1 != nil {
		log.Error("PrepareWebhooks: %v", err)
	} else {
		go HookQueue.Add(rel.Repo.ID)
	}

	return err
}

// DeleteReleaseByID deletes a release and corresponding Git tag by given ID.
func DeleteReleaseByID(id int64, doer *User, delTag bool) error {
	rel, err := GetReleaseByID(id)
	if err != nil {
		return fmt.Errorf("GetReleaseByID: %v", err)
	}

	repo, err := GetRepositoryByID(rel.RepoID)
	if err != nil {
		return fmt.Errorf("GetRepositoryByID: %v", err)
	}

	if delTag {
		_, stderr, err := process.GetManager().ExecDir(-1, repo.RepoPath(),
			fmt.Sprintf("DeleteReleaseByID (git tag -d): %d", rel.ID),
			git.GitExecutable, "tag", "-d", rel.TagName)
		if err != nil && !strings.Contains(stderr, "not found") {
			return fmt.Errorf("git tag -d: %v - %s", err, stderr)
		}

		if _, err = x.ID(rel.ID).Delete(new(Release)); err != nil {
			return fmt.Errorf("Delete: %v", err)
		}
	} else {
		rel.IsTag = true
		rel.IsDraft = false
		rel.IsPrerelease = false
		rel.Title = ""
		rel.Note = ""

		if _, err = x.ID(rel.ID).AllCols().Update(rel); err != nil {
			return fmt.Errorf("Update: %v", err)
		}
	}

	rel.Repo = repo
	if err = rel.LoadAttributes(); err != nil {
		return fmt.Errorf("LoadAttributes: %v", err)
	}

	mode, _ := AccessLevel(doer, rel.Repo)
	if err := PrepareWebhooks(rel.Repo, HookEventRelease, &api.ReleasePayload{
		Action:     api.HookReleaseDeleted,
		Release:    rel.APIFormat(),
		Repository: rel.Repo.APIFormat(mode),
		Sender:     doer.APIFormat(),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	} else {
		go HookQueue.Add(rel.Repo.ID)
	}

	return nil
}

// SyncReleasesWithTags synchronizes release table with repository tags
func SyncReleasesWithTags(repo *Repository, gitRepo *git.Repository) error {
	existingRelTags := make(map[string]struct{})
	opts := FindReleasesOptions{IncludeDrafts: true, IncludeTags: true}
	for page := 1; ; page++ {
		rels, err := GetReleasesByRepoID(repo.ID, opts, page, 100)
		if err != nil {
			return fmt.Errorf("GetReleasesByRepoID: %v", err)
		}
		if len(rels) == 0 {
			break
		}
		for _, rel := range rels {
			if rel.IsDraft {
				continue
			}
			commitID, err := gitRepo.GetTagCommitID(rel.TagName)
			if err != nil && !git.IsErrNotExist(err) {
				return fmt.Errorf("GetTagCommitID: %v", err)
			}
			if git.IsErrNotExist(err) || commitID != rel.Sha1 {
				if err := pushUpdateDeleteTag(repo, rel.TagName); err != nil {
					return fmt.Errorf("pushUpdateDeleteTag: %v", err)
				}
			} else {
				existingRelTags[strings.ToLower(rel.TagName)] = struct{}{}
			}
		}
	}
	tags, err := gitRepo.GetTags()
	if err != nil {
		return fmt.Errorf("GetTags: %v", err)
	}
	for _, tagName := range tags {
		if _, ok := existingRelTags[strings.ToLower(tagName)]; !ok {
			if err := pushUpdateAddTag(repo, gitRepo, tagName); err != nil {
				return fmt.Errorf("pushUpdateAddTag: %v", err)
			}
		}
	}
	return nil
}
