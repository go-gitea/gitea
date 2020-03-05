// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"html/template"

	// Needed for jpeg support
	_ "image/jpeg"
	"image/png"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/sync"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"github.com/mcuadros/go-version"
	"github.com/unknwon/com"
	"xorm.io/builder"
)

// RepoWorkingPool represents a working pool to order the parallel changes to the same repository
var RepoWorkingPool = sync.NewExclusivePool()

var (
	// ErrMirrorNotExist mirror does not exist error
	ErrMirrorNotExist = errors.New("Mirror does not exist")

	// ErrNameEmpty name is empty error
	ErrNameEmpty = errors.New("Name is empty")
)

var (
	// Gitignores contains the gitiginore files
	Gitignores []string

	// Licenses contains the license files
	Licenses []string

	// Readmes contains the readme files
	Readmes []string

	// LabelTemplates contains the label template files and the list of labels for each file
	LabelTemplates map[string]string

	// ItemsPerPage maximum items per page in forks, watchers and stars of a repo
	ItemsPerPage = 40
)

// loadRepoConfig loads the repository config
func loadRepoConfig() {
	// Load .gitignore and license files and readme templates.
	types := []string{"gitignore", "license", "readme", "label"}
	typeFiles := make([][]string, 4)
	for i, t := range types {
		files, err := options.Dir(t)
		if err != nil {
			log.Fatal("Failed to get %s files: %v", t, err)
		}
		customPath := path.Join(setting.CustomPath, "options", t)
		if com.IsDir(customPath) {
			customFiles, err := com.StatDir(customPath)
			if err != nil {
				log.Fatal("Failed to get custom %s files: %v", t, err)
			}

			for _, f := range customFiles {
				if !com.IsSliceContainsStr(files, f) {
					files = append(files, f)
				}
			}
		}
		typeFiles[i] = files
	}

	Gitignores = typeFiles[0]
	Licenses = typeFiles[1]
	Readmes = typeFiles[2]
	LabelTemplatesFiles := typeFiles[3]
	sort.Strings(Gitignores)
	sort.Strings(Licenses)
	sort.Strings(Readmes)
	sort.Strings(LabelTemplatesFiles)

	// Load label templates
	LabelTemplates = make(map[string]string)
	for _, templateFile := range LabelTemplatesFiles {
		labels, err := LoadLabelsFormatted(templateFile)
		if err != nil {
			log.Error("Failed to load labels: %v", err)
		}
		LabelTemplates[templateFile] = labels
	}

	// Filter out invalid names and promote preferred licenses.
	sortedLicenses := make([]string, 0, len(Licenses))
	for _, name := range setting.Repository.PreferredLicenses {
		if com.IsSliceContainsStr(Licenses, name) {
			sortedLicenses = append(sortedLicenses, name)
		}
	}
	for _, name := range Licenses {
		if !com.IsSliceContainsStr(setting.Repository.PreferredLicenses, name) {
			sortedLicenses = append(sortedLicenses, name)
		}
	}
	Licenses = sortedLicenses
}

// NewRepoContext creates a new repository context
func NewRepoContext() {
	loadRepoConfig()

	RemoveAllWithNotice("Clean up repository temporary data", filepath.Join(setting.AppDataPath, "tmp"))
}

// RepositoryStatus defines the status of repository
type RepositoryStatus int

// all kinds of RepositoryStatus
const (
	RepositoryReady         RepositoryStatus = iota // a normal repository
	RepositoryBeingMigrated                         // repository is migrating
)

// Repository represents a git repository.
type Repository struct {
	ID                  int64                  `xorm:"pk autoincr"`
	OwnerID             int64                  `xorm:"UNIQUE(s) index"`
	OwnerName           string                 `xorm:"-"`
	Owner               *User                  `xorm:"-"`
	LowerName           string                 `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name                string                 `xorm:"INDEX NOT NULL"`
	Description         string                 `xorm:"TEXT"`
	Website             string                 `xorm:"VARCHAR(2048)"`
	OriginalServiceType structs.GitServiceType `xorm:"index"`
	OriginalURL         string                 `xorm:"VARCHAR(2048)"`
	DefaultBranch       string

	NumWatches          int
	NumStars            int
	NumForks            int
	NumIssues           int
	NumClosedIssues     int
	NumOpenIssues       int `xorm:"-"`
	NumPulls            int
	NumClosedPulls      int
	NumOpenPulls        int `xorm:"-"`
	NumMilestones       int `xorm:"NOT NULL DEFAULT 0"`
	NumClosedMilestones int `xorm:"NOT NULL DEFAULT 0"`
	NumOpenMilestones   int `xorm:"-"`
	NumReleases         int `xorm:"-"`

	IsPrivate  bool `xorm:"INDEX"`
	IsEmpty    bool `xorm:"INDEX"`
	IsArchived bool `xorm:"INDEX"`
	IsMirror   bool `xorm:"INDEX"`
	*Mirror    `xorm:"-"`
	Status     RepositoryStatus `xorm:"NOT NULL DEFAULT 0"`

	RenderingMetas map[string]string `xorm:"-"`
	Units          []*RepoUnit       `xorm:"-"`

	IsFork                          bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	ForkID                          int64              `xorm:"INDEX"`
	BaseRepo                        *Repository        `xorm:"-"`
	IsTemplate                      bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	TemplateID                      int64              `xorm:"INDEX"`
	TemplateRepo                    *Repository        `xorm:"-"`
	Size                            int64              `xorm:"NOT NULL DEFAULT 0"`
	IndexerStatus                   *RepoIndexerStatus `xorm:"-"`
	IsFsckEnabled                   bool               `xorm:"NOT NULL DEFAULT true"`
	CloseIssuesViaCommitInAnyBranch bool               `xorm:"NOT NULL DEFAULT false"`
	Topics                          []string           `xorm:"TEXT JSON"`

	// Avatar: ID(10-20)-md5(32) - must fit into 64 symbols
	Avatar string `xorm:"VARCHAR(64)"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// SanitizedOriginalURL returns a sanitized OriginalURL
func (repo *Repository) SanitizedOriginalURL() string {
	if repo.OriginalURL == "" {
		return ""
	}
	return util.SanitizeURLCredentials(repo.OriginalURL, false)
}

// ColorFormat returns a colored string to represent this repo
func (repo *Repository) ColorFormat(s fmt.State) {
	var ownerName interface{}

	if repo.OwnerName != "" {
		ownerName = repo.OwnerName
	} else if repo.Owner != nil {
		ownerName = repo.Owner.Name
	} else {
		ownerName = log.NewColoredIDValue(strconv.FormatInt(repo.OwnerID, 10))
	}

	log.ColorFprintf(s, "%d:%s/%s",
		log.NewColoredIDValue(repo.ID),
		ownerName,
		repo.Name)
}

// IsBeingMigrated indicates that repository is being migtated
func (repo *Repository) IsBeingMigrated() bool {
	return repo.Status == RepositoryBeingMigrated
}

// IsBeingCreated indicates that repository is being migrated or forked
func (repo *Repository) IsBeingCreated() bool {
	return repo.IsBeingMigrated()
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (repo *Repository) AfterLoad() {
	// FIXME: use models migration to solve all at once.
	if len(repo.DefaultBranch) == 0 {
		repo.DefaultBranch = "master"
	}

	repo.NumOpenIssues = repo.NumIssues - repo.NumClosedIssues
	repo.NumOpenPulls = repo.NumPulls - repo.NumClosedPulls
	repo.NumOpenMilestones = repo.NumMilestones - repo.NumClosedMilestones
}

// MustOwner always returns a valid *User object to avoid
// conceptually impossible error handling.
// It creates a fake object that contains error details
// when error occurs.
func (repo *Repository) MustOwner() *User {
	return repo.mustOwner(x)
}

// MustOwnerName always returns valid owner name to avoid
// conceptually impossible error handling.
// It returns "error" and logs error details when error
// occurs.
func (repo *Repository) MustOwnerName() string {
	return repo.mustOwnerName(x)
}

// FullName returns the repository full name
func (repo *Repository) FullName() string {
	return repo.MustOwnerName() + "/" + repo.Name
}

// HTMLURL returns the repository HTML URL
func (repo *Repository) HTMLURL() string {
	return setting.AppURL + repo.FullName()
}

// APIURL returns the repository API URL
func (repo *Repository) APIURL() string {
	return setting.AppURL + path.Join("api/v1/repos", repo.FullName())
}

// APIFormat converts a Repository to api.Repository
func (repo *Repository) APIFormat(mode AccessMode) *api.Repository {
	return repo.innerAPIFormat(x, mode, false)
}

// GetCommitsCountCacheKey returns cache key used for commits count caching.
func (repo *Repository) GetCommitsCountCacheKey(contextName string, isRef bool) string {
	var prefix string
	if isRef {
		prefix = "ref"
	} else {
		prefix = "commit"
	}
	return fmt.Sprintf("commits-count-%d-%s-%s", repo.ID, prefix, contextName)
}

func (repo *Repository) innerAPIFormat(e Engine, mode AccessMode, isParent bool) *api.Repository {
	var parent *api.Repository

	cloneLink := repo.cloneLink(e, false)
	permission := &api.Permission{
		Admin: mode >= AccessModeAdmin,
		Push:  mode >= AccessModeWrite,
		Pull:  mode >= AccessModeRead,
	}
	if !isParent {
		err := repo.getBaseRepo(e)
		if err != nil {
			log.Error("APIFormat: %v", err)
		}
		if repo.BaseRepo != nil {
			parent = repo.BaseRepo.innerAPIFormat(e, mode, true)
		}
	}
	hasIssues := false
	var externalTracker *api.ExternalTracker
	var internalTracker *api.InternalTracker
	if unit, err := repo.getUnit(e, UnitTypeIssues); err == nil {
		config := unit.IssuesConfig()
		hasIssues = true
		internalTracker = &api.InternalTracker{
			EnableTimeTracker:                config.EnableTimetracker,
			AllowOnlyContributorsToTrackTime: config.AllowOnlyContributorsToTrackTime,
			EnableIssueDependencies:          config.EnableDependencies,
		}
	} else if unit, err := repo.getUnit(e, UnitTypeExternalTracker); err == nil {
		config := unit.ExternalTrackerConfig()
		hasIssues = true
		externalTracker = &api.ExternalTracker{
			ExternalTrackerURL:    config.ExternalTrackerURL,
			ExternalTrackerFormat: config.ExternalTrackerFormat,
			ExternalTrackerStyle:  config.ExternalTrackerStyle,
		}
	}
	hasWiki := false
	var externalWiki *api.ExternalWiki
	if _, err := repo.getUnit(e, UnitTypeWiki); err == nil {
		hasWiki = true
	} else if unit, err := repo.getUnit(e, UnitTypeExternalWiki); err == nil {
		hasWiki = true
		config := unit.ExternalWikiConfig()
		externalWiki = &api.ExternalWiki{
			ExternalWikiURL: config.ExternalWikiURL,
		}
	}
	hasPullRequests := false
	ignoreWhitespaceConflicts := false
	allowMerge := false
	allowRebase := false
	allowRebaseMerge := false
	allowSquash := false
	if unit, err := repo.getUnit(e, UnitTypePullRequests); err == nil {
		config := unit.PullRequestsConfig()
		hasPullRequests = true
		ignoreWhitespaceConflicts = config.IgnoreWhitespaceConflicts
		allowMerge = config.AllowMerge
		allowRebase = config.AllowRebase
		allowRebaseMerge = config.AllowRebaseMerge
		allowSquash = config.AllowSquash
	}

	return &api.Repository{
		ID:                        repo.ID,
		Owner:                     repo.Owner.APIFormat(),
		Name:                      repo.Name,
		FullName:                  repo.FullName(),
		Description:               repo.Description,
		Private:                   repo.IsPrivate,
		Template:                  repo.IsTemplate,
		Empty:                     repo.IsEmpty,
		Archived:                  repo.IsArchived,
		Size:                      int(repo.Size / 1024),
		Fork:                      repo.IsFork,
		Parent:                    parent,
		Mirror:                    repo.IsMirror,
		HTMLURL:                   repo.HTMLURL(),
		SSHURL:                    cloneLink.SSH,
		CloneURL:                  cloneLink.HTTPS,
		Website:                   repo.Website,
		Stars:                     repo.NumStars,
		Forks:                     repo.NumForks,
		Watchers:                  repo.NumWatches,
		OpenIssues:                repo.NumOpenIssues,
		OpenPulls:                 repo.NumOpenPulls,
		Releases:                  repo.NumReleases,
		DefaultBranch:             repo.DefaultBranch,
		Created:                   repo.CreatedUnix.AsTime(),
		Updated:                   repo.UpdatedUnix.AsTime(),
		Permissions:               permission,
		HasIssues:                 hasIssues,
		ExternalTracker:           externalTracker,
		InternalTracker:           internalTracker,
		HasWiki:                   hasWiki,
		ExternalWiki:              externalWiki,
		HasPullRequests:           hasPullRequests,
		IgnoreWhitespaceConflicts: ignoreWhitespaceConflicts,
		AllowMerge:                allowMerge,
		AllowRebase:               allowRebase,
		AllowRebaseMerge:          allowRebaseMerge,
		AllowSquash:               allowSquash,
		AvatarURL:                 repo.avatarLink(e),
	}
}

func (repo *Repository) getUnits(e Engine) (err error) {
	if repo.Units != nil {
		return nil
	}

	repo.Units, err = getUnitsByRepoID(e, repo.ID)
	return err
}

// CheckUnitUser check whether user could visit the unit of this repository
func (repo *Repository) CheckUnitUser(userID int64, isAdmin bool, unitType UnitType) bool {
	return repo.checkUnitUser(x, userID, isAdmin, unitType)
}

func (repo *Repository) checkUnitUser(e Engine, userID int64, isAdmin bool, unitType UnitType) bool {
	if isAdmin {
		return true
	}
	user, err := getUserByID(e, userID)
	if err != nil {
		return false
	}
	perm, err := getUserRepoPermission(e, repo, user)
	if err != nil {
		return false
	}

	return perm.CanRead(unitType)
}

// UnitEnabled if this repository has the given unit enabled
func (repo *Repository) UnitEnabled(tp UnitType) bool {
	if err := repo.getUnits(x); err != nil {
		log.Warn("Error loading repository (ID: %d) units: %s", repo.ID, err.Error())
	}
	for _, unit := range repo.Units {
		if unit.Type == tp {
			return true
		}
	}
	return false
}

// ErrUnitTypeNotExist represents a "UnitTypeNotExist" kind of error.
type ErrUnitTypeNotExist struct {
	UT UnitType
}

// IsErrUnitTypeNotExist checks if an error is a ErrUnitNotExist.
func IsErrUnitTypeNotExist(err error) bool {
	_, ok := err.(ErrUnitTypeNotExist)
	return ok
}

func (err ErrUnitTypeNotExist) Error() string {
	return fmt.Sprintf("Unit type does not exist: %s", err.UT.String())
}

// MustGetUnit always returns a RepoUnit object
func (repo *Repository) MustGetUnit(tp UnitType) *RepoUnit {
	ru, err := repo.GetUnit(tp)
	if err == nil {
		return ru
	}

	if tp == UnitTypeExternalWiki {
		return &RepoUnit{
			Type:   tp,
			Config: new(ExternalWikiConfig),
		}
	} else if tp == UnitTypeExternalTracker {
		return &RepoUnit{
			Type:   tp,
			Config: new(ExternalTrackerConfig),
		}
	} else if tp == UnitTypePullRequests {
		return &RepoUnit{
			Type:   tp,
			Config: new(PullRequestsConfig),
		}
	} else if tp == UnitTypeIssues {
		return &RepoUnit{
			Type:   tp,
			Config: new(IssuesConfig),
		}
	}
	return &RepoUnit{
		Type:   tp,
		Config: new(UnitConfig),
	}
}

// GetUnit returns a RepoUnit object
func (repo *Repository) GetUnit(tp UnitType) (*RepoUnit, error) {
	return repo.getUnit(x, tp)
}

func (repo *Repository) getUnit(e Engine, tp UnitType) (*RepoUnit, error) {
	if err := repo.getUnits(e); err != nil {
		return nil, err
	}
	for _, unit := range repo.Units {
		if unit.Type == tp {
			return unit, nil
		}
	}
	return nil, ErrUnitTypeNotExist{tp}
}

func (repo *Repository) getOwner(e Engine) (err error) {
	if repo.Owner != nil {
		return nil
	}

	repo.Owner, err = getUserByID(e, repo.OwnerID)
	return err
}

// GetOwner returns the repository owner
func (repo *Repository) GetOwner() error {
	return repo.getOwner(x)
}

func (repo *Repository) mustOwner(e Engine) *User {
	if err := repo.getOwner(e); err != nil {
		return &User{
			Name:     "error",
			FullName: err.Error(),
		}
	}

	return repo.Owner
}

func (repo *Repository) getOwnerName(e Engine) error {
	if len(repo.OwnerName) > 0 {
		return nil
	}

	if repo.Owner != nil {
		repo.OwnerName = repo.Owner.Name
		return nil
	}

	u := new(User)
	has, err := e.ID(repo.OwnerID).Cols("name").Get(u)
	if err != nil {
		return err
	} else if !has {
		return ErrUserNotExist{repo.OwnerID, "", 0}
	}
	repo.OwnerName = u.Name
	return nil
}

// GetOwnerName returns the repository owner name
func (repo *Repository) GetOwnerName() error {
	return repo.getOwnerName(x)
}

func (repo *Repository) mustOwnerName(e Engine) string {
	if err := repo.getOwnerName(e); err != nil {
		log.Error("Error loading repository owner name: %v", err)
		return "error"
	}

	return repo.OwnerName
}

// ComposeMetas composes a map of metas for properly rendering issue links and external issue trackers.
func (repo *Repository) ComposeMetas() map[string]string {
	if repo.RenderingMetas == nil {
		metas := map[string]string{
			"user":     repo.MustOwner().Name,
			"repo":     repo.Name,
			"repoPath": repo.RepoPath(),
		}

		unit, err := repo.GetUnit(UnitTypeExternalTracker)
		if err == nil {
			metas["format"] = unit.ExternalTrackerConfig().ExternalTrackerFormat
			switch unit.ExternalTrackerConfig().ExternalTrackerStyle {
			case markup.IssueNameStyleAlphanumeric:
				metas["style"] = markup.IssueNameStyleAlphanumeric
			default:
				metas["style"] = markup.IssueNameStyleNumeric
			}
		}

		if repo.Owner.IsOrganization() {
			teams := make([]string, 0, 5)
			_ = x.Table("team_repo").
				Join("INNER", "team", "team.id = team_repo.team_id").
				Where("team_repo.repo_id = ?", repo.ID).
				Select("team.lower_name").
				OrderBy("team.lower_name").
				Find(&teams)
			metas["teams"] = "," + strings.Join(teams, ",") + ","
			metas["org"] = repo.Owner.LowerName
		}

		repo.RenderingMetas = metas
	}
	return repo.RenderingMetas
}

// DeleteWiki removes the actual and local copy of repository wiki.
func (repo *Repository) DeleteWiki() error {
	return repo.deleteWiki(x)
}

func (repo *Repository) deleteWiki(e Engine) error {
	wikiPaths := []string{repo.WikiPath()}
	for _, wikiPath := range wikiPaths {
		removeAllWithNotice(e, "Delete repository wiki", wikiPath)
	}

	_, err := e.Where("repo_id = ?", repo.ID).And("type = ?", UnitTypeWiki).Delete(new(RepoUnit))
	return err
}

func (repo *Repository) getAssignees(e Engine) (_ []*User, err error) {
	if err = repo.getOwner(e); err != nil {
		return nil, err
	}

	accesses := make([]*Access, 0, 10)
	if err = e.
		Where("repo_id = ? AND mode >= ?", repo.ID, AccessModeWrite).
		Find(&accesses); err != nil {
		return nil, err
	}

	// Leave a seat for owner itself to append later, but if owner is an organization
	// and just waste 1 unit is cheaper than re-allocate memory once.
	users := make([]*User, 0, len(accesses)+1)
	if len(accesses) > 0 {
		userIDs := make([]int64, len(accesses))
		for i := 0; i < len(accesses); i++ {
			userIDs[i] = accesses[i].UserID
		}

		if err = e.In("id", userIDs).Find(&users); err != nil {
			return nil, err
		}
	}
	if !repo.Owner.IsOrganization() {
		users = append(users, repo.Owner)
	}

	return users, nil
}

// GetAssignees returns all users that have write access and can be assigned to issues
// of the repository,
func (repo *Repository) GetAssignees() (_ []*User, err error) {
	return repo.getAssignees(x)
}

// GetMilestoneByID returns the milestone belongs to repository by given ID.
func (repo *Repository) GetMilestoneByID(milestoneID int64) (*Milestone, error) {
	return GetMilestoneByRepoID(repo.ID, milestoneID)
}

// IssueStats returns number of open and closed repository issues by given filter mode.
func (repo *Repository) IssueStats(uid int64, filterMode int, isPull bool) (int64, int64) {
	return GetRepoIssueStats(repo.ID, uid, filterMode, isPull)
}

// GetMirror sets the repository mirror, returns an error upon failure
func (repo *Repository) GetMirror() (err error) {
	repo.Mirror, err = GetMirrorByRepoID(repo.ID)
	return err
}

// GetBaseRepo populates repo.BaseRepo for a fork repository and
// returns an error on failure (NOTE: no error is returned for
// non-fork repositories, and BaseRepo will be left untouched)
func (repo *Repository) GetBaseRepo() (err error) {
	return repo.getBaseRepo(x)
}

func (repo *Repository) getBaseRepo(e Engine) (err error) {
	if !repo.IsFork {
		return nil
	}

	repo.BaseRepo, err = getRepositoryByID(e, repo.ForkID)
	return err
}

// IsGenerated returns whether _this_ repository was generated from a template
func (repo *Repository) IsGenerated() bool {
	return repo.TemplateID != 0
}

// GetTemplateRepo populates repo.TemplateRepo for a generated repository and
// returns an error on failure (NOTE: no error is returned for
// non-generated repositories, and TemplateRepo will be left untouched)
func (repo *Repository) GetTemplateRepo() (err error) {
	return repo.getTemplateRepo(x)
}

func (repo *Repository) getTemplateRepo(e Engine) (err error) {
	if !repo.IsGenerated() {
		return nil
	}

	repo.TemplateRepo, err = getRepositoryByID(e, repo.TemplateID)
	return err
}

func (repo *Repository) repoPath(e Engine) string {
	return RepoPath(repo.mustOwnerName(e), repo.Name)
}

// RepoPath returns the repository path
func (repo *Repository) RepoPath() string {
	return repo.repoPath(x)
}

// GitConfigPath returns the path to a repository's git config/ directory
func GitConfigPath(repoPath string) string {
	return filepath.Join(repoPath, "config")
}

// GitConfigPath returns the repository git config path
func (repo *Repository) GitConfigPath() string {
	return GitConfigPath(repo.RepoPath())
}

// RelLink returns the repository relative link
func (repo *Repository) RelLink() string {
	return "/" + repo.FullName()
}

// Link returns the repository link
func (repo *Repository) Link() string {
	return setting.AppSubURL + "/" + repo.FullName()
}

// ComposeCompareURL returns the repository comparison URL
func (repo *Repository) ComposeCompareURL(oldCommitID, newCommitID string) string {
	return fmt.Sprintf("%s/%s/compare/%s...%s", repo.MustOwner().Name, repo.Name, oldCommitID, newCommitID)
}

// UpdateDefaultBranch updates the default branch
func (repo *Repository) UpdateDefaultBranch() error {
	_, err := x.ID(repo.ID).Cols("default_branch").Update(repo)
	return err
}

// IsOwnedBy returns true when user owns this repository
func (repo *Repository) IsOwnedBy(userID int64) bool {
	return repo.OwnerID == userID
}

func (repo *Repository) updateSize(e Engine) error {
	size, err := util.GetDirectorySize(repo.repoPath(e))
	if err != nil {
		return fmt.Errorf("UpdateSize: %v", err)
	}

	repo.Size = size
	_, err = e.ID(repo.ID).Cols("size").Update(repo)
	return err
}

// UpdateSize updates the repository size, calculating it using util.GetDirectorySize
func (repo *Repository) UpdateSize() error {
	return repo.updateSize(x)
}

// CanUserFork returns true if specified user can fork repository.
func (repo *Repository) CanUserFork(user *User) (bool, error) {
	if user == nil {
		return false, nil
	}
	if repo.OwnerID != user.ID && !user.HasForkedRepo(repo.ID) {
		return true, nil
	}
	if err := user.GetOwnedOrganizations(); err != nil {
		return false, err
	}
	for _, org := range user.OwnedOrgs {
		if repo.OwnerID != org.ID && !org.HasForkedRepo(repo.ID) {
			return true, nil
		}
	}
	return false, nil
}

// CanUserDelete returns true if user could delete the repository
func (repo *Repository) CanUserDelete(user *User) (bool, error) {
	if user.IsAdmin || user.ID == repo.OwnerID {
		return true, nil
	}

	if err := repo.GetOwner(); err != nil {
		return false, err
	}

	if repo.Owner.IsOrganization() {
		isOwner, err := repo.Owner.IsOwnedBy(user.ID)
		if err != nil {
			return false, err
		} else if isOwner {
			return true, nil
		}
	}

	return false, nil
}

// CanEnablePulls returns true if repository meets the requirements of accepting pulls.
func (repo *Repository) CanEnablePulls() bool {
	return !repo.IsMirror && !repo.IsEmpty
}

// AllowsPulls returns true if repository meets the requirements of accepting pulls and has them enabled.
func (repo *Repository) AllowsPulls() bool {
	return repo.CanEnablePulls() && repo.UnitEnabled(UnitTypePullRequests)
}

// CanEnableEditor returns true if repository meets the requirements of web editor.
func (repo *Repository) CanEnableEditor() bool {
	return !repo.IsMirror
}

// GetReaders returns all users that have explicit read access or higher to the repository.
func (repo *Repository) GetReaders() (_ []*User, err error) {
	return repo.getUsersWithAccessMode(x, AccessModeRead)
}

// GetWriters returns all users that have write access to the repository.
func (repo *Repository) GetWriters() (_ []*User, err error) {
	return repo.getUsersWithAccessMode(x, AccessModeWrite)
}

// IsReader returns true if user has explicit read access or higher to the repository.
func (repo *Repository) IsReader(userID int64) (bool, error) {
	if repo.OwnerID == userID {
		return true, nil
	}
	return x.Where("repo_id = ? AND user_id = ? AND mode >= ?", repo.ID, userID, AccessModeRead).Get(&Access{})
}

// getUsersWithAccessMode returns users that have at least given access mode to the repository.
func (repo *Repository) getUsersWithAccessMode(e Engine, mode AccessMode) (_ []*User, err error) {
	if err = repo.getOwner(e); err != nil {
		return nil, err
	}

	accesses := make([]*Access, 0, 10)
	if err = e.Where("repo_id = ? AND mode >= ?", repo.ID, mode).Find(&accesses); err != nil {
		return nil, err
	}

	// Leave a seat for owner itself to append later, but if owner is an organization
	// and just waste 1 unit is cheaper than re-allocate memory once.
	users := make([]*User, 0, len(accesses)+1)
	if len(accesses) > 0 {
		userIDs := make([]int64, len(accesses))
		for i := 0; i < len(accesses); i++ {
			userIDs[i] = accesses[i].UserID
		}

		if err = e.In("id", userIDs).Find(&users); err != nil {
			return nil, err
		}
	}
	if !repo.Owner.IsOrganization() {
		users = append(users, repo.Owner)
	}

	return users, nil
}

// DescriptionHTML does special handles to description and return HTML string.
func (repo *Repository) DescriptionHTML() template.HTML {
	desc, err := markup.RenderDescriptionHTML([]byte(repo.Description), repo.HTMLURL(), repo.ComposeMetas())
	if err != nil {
		log.Error("Failed to render description for %s (ID: %d): %v", repo.Name, repo.ID, err)
		return template.HTML(markup.Sanitize(repo.Description))
	}
	return template.HTML(markup.Sanitize(string(desc)))
}

func isRepositoryExist(e Engine, u *User, repoName string) (bool, error) {
	has, err := e.Get(&Repository{
		OwnerID:   u.ID,
		LowerName: strings.ToLower(repoName),
	})
	return has && com.IsDir(RepoPath(u.Name, repoName)), err
}

// IsRepositoryExist returns true if the repository with given name under user has already existed.
func IsRepositoryExist(u *User, repoName string) (bool, error) {
	return isRepositoryExist(x, u, repoName)
}

// CloneLink represents different types of clone URLs of repository.
type CloneLink struct {
	SSH   string
	HTTPS string
	Git   string
}

// ComposeHTTPSCloneURL returns HTTPS clone URL based on given owner and repository name.
func ComposeHTTPSCloneURL(owner, repo string) string {
	return fmt.Sprintf("%s%s/%s.git", setting.AppURL, url.PathEscape(owner), url.PathEscape(repo))
}

func (repo *Repository) cloneLink(e Engine, isWiki bool) *CloneLink {
	repoName := repo.Name
	if isWiki {
		repoName += ".wiki"
	}

	sshUser := setting.RunUser
	if setting.SSH.StartBuiltinServer {
		sshUser = setting.SSH.BuiltinServerUser
	}

	repo.Owner = repo.mustOwner(e)
	cl := new(CloneLink)
	if setting.SSH.Port != 22 {
		cl.SSH = fmt.Sprintf("ssh://%s@%s:%d/%s/%s.git", sshUser, setting.SSH.Domain, setting.SSH.Port, repo.Owner.Name, repoName)
	} else if setting.Repository.UseCompatSSHURI {
		cl.SSH = fmt.Sprintf("ssh://%s@%s/%s/%s.git", sshUser, setting.SSH.Domain, repo.Owner.Name, repoName)
	} else {
		cl.SSH = fmt.Sprintf("%s@%s:%s/%s.git", sshUser, setting.SSH.Domain, repo.Owner.Name, repoName)
	}
	cl.HTTPS = ComposeHTTPSCloneURL(repo.Owner.Name, repoName)
	return cl
}

// CloneLink returns clone URLs of repository.
func (repo *Repository) CloneLink() (cl *CloneLink) {
	return repo.cloneLink(x, false)
}

// CheckCreateRepository check if could created a repository
func CheckCreateRepository(doer, u *User, name string) error {
	if !doer.CanCreateRepo() {
		return ErrReachLimitOfRepo{u.MaxRepoCreation}
	}

	if err := IsUsableRepoName(name); err != nil {
		return err
	}

	has, err := isRepositoryExist(x, u, name)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return ErrRepoAlreadyExist{u.Name, name}
	}
	return nil
}

// CreateDelegateHooks creates all the hooks scripts for the repo
func CreateDelegateHooks(repoPath string) error {
	return createDelegateHooks(repoPath)
}

// createDelegateHooks creates all the hooks scripts for the repo
func createDelegateHooks(repoPath string) (err error) {

	var (
		hookNames = []string{"pre-receive", "update", "post-receive"}
		hookTpls  = []string{
			fmt.Sprintf("#!/usr/bin/env %s\ndata=$(cat)\nexitcodes=\"\"\nhookname=$(basename $0)\nGIT_DIR=${GIT_DIR:-$(dirname $0)}\n\nfor hook in ${GIT_DIR}/hooks/${hookname}.d/*; do\ntest -x \"${hook}\" || continue\necho \"${data}\" | \"${hook}\"\nexitcodes=\"${exitcodes} $?\"\ndone\n\nfor i in ${exitcodes}; do\n[ ${i} -eq 0 ] || exit ${i}\ndone\n", setting.ScriptType),
			fmt.Sprintf("#!/usr/bin/env %s\nexitcodes=\"\"\nhookname=$(basename $0)\nGIT_DIR=${GIT_DIR:-$(dirname $0)}\n\nfor hook in ${GIT_DIR}/hooks/${hookname}.d/*; do\ntest -x \"${hook}\" || continue\n\"${hook}\" $1 $2 $3\nexitcodes=\"${exitcodes} $?\"\ndone\n\nfor i in ${exitcodes}; do\n[ ${i} -eq 0 ] || exit ${i}\ndone\n", setting.ScriptType),
			fmt.Sprintf("#!/usr/bin/env %s\ndata=$(cat)\nexitcodes=\"\"\nhookname=$(basename $0)\nGIT_DIR=${GIT_DIR:-$(dirname $0)}\n\nfor hook in ${GIT_DIR}/hooks/${hookname}.d/*; do\ntest -x \"${hook}\" || continue\necho \"${data}\" | \"${hook}\"\nexitcodes=\"${exitcodes} $?\"\ndone\n\nfor i in ${exitcodes}; do\n[ ${i} -eq 0 ] || exit ${i}\ndone\n", setting.ScriptType),
		}
		giteaHookTpls = []string{
			fmt.Sprintf("#!/usr/bin/env %s\n\"%s\" hook --config='%s' pre-receive\n", setting.ScriptType, setting.AppPath, setting.CustomConf),
			fmt.Sprintf("#!/usr/bin/env %s\n\"%s\" hook --config='%s' update $1 $2 $3\n", setting.ScriptType, setting.AppPath, setting.CustomConf),
			fmt.Sprintf("#!/usr/bin/env %s\n\"%s\" hook --config='%s' post-receive\n", setting.ScriptType, setting.AppPath, setting.CustomConf),
		}
	)

	hookDir := filepath.Join(repoPath, "hooks")

	for i, hookName := range hookNames {
		oldHookPath := filepath.Join(hookDir, hookName)
		newHookPath := filepath.Join(hookDir, hookName+".d", "gitea")

		if err := os.MkdirAll(filepath.Join(hookDir, hookName+".d"), os.ModePerm); err != nil {
			return fmt.Errorf("create hooks dir '%s': %v", filepath.Join(hookDir, hookName+".d"), err)
		}

		// WARNING: This will override all old server-side hooks
		if err = os.Remove(oldHookPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("unable to pre-remove old hook file '%s' prior to rewriting: %v ", oldHookPath, err)
		}
		if err = ioutil.WriteFile(oldHookPath, []byte(hookTpls[i]), 0777); err != nil {
			return fmt.Errorf("write old hook file '%s': %v", oldHookPath, err)
		}

		if err = os.Remove(newHookPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("unable to pre-remove new hook file '%s' prior to rewriting: %v", newHookPath, err)
		}
		if err = ioutil.WriteFile(newHookPath, []byte(giteaHookTpls[i]), 0777); err != nil {
			return fmt.Errorf("write new hook file '%s': %v", newHookPath, err)
		}
	}

	return nil
}

// initRepoCommit temporarily changes with work directory.
func initRepoCommit(tmpPath string, repo *Repository, u *User) (err error) {
	commitTimeStr := time.Now().Format(time.RFC3339)

	sig := u.NewGitSig()
	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+sig.Name,
		"GIT_AUTHOR_EMAIL="+sig.Email,
		"GIT_AUTHOR_DATE="+commitTimeStr,
		"GIT_COMMITTER_NAME="+sig.Name,
		"GIT_COMMITTER_EMAIL="+sig.Email,
		"GIT_COMMITTER_DATE="+commitTimeStr,
	)

	if stdout, err := git.NewCommand("add", "--all").
		SetDescription(fmt.Sprintf("initRepoCommit (git add): %s", tmpPath)).
		RunInDir(tmpPath); err != nil {
		log.Error("git add --all failed: Stdout: %s\nError: %v", stdout, err)
		return fmt.Errorf("git add --all: %v", err)
	}

	binVersion, err := git.BinVersion()
	if err != nil {
		return fmt.Errorf("Unable to get git version: %v", err)
	}

	args := []string{
		"commit", fmt.Sprintf("--author='%s <%s>'", sig.Name, sig.Email),
		"-m", "Initial commit",
	}

	if version.Compare(binVersion, "1.7.9", ">=") {
		sign, keyID := SignInitialCommit(tmpPath, u)
		if sign {
			args = append(args, "-S"+keyID)
		} else if version.Compare(binVersion, "2.0.0", ">=") {
			args = append(args, "--no-gpg-sign")
		}
	}

	if stdout, err := git.NewCommand(args...).
		SetDescription(fmt.Sprintf("initRepoCommit (git commit): %s", tmpPath)).
		RunInDirWithEnv(tmpPath, env); err != nil {
		log.Error("Failed to commit: %v: Stdout: %s\nError: %v", args, stdout, err)
		return fmt.Errorf("git commit: %v", err)
	}

	if stdout, err := git.NewCommand("push", "origin", "master").
		SetDescription(fmt.Sprintf("initRepoCommit (git push): %s", tmpPath)).
		RunInDirWithEnv(tmpPath, InternalPushingEnvironment(u, repo)); err != nil {
		log.Error("Failed to push back to master: Stdout: %s\nError: %v", stdout, err)
		return fmt.Errorf("git push: %v", err)
	}

	return nil
}

// CreateRepoOptions contains the create repository options
type CreateRepoOptions struct {
	Name        string
	Description string
	OriginalURL string
	Gitignores  string
	IssueLabels string
	License     string
	Readme      string
	IsPrivate   bool
	IsMirror    bool
	AutoInit    bool
	Status      RepositoryStatus
}

func getRepoInitFile(tp, name string) ([]byte, error) {
	cleanedName := strings.TrimLeft(path.Clean("/"+name), "/")
	relPath := path.Join("options", tp, cleanedName)

	// Use custom file when available.
	customPath := path.Join(setting.CustomPath, relPath)
	if com.IsFile(customPath) {
		return ioutil.ReadFile(customPath)
	}

	switch tp {
	case "readme":
		return options.Readme(cleanedName)
	case "gitignore":
		return options.Gitignore(cleanedName)
	case "license":
		return options.License(cleanedName)
	case "label":
		return options.Labels(cleanedName)
	default:
		return []byte{}, fmt.Errorf("Invalid init file type")
	}
}

func prepareRepoCommit(e Engine, repo *Repository, tmpDir, repoPath string, opts CreateRepoOptions) error {
	commitTimeStr := time.Now().Format(time.RFC3339)
	authorSig := repo.Owner.NewGitSig()

	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+authorSig.Name,
		"GIT_AUTHOR_EMAIL="+authorSig.Email,
		"GIT_AUTHOR_DATE="+commitTimeStr,
		"GIT_COMMITTER_NAME="+authorSig.Name,
		"GIT_COMMITTER_EMAIL="+authorSig.Email,
		"GIT_COMMITTER_DATE="+commitTimeStr,
	)

	// Clone to temporary path and do the init commit.
	if stdout, err := git.NewCommand("clone", repoPath, tmpDir).
		SetDescription(fmt.Sprintf("initRepository (git clone): %s to %s", repoPath, tmpDir)).
		RunInDirWithEnv("", env); err != nil {
		log.Error("Failed to clone from %v into %s: stdout: %s\nError: %v", repo, tmpDir, stdout, err)
		return fmt.Errorf("git clone: %v", err)
	}

	// README
	data, err := getRepoInitFile("readme", opts.Readme)
	if err != nil {
		return fmt.Errorf("getRepoInitFile[%s]: %v", opts.Readme, err)
	}

	cloneLink := repo.cloneLink(e, false)
	match := map[string]string{
		"Name":           repo.Name,
		"Description":    repo.Description,
		"CloneURL.SSH":   cloneLink.SSH,
		"CloneURL.HTTPS": cloneLink.HTTPS,
	}
	if err = ioutil.WriteFile(filepath.Join(tmpDir, "README.md"),
		[]byte(com.Expand(string(data), match)), 0644); err != nil {
		return fmt.Errorf("write README.md: %v", err)
	}

	// .gitignore
	if len(opts.Gitignores) > 0 {
		var buf bytes.Buffer
		names := strings.Split(opts.Gitignores, ",")
		for _, name := range names {
			data, err = getRepoInitFile("gitignore", name)
			if err != nil {
				return fmt.Errorf("getRepoInitFile[%s]: %v", name, err)
			}
			buf.WriteString("# ---> " + name + "\n")
			buf.Write(data)
			buf.WriteString("\n")
		}

		if buf.Len() > 0 {
			if err = ioutil.WriteFile(filepath.Join(tmpDir, ".gitignore"), buf.Bytes(), 0644); err != nil {
				return fmt.Errorf("write .gitignore: %v", err)
			}
		}
	}

	// LICENSE
	if len(opts.License) > 0 {
		data, err = getRepoInitFile("license", opts.License)
		if err != nil {
			return fmt.Errorf("getRepoInitFile[%s]: %v", opts.License, err)
		}

		if err = ioutil.WriteFile(filepath.Join(tmpDir, "LICENSE"), data, 0644); err != nil {
			return fmt.Errorf("write LICENSE: %v", err)
		}
	}

	return nil
}

func checkInitRepository(repoPath string) (err error) {
	// Somehow the directory could exist.
	if com.IsExist(repoPath) {
		return fmt.Errorf("initRepository: path already exists: %s", repoPath)
	}

	// Init git bare new repository.
	if err = git.InitRepository(repoPath, true); err != nil {
		return fmt.Errorf("InitRepository: %v", err)
	} else if err = createDelegateHooks(repoPath); err != nil {
		return fmt.Errorf("createDelegateHooks: %v", err)
	}
	return nil
}

// InitRepository initializes README and .gitignore if needed.
func initRepository(e Engine, repoPath string, u *User, repo *Repository, opts CreateRepoOptions) (err error) {
	if err = checkInitRepository(repoPath); err != nil {
		return err
	}

	// Initialize repository according to user's choice.
	if opts.AutoInit {
		tmpDir, err := ioutil.TempDir(os.TempDir(), "gitea-"+repo.Name)
		if err != nil {
			return fmt.Errorf("Failed to create temp dir for repository %s: %v", repo.repoPath(e), err)
		}

		defer os.RemoveAll(tmpDir)

		if err = prepareRepoCommit(e, repo, tmpDir, repoPath, opts); err != nil {
			return fmt.Errorf("prepareRepoCommit: %v", err)
		}

		// Apply changes and commit.
		if err = initRepoCommit(tmpDir, repo, u); err != nil {
			return fmt.Errorf("initRepoCommit: %v", err)
		}
	}

	// Re-fetch the repository from database before updating it (else it would
	// override changes that were done earlier with sql)
	if repo, err = getRepositoryByID(e, repo.ID); err != nil {
		return fmt.Errorf("getRepositoryByID: %v", err)
	}

	if !opts.AutoInit {
		repo.IsEmpty = true
	}

	repo.DefaultBranch = "master"
	if err = updateRepository(e, repo, false); err != nil {
		return fmt.Errorf("updateRepository: %v", err)
	}

	return nil
}

var (
	reservedRepoNames    = []string{".", ".."}
	reservedRepoPatterns = []string{"*.git", "*.wiki"}
)

// IsUsableRepoName returns true when repository is usable
func IsUsableRepoName(name string) error {
	return isUsableName(reservedRepoNames, reservedRepoPatterns, name)
}

func createRepository(e Engine, doer, u *User, repo *Repository) (err error) {
	if err = IsUsableRepoName(repo.Name); err != nil {
		return err
	}

	has, err := isRepositoryExist(e, u, repo.Name)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return ErrRepoAlreadyExist{u.Name, repo.Name}
	}

	if _, err = e.Insert(repo); err != nil {
		return err
	}
	if err = deleteRepoRedirect(e, u.ID, repo.Name); err != nil {
		return err
	}

	// insert units for repo
	var units = make([]RepoUnit, 0, len(DefaultRepoUnits))
	for _, tp := range DefaultRepoUnits {
		if tp == UnitTypeIssues {
			units = append(units, RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
				Config: &IssuesConfig{
					EnableTimetracker:                setting.Service.DefaultEnableTimetracking,
					AllowOnlyContributorsToTrackTime: setting.Service.DefaultAllowOnlyContributorsToTrackTime,
					EnableDependencies:               setting.Service.DefaultEnableDependencies,
				},
			})
		} else if tp == UnitTypePullRequests {
			units = append(units, RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
				Config: &PullRequestsConfig{AllowMerge: true, AllowRebase: true, AllowRebaseMerge: true, AllowSquash: true},
			})
		} else {
			units = append(units, RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
			})
		}

	}

	if _, err = e.Insert(&units); err != nil {
		return err
	}

	// Remember visibility preference.
	u.LastRepoVisibility = repo.IsPrivate
	if err = updateUserCols(e, u, "last_repo_visibility"); err != nil {
		return fmt.Errorf("updateUser: %v", err)
	}

	if _, err = e.Incr("num_repos").ID(u.ID).Update(new(User)); err != nil {
		return fmt.Errorf("increment user total_repos: %v", err)
	}
	u.NumRepos++

	// Give access to all members in teams with access to all repositories.
	if u.IsOrganization() {
		if err := u.GetTeams(); err != nil {
			return fmt.Errorf("GetTeams: %v", err)
		}
		for _, t := range u.Teams {
			if t.IncludesAllRepositories {
				if err := t.addRepository(e, repo); err != nil {
					return fmt.Errorf("addRepository: %v", err)
				}
			}
		}

		if isAdmin, err := isUserRepoAdmin(e, repo, doer); err != nil {
			return fmt.Errorf("isUserRepoAdmin: %v", err)
		} else if !isAdmin {
			// Make creator repo admin if it wan't assigned automatically
			if err = repo.addCollaborator(e, doer); err != nil {
				return fmt.Errorf("AddCollaborator: %v", err)
			}
			if err = repo.changeCollaborationAccessMode(e, doer.ID, AccessModeAdmin); err != nil {
				return fmt.Errorf("ChangeCollaborationAccessMode: %v", err)
			}
		}
	} else if err = repo.recalculateAccesses(e); err != nil {
		// Organization automatically called this in addRepository method.
		return fmt.Errorf("recalculateAccesses: %v", err)
	}

	if setting.Service.AutoWatchNewRepos {
		if err = watchRepo(e, doer.ID, repo.ID, true); err != nil {
			return fmt.Errorf("watchRepo: %v", err)
		}
	}

	if err = copyDefaultWebhooksToRepo(e, repo.ID); err != nil {
		return fmt.Errorf("copyDefaultWebhooksToRepo: %v", err)
	}

	return nil
}

// CreateRepository creates a repository for the user/organization.
func CreateRepository(doer, u *User, opts CreateRepoOptions) (_ *Repository, err error) {
	if !doer.IsAdmin && !u.CanCreateRepo() {
		return nil, ErrReachLimitOfRepo{u.MaxRepoCreation}
	}

	repo := &Repository{
		OwnerID:                         u.ID,
		Owner:                           u,
		Name:                            opts.Name,
		LowerName:                       strings.ToLower(opts.Name),
		Description:                     opts.Description,
		OriginalURL:                     opts.OriginalURL,
		IsPrivate:                       opts.IsPrivate,
		IsFsckEnabled:                   !opts.IsMirror,
		CloseIssuesViaCommitInAnyBranch: setting.Repository.DefaultCloseIssuesViaCommitsInAnyBranch,
		Status:                          opts.Status,
		IsEmpty:                         !opts.AutoInit,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	if err = createRepository(sess, doer, u, repo); err != nil {
		return nil, err
	}

	// No need for init mirror.
	if !opts.IsMirror {
		repoPath := RepoPath(u.Name, repo.Name)
		if err = initRepository(sess, repoPath, u, repo, opts); err != nil {
			if err2 := os.RemoveAll(repoPath); err2 != nil {
				log.Error("initRepository: %v", err)
				return nil, fmt.Errorf(
					"delete repo directory %s/%s failed(2): %v", u.Name, repo.Name, err2)
			}
			return nil, fmt.Errorf("initRepository: %v", err)
		}

		// Initialize Issue Labels if selected
		if len(opts.IssueLabels) > 0 {
			if err = initalizeLabels(sess, repo.ID, opts.IssueLabels); err != nil {
				return nil, fmt.Errorf("initalizeLabels: %v", err)
			}
		}

		if stdout, err := git.NewCommand("update-server-info").
			SetDescription(fmt.Sprintf("CreateRepository(git update-server-info): %s", repoPath)).
			RunInDir(repoPath); err != nil {
			log.Error("CreateRepitory(git update-server-info) in %v: Stdout: %s\nError: %v", repo, stdout, err)
			return nil, fmt.Errorf("CreateRepository(git update-server-info): %v", err)
		}
	}

	if err = sess.Commit(); err != nil {
		return nil, err
	}

	return repo, err
}

func countRepositories(userID int64, private bool) int64 {
	sess := x.Where("id > 0")

	if userID > 0 {
		sess.And("owner_id = ?", userID)
	}
	if !private {
		sess.And("is_private=?", false)
	}

	count, err := sess.Count(new(Repository))
	if err != nil {
		log.Error("countRepositories: %v", err)
	}
	return count
}

// CountRepositories returns number of repositories.
// Argument private only takes effect when it is false,
// set it true to count all repositories.
func CountRepositories(private bool) int64 {
	return countRepositories(-1, private)
}

// CountUserRepositories returns number of repositories user owns.
// Argument private only takes effect when it is false,
// set it true to count all repositories.
func CountUserRepositories(userID int64, private bool) int64 {
	return countRepositories(userID, private)
}

// RepoPath returns repository path by given user and repository name.
func RepoPath(userName, repoName string) string {
	return filepath.Join(UserPath(userName), strings.ToLower(repoName)+".git")
}

// TransferOwnership transfers all corresponding setting from old user to new one.
func TransferOwnership(doer *User, newOwnerName string, repo *Repository) error {
	newOwner, err := GetUserByName(newOwnerName)
	if err != nil {
		return fmt.Errorf("get new owner '%s': %v", newOwnerName, err)
	}

	// Check if new owner has repository with same name.
	has, err := IsRepositoryExist(newOwner, repo.Name)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return ErrRepoAlreadyExist{newOwnerName, repo.Name}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return fmt.Errorf("sess.Begin: %v", err)
	}

	oldOwner := repo.Owner

	// Note: we have to set value here to make sure recalculate accesses is based on
	// new owner.
	repo.OwnerID = newOwner.ID
	repo.Owner = newOwner

	// Update repository.
	if _, err := sess.ID(repo.ID).Update(repo); err != nil {
		return fmt.Errorf("update owner: %v", err)
	}

	// Remove redundant collaborators.
	collaborators, err := repo.getCollaborators(sess)
	if err != nil {
		return fmt.Errorf("getCollaborators: %v", err)
	}

	// Dummy object.
	collaboration := &Collaboration{RepoID: repo.ID}
	for _, c := range collaborators {
		if c.ID != newOwner.ID {
			isMember, err := isOrganizationMember(sess, newOwner.ID, c.ID)
			if err != nil {
				return fmt.Errorf("IsOrgMember: %v", err)
			} else if !isMember {
				continue
			}
		}
		collaboration.UserID = c.ID
		if _, err = sess.Delete(collaboration); err != nil {
			return fmt.Errorf("remove collaborator '%d': %v", c.ID, err)
		}
	}

	// Remove old team-repository relations.
	if oldOwner.IsOrganization() {
		if err = oldOwner.removeOrgRepo(sess, repo.ID); err != nil {
			return fmt.Errorf("removeOrgRepo: %v", err)
		}
	}

	if newOwner.IsOrganization() {
		if err := newOwner.GetTeams(); err != nil {
			return fmt.Errorf("GetTeams: %v", err)
		}
		for _, t := range newOwner.Teams {
			if t.IncludesAllRepositories {
				if err := t.addRepository(sess, repo); err != nil {
					return fmt.Errorf("addRepository: %v", err)
				}
			}
		}
	} else if err = repo.recalculateAccesses(sess); err != nil {
		// Organization called this in addRepository method.
		return fmt.Errorf("recalculateAccesses: %v", err)
	}

	// Update repository count.
	if _, err = sess.Exec("UPDATE `user` SET num_repos=num_repos+1 WHERE id=?", newOwner.ID); err != nil {
		return fmt.Errorf("increase new owner repository count: %v", err)
	} else if _, err = sess.Exec("UPDATE `user` SET num_repos=num_repos-1 WHERE id=?", oldOwner.ID); err != nil {
		return fmt.Errorf("decrease old owner repository count: %v", err)
	}

	if err = watchRepo(sess, doer.ID, repo.ID, true); err != nil {
		return fmt.Errorf("watchRepo: %v", err)
	}

	// Remove watch for organization.
	if oldOwner.IsOrganization() {
		if err = watchRepo(sess, oldOwner.ID, repo.ID, false); err != nil {
			return fmt.Errorf("watchRepo [false]: %v", err)
		}
	}

	// Rename remote repository to new path and delete local copy.
	dir := UserPath(newOwner.Name)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", dir, err)
	}

	if err = os.Rename(RepoPath(oldOwner.Name, repo.Name), RepoPath(newOwner.Name, repo.Name)); err != nil {
		return fmt.Errorf("rename repository directory: %v", err)
	}

	// Rename remote wiki repository to new path and delete local copy.
	wikiPath := WikiPath(oldOwner.Name, repo.Name)
	if com.IsExist(wikiPath) {
		if err = os.Rename(wikiPath, WikiPath(newOwner.Name, repo.Name)); err != nil {
			return fmt.Errorf("rename repository wiki: %v", err)
		}
	}

	// If there was previously a redirect at this location, remove it.
	if err = deleteRepoRedirect(sess, newOwner.ID, repo.Name); err != nil {
		return fmt.Errorf("delete repo redirect: %v", err)
	}

	if err := NewRepoRedirect(DBContext{sess}, oldOwner.ID, repo.ID, repo.Name, repo.Name); err != nil {
		return fmt.Errorf("NewRepoRedirect: %v", err)
	}

	return sess.Commit()
}

// ChangeRepositoryName changes all corresponding setting from old repository name to new one.
func ChangeRepositoryName(doer *User, repo *Repository, newRepoName string) (err error) {
	oldRepoName := repo.Name
	newRepoName = strings.ToLower(newRepoName)
	if err = IsUsableRepoName(newRepoName); err != nil {
		return err
	}

	if err := repo.GetOwner(); err != nil {
		return err
	}

	has, err := IsRepositoryExist(repo.Owner, newRepoName)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return ErrRepoAlreadyExist{repo.Owner.Name, newRepoName}
	}

	newRepoPath := RepoPath(repo.Owner.Name, newRepoName)
	if err = os.Rename(repo.RepoPath(), newRepoPath); err != nil {
		return fmt.Errorf("rename repository directory: %v", err)
	}

	wikiPath := repo.WikiPath()
	if com.IsExist(wikiPath) {
		if err = os.Rename(wikiPath, WikiPath(repo.Owner.Name, newRepoName)); err != nil {
			return fmt.Errorf("rename repository wiki: %v", err)
		}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return fmt.Errorf("sess.Begin: %v", err)
	}

	// If there was previously a redirect at this location, remove it.
	if err = deleteRepoRedirect(sess, repo.OwnerID, newRepoName); err != nil {
		return fmt.Errorf("delete repo redirect: %v", err)
	}

	if err := NewRepoRedirect(DBContext{sess}, repo.Owner.ID, repo.ID, oldRepoName, newRepoName); err != nil {
		return err
	}

	return sess.Commit()
}

func getRepositoriesByForkID(e Engine, forkID int64) ([]*Repository, error) {
	repos := make([]*Repository, 0, 10)
	return repos, e.
		Where("fork_id=?", forkID).
		Find(&repos)
}

// GetRepositoriesByForkID returns all repositories with given fork ID.
func GetRepositoriesByForkID(forkID int64) ([]*Repository, error) {
	return getRepositoriesByForkID(x, forkID)
}

func updateRepository(e Engine, repo *Repository, visibilityChanged bool) (err error) {
	repo.LowerName = strings.ToLower(repo.Name)

	if len(repo.Description) > 255 {
		repo.Description = repo.Description[:255]
	}
	if len(repo.Website) > 255 {
		repo.Website = repo.Website[:255]
	}

	if _, err = e.ID(repo.ID).AllCols().Update(repo); err != nil {
		return fmt.Errorf("update: %v", err)
	}

	if visibilityChanged {
		if err = repo.getOwner(e); err != nil {
			return fmt.Errorf("getOwner: %v", err)
		}
		if repo.Owner.IsOrganization() {
			// Organization repository need to recalculate access table when visibility is changed.
			if err = repo.recalculateTeamAccesses(e, 0); err != nil {
				return fmt.Errorf("recalculateTeamAccesses: %v", err)
			}
		}

		// If repo has become private, we need to set its actions to private.
		if repo.IsPrivate {
			_, err = e.Where("repo_id = ?", repo.ID).Cols("is_private").Update(&Action{
				IsPrivate: true,
			})
			if err != nil {
				return err
			}
		}

		// Create/Remove git-daemon-export-ok for git-daemon...
		daemonExportFile := path.Join(repo.repoPath(e), `git-daemon-export-ok`)
		if repo.IsPrivate && com.IsExist(daemonExportFile) {
			if err = os.Remove(daemonExportFile); err != nil {
				log.Error("Failed to remove %s: %v", daemonExportFile, err)
			}
		} else if !repo.IsPrivate && !com.IsExist(daemonExportFile) {
			if f, err := os.Create(daemonExportFile); err != nil {
				log.Error("Failed to create %s: %v", daemonExportFile, err)
			} else {
				f.Close()
			}
		}

		forkRepos, err := getRepositoriesByForkID(e, repo.ID)
		if err != nil {
			return fmt.Errorf("getRepositoriesByForkID: %v", err)
		}
		for i := range forkRepos {
			forkRepos[i].IsPrivate = repo.IsPrivate
			if err = updateRepository(e, forkRepos[i], true); err != nil {
				return fmt.Errorf("updateRepository[%d]: %v", forkRepos[i].ID, err)
			}
		}

		if err = repo.updateSize(e); err != nil {
			log.Error("Failed to update size for repository: %v", err)
		}
	}

	return nil
}

// UpdateRepository updates a repository
func UpdateRepository(repo *Repository, visibilityChanged bool) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = updateRepository(sess, repo, visibilityChanged); err != nil {
		return fmt.Errorf("updateRepository: %v", err)
	}

	return sess.Commit()
}

// UpdateRepositoryUpdatedTime updates a repository's updated time
func UpdateRepositoryUpdatedTime(repoID int64, updateTime time.Time) error {
	_, err := x.Exec("UPDATE repository SET updated_unix = ? WHERE id = ?", updateTime.Unix(), repoID)
	return err
}

// UpdateRepositoryUnits updates a repository's units
func UpdateRepositoryUnits(repo *Repository, units []RepoUnit) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.Where("repo_id = ?", repo.ID).Delete(new(RepoUnit)); err != nil {
		return err
	}

	if _, err = sess.Insert(units); err != nil {
		return err
	}

	return sess.Commit()
}

// DeleteRepository deletes a repository for a user or organization.
func DeleteRepository(doer *User, uid, repoID int64) error {
	// In case is a organization.
	org, err := GetUserByID(uid)
	if err != nil {
		return err
	}
	if org.IsOrganization() {
		if err = org.GetTeams(); err != nil {
			return err
		}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	repo := &Repository{ID: repoID, OwnerID: uid}
	has, err := sess.Get(repo)
	if err != nil {
		return err
	} else if !has {
		return ErrRepoNotExist{repoID, uid, "", ""}
	}

	// Delete Deploy Keys
	deployKeys, err := listDeployKeys(sess, repo.ID)
	if err != nil {
		return fmt.Errorf("listDeployKeys: %v", err)
	}
	for _, dKey := range deployKeys {
		if err := deleteDeployKey(sess, doer, dKey.ID); err != nil {
			return fmt.Errorf("deleteDeployKeys: %v", err)
		}
	}

	if cnt, err := sess.ID(repoID).Delete(&Repository{}); err != nil {
		return err
	} else if cnt != 1 {
		return ErrRepoNotExist{repoID, uid, "", ""}
	}

	if org.IsOrganization() {
		for _, t := range org.Teams {
			if !t.hasRepository(sess, repoID) {
				continue
			} else if err = t.removeRepository(sess, repo, false); err != nil {
				return err
			}
		}
	}

	attachments := make([]*Attachment, 0, 20)
	if err = sess.Join("INNER", "`release`", "`release`.id = `attachment`.release_id").
		Where("`release`.repo_id = ?", repoID).
		Find(&attachments); err != nil {
		return err
	}
	releaseAttachments := make([]string, 0, len(attachments))
	for i := 0; i < len(attachments); i++ {
		releaseAttachments = append(releaseAttachments, attachments[i].LocalPath())
	}

	if err = deleteBeans(sess,
		&Access{RepoID: repo.ID},
		&Action{RepoID: repo.ID},
		&Watch{RepoID: repoID},
		&Star{RepoID: repoID},
		&Mirror{RepoID: repoID},
		&Milestone{RepoID: repoID},
		&Release{RepoID: repoID},
		&Collaboration{RepoID: repoID},
		&PullRequest{BaseRepoID: repoID},
		&RepoUnit{RepoID: repoID},
		&RepoRedirect{RedirectRepoID: repoID},
		&Webhook{RepoID: repoID},
		&HookTask{RepoID: repoID},
		&Notification{RepoID: repoID},
		&CommitStatus{RepoID: repoID},
		&RepoIndexerStatus{RepoID: repoID},
		&Comment{RefRepoID: repoID},
		&Task{RepoID: repoID},
	); err != nil {
		return fmt.Errorf("deleteBeans: %v", err)
	}

	deleteCond := builder.Select("id").From("issue").Where(builder.Eq{"repo_id": repoID})
	// Delete comments and attachments
	if _, err = sess.In("issue_id", deleteCond).
		Delete(&Comment{}); err != nil {
		return err
	}

	// Dependencies for issues in this repository
	if _, err = sess.In("issue_id", deleteCond).
		Delete(&IssueDependency{}); err != nil {
		return err
	}

	// Delete dependencies for issues in other repositories
	if _, err = sess.In("dependency_id", deleteCond).
		Delete(&IssueDependency{}); err != nil {
		return err
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&IssueUser{}); err != nil {
		return err
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&Reaction{}); err != nil {
		return err
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&IssueWatch{}); err != nil {
		return err
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&Stopwatch{}); err != nil {
		return err
	}

	attachments = attachments[:0]
	if err = sess.Join("INNER", "issue", "issue.id = attachment.issue_id").
		Where("issue.repo_id = ?", repoID).
		Find(&attachments); err != nil {
		return err
	}
	attachmentPaths := make([]string, 0, len(attachments))
	for j := range attachments {
		attachmentPaths = append(attachmentPaths, attachments[j].LocalPath())
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&Attachment{}); err != nil {
		return err
	}

	if _, err = sess.Delete(&Issue{RepoID: repoID}); err != nil {
		return err
	}

	if _, err = sess.Where("repo_id = ?", repoID).Delete(new(RepoUnit)); err != nil {
		return err
	}

	if repo.IsFork {
		if _, err = sess.Exec("UPDATE `repository` SET num_forks=num_forks-1 WHERE id=?", repo.ForkID); err != nil {
			return fmt.Errorf("decrease fork count: %v", err)
		}
	}

	if _, err = sess.Exec("UPDATE `user` SET num_repos=num_repos-1 WHERE id=?", uid); err != nil {
		return err
	}

	if len(repo.Topics) > 0 {
		if err = removeTopicsFromRepo(sess, repo.ID); err != nil {
			return err
		}
	}

	// FIXME: Remove repository files should be executed after transaction succeed.
	repoPath := repo.repoPath(sess)
	removeAllWithNotice(sess, "Delete repository files", repoPath)

	err = repo.deleteWiki(sess)
	if err != nil {
		return err
	}

	// Remove LFS objects
	var lfsObjects []*LFSMetaObject
	if err = sess.Where("repository_id=?", repoID).Find(&lfsObjects); err != nil {
		return err
	}

	for _, v := range lfsObjects {
		count, err := sess.Count(&LFSMetaObject{Oid: v.Oid})
		if err != nil {
			return err
		}
		if count > 1 {
			continue
		}

		oidPath := filepath.Join(setting.LFS.ContentPath, v.Oid[0:2], v.Oid[2:4], v.Oid[4:len(v.Oid)])
		removeAllWithNotice(sess, "Delete orphaned LFS file", oidPath)
	}

	if _, err := sess.Delete(&LFSMetaObject{RepositoryID: repoID}); err != nil {
		return err
	}

	if repo.NumForks > 0 {
		if _, err = sess.Exec("UPDATE `repository` SET fork_id=0,is_fork=? WHERE fork_id=?", false, repo.ID); err != nil {
			log.Error("reset 'fork_id' and 'is_fork': %v", err)
		}
	}

	if err = sess.Commit(); err != nil {
		if len(deployKeys) > 0 {
			// We need to rewrite the public keys because the commit failed
			if err2 := RewriteAllPublicKeys(); err2 != nil {
				return fmt.Errorf("Commit: %v SSH Keys: %v", err, err2)
			}
		}
		return fmt.Errorf("Commit: %v", err)
	}

	sess.Close()

	// We should always delete the files after the database transaction succeed. If
	// we delete the file but the database rollback, the repository will be borken.

	// Remove issue attachment files.
	for i := range attachmentPaths {
		removeAllWithNotice(x, "Delete issue attachment", attachmentPaths[i])
	}

	// Remove release attachment files.
	for i := range releaseAttachments {
		removeAllWithNotice(x, "Delete release attachment", releaseAttachments[i])
	}

	if len(repo.Avatar) > 0 {
		avatarPath := repo.CustomAvatarPath()
		if com.IsExist(avatarPath) {
			if err := os.Remove(avatarPath); err != nil {
				return fmt.Errorf("Failed to remove %s: %v", avatarPath, err)
			}
		}
	}

	return nil
}

// GetRepositoryByOwnerAndName returns the repository by given ownername and reponame.
func GetRepositoryByOwnerAndName(ownerName, repoName string) (*Repository, error) {
	return getRepositoryByOwnerAndName(x, ownerName, repoName)
}

func getRepositoryByOwnerAndName(e Engine, ownerName, repoName string) (*Repository, error) {
	var repo Repository
	has, err := e.Table("repository").Select("repository.*").
		Join("INNER", "`user`", "`user`.id = repository.owner_id").
		Where("repository.lower_name = ?", strings.ToLower(repoName)).
		And("`user`.lower_name = ?", strings.ToLower(ownerName)).
		Get(&repo)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRepoNotExist{0, 0, ownerName, repoName}
	}
	return &repo, nil
}

// GetRepositoryByName returns the repository by given name under user if exists.
func GetRepositoryByName(ownerID int64, name string) (*Repository, error) {
	repo := &Repository{
		OwnerID:   ownerID,
		LowerName: strings.ToLower(name),
	}
	has, err := x.Get(repo)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRepoNotExist{0, ownerID, "", name}
	}
	return repo, err
}

func getRepositoryByID(e Engine, id int64) (*Repository, error) {
	repo := new(Repository)
	has, err := e.ID(id).Get(repo)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRepoNotExist{id, 0, "", ""}
	}
	return repo, nil
}

// GetRepositoryByID returns the repository by given id if exists.
func GetRepositoryByID(id int64) (*Repository, error) {
	return getRepositoryByID(x, id)
}

// GetRepositoriesMapByIDs returns the repositories by given id slice.
func GetRepositoriesMapByIDs(ids []int64) (map[int64]*Repository, error) {
	var repos = make(map[int64]*Repository, len(ids))
	return repos, x.In("id", ids).Find(&repos)
}

// GetUserRepositories returns a list of repositories of given user.
func GetUserRepositories(userID int64, private bool, page, pageSize int, orderBy string) ([]*Repository, error) {
	if len(orderBy) == 0 {
		orderBy = "updated_unix DESC"
	}

	sess := x.
		Where("owner_id = ?", userID).
		OrderBy(orderBy)
	if !private {
		sess.And("is_private=?", false)
	}

	if page <= 0 {
		page = 1
	}
	sess.Limit(pageSize, (page-1)*pageSize)

	repos := make([]*Repository, 0, pageSize)
	return repos, sess.Find(&repos)
}

// GetUserMirrorRepositories returns a list of mirror repositories of given user.
func GetUserMirrorRepositories(userID int64) ([]*Repository, error) {
	repos := make([]*Repository, 0, 10)
	return repos, x.
		Where("owner_id = ?", userID).
		And("is_mirror = ?", true).
		Find(&repos)
}

func getRepositoryCount(e Engine, u *User) (int64, error) {
	return e.Count(&Repository{OwnerID: u.ID})
}

func getPublicRepositoryCount(e Engine, u *User) (int64, error) {
	return e.Where("is_private = ?", false).Count(&Repository{OwnerID: u.ID})
}

func getPrivateRepositoryCount(e Engine, u *User) (int64, error) {
	return e.Where("is_private = ?", true).Count(&Repository{OwnerID: u.ID})
}

// GetRepositoryCount returns the total number of repositories of user.
func GetRepositoryCount(u *User) (int64, error) {
	return getRepositoryCount(x, u)
}

// GetPublicRepositoryCount returns the total number of public repositories of user.
func GetPublicRepositoryCount(u *User) (int64, error) {
	return getPublicRepositoryCount(x, u)
}

// GetPrivateRepositoryCount returns the total number of private repositories of user.
func GetPrivateRepositoryCount(u *User) (int64, error) {
	return getPrivateRepositoryCount(x, u)
}

// DeleteRepositoryArchives deletes all repositories' archives.
func DeleteRepositoryArchives() error {
	return x.
		Where("id > 0").
		Iterate(new(Repository),
			func(idx int, bean interface{}) error {
				repo := bean.(*Repository)
				return os.RemoveAll(filepath.Join(repo.RepoPath(), "archives"))
			})
}

// DeleteOldRepositoryArchives deletes old repository archives.
func DeleteOldRepositoryArchives(ctx context.Context) {
	log.Trace("Doing: ArchiveCleanup")

	if err := x.Where("id > 0").Iterate(new(Repository), func(idx int, bean interface{}) error {
		return deleteOldRepositoryArchives(ctx, idx, bean)
	}); err != nil {
		log.Error("ArchiveClean: %v", err)
	}
}

func deleteOldRepositoryArchives(ctx context.Context, idx int, bean interface{}) error {
	repo := bean.(*Repository)
	basePath := filepath.Join(repo.RepoPath(), "archives")

	for _, ty := range []string{"zip", "targz"} {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Aborted due to shutdown:\nin delete of old repository archives %v\nat delete file %s", repo, ty)
		default:
		}

		path := filepath.Join(basePath, ty)
		file, err := os.Open(path)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Warn("Unable to open directory %s: %v", path, err)
				return err
			}

			// If the directory doesn't exist, that's okay.
			continue
		}

		files, err := file.Readdir(0)
		file.Close()
		if err != nil {
			log.Warn("Unable to read directory %s: %v", path, err)
			return err
		}

		minimumOldestTime := time.Now().Add(-setting.Cron.ArchiveCleanup.OlderThan)
		for _, info := range files {
			if info.ModTime().Before(minimumOldestTime) && !info.IsDir() {
				select {
				case <-ctx.Done():
					return fmt.Errorf("Aborted due to shutdown:\nin delete of old repository archives %v\nat delete file %s - %s", repo, ty, info.Name())
				default:
				}
				toDelete := filepath.Join(path, info.Name())
				// This is a best-effort purge, so we do not check error codes to confirm removal.
				if err = os.Remove(toDelete); err != nil {
					log.Trace("Unable to delete %s, but proceeding: %v", toDelete, err)
				}
			}
		}
	}

	return nil
}

func gatherMissingRepoRecords() ([]*Repository, error) {
	repos := make([]*Repository, 0, 10)
	if err := x.
		Where("id > 0").
		Iterate(new(Repository),
			func(idx int, bean interface{}) error {
				repo := bean.(*Repository)
				if !com.IsDir(repo.RepoPath()) {
					repos = append(repos, repo)
				}
				return nil
			}); err != nil {
		if err2 := CreateRepositoryNotice(fmt.Sprintf("gatherMissingRepoRecords: %v", err)); err2 != nil {
			return nil, fmt.Errorf("CreateRepositoryNotice: %v", err)
		}
	}
	return repos, nil
}

// DeleteMissingRepositories deletes all repository records that lost Git files.
func DeleteMissingRepositories(doer *User) error {
	repos, err := gatherMissingRepoRecords()
	if err != nil {
		return fmt.Errorf("gatherMissingRepoRecords: %v", err)
	}

	if len(repos) == 0 {
		return nil
	}

	for _, repo := range repos {
		log.Trace("Deleting %d/%d...", repo.OwnerID, repo.ID)
		if err := DeleteRepository(doer, repo.OwnerID, repo.ID); err != nil {
			if err2 := CreateRepositoryNotice(fmt.Sprintf("DeleteRepository [%d]: %v", repo.ID, err)); err2 != nil {
				return fmt.Errorf("CreateRepositoryNotice: %v", err)
			}
		}
	}
	return nil
}

// ReinitMissingRepositories reinitializes all repository records that lost Git files.
func ReinitMissingRepositories() error {
	repos, err := gatherMissingRepoRecords()
	if err != nil {
		return fmt.Errorf("gatherMissingRepoRecords: %v", err)
	}

	if len(repos) == 0 {
		return nil
	}

	for _, repo := range repos {
		log.Trace("Initializing %d/%d...", repo.OwnerID, repo.ID)
		if err := git.InitRepository(repo.RepoPath(), true); err != nil {
			if err2 := CreateRepositoryNotice(fmt.Sprintf("InitRepository [%d]: %v", repo.ID, err)); err2 != nil {
				return fmt.Errorf("CreateRepositoryNotice: %v", err)
			}
		}
	}
	return nil
}

// SyncRepositoryHooks rewrites all repositories' pre-receive, update and post-receive hooks
// to make sure the binary and custom conf path are up-to-date.
func SyncRepositoryHooks() error {
	return x.Cols("owner_id", "name").Where("id > 0").Iterate(new(Repository),
		func(idx int, bean interface{}) error {
			if err := createDelegateHooks(bean.(*Repository).RepoPath()); err != nil {
				return fmt.Errorf("SyncRepositoryHook: %v", err)
			}
			if bean.(*Repository).HasWiki() {
				if err := createDelegateHooks(bean.(*Repository).WikiPath()); err != nil {
					return fmt.Errorf("SyncRepositoryHook: %v", err)
				}
			}
			return nil
		})
}

// GitFsck calls 'git fsck' to check repository health.
func GitFsck(ctx context.Context) {
	log.Trace("Doing: GitFsck")
	if err := x.
		Where("id>0 AND is_fsck_enabled=?", true).BufferSize(setting.Database.IterateBufferSize).
		Iterate(new(Repository),
			func(idx int, bean interface{}) error {
				select {
				case <-ctx.Done():
					return fmt.Errorf("Aborted due to shutdown")
				default:
				}
				repo := bean.(*Repository)
				repoPath := repo.RepoPath()
				log.Trace("Running health check on repository %s", repoPath)
				if err := git.Fsck(repoPath, setting.Cron.RepoHealthCheck.Timeout, setting.Cron.RepoHealthCheck.Args...); err != nil {
					desc := fmt.Sprintf("Failed to health check repository (%s): %v", repoPath, err)
					log.Warn(desc)
					if err = CreateRepositoryNotice(desc); err != nil {
						log.Error("CreateRepositoryNotice: %v", err)
					}
				}
				return nil
			}); err != nil {
		log.Error("GitFsck: %v", err)
	}
	log.Trace("Finished: GitFsck")
}

// GitGcRepos calls 'git gc' to remove unnecessary files and optimize the local repository
func GitGcRepos() error {
	args := append([]string{"gc"}, setting.Git.GCArgs...)
	return x.
		Where("id > 0").BufferSize(setting.Database.IterateBufferSize).
		Iterate(new(Repository),
			func(idx int, bean interface{}) error {
				repo := bean.(*Repository)
				if err := repo.GetOwner(); err != nil {
					return err
				}
				if stdout, err := git.NewCommand(args...).
					SetDescription(fmt.Sprintf("Repository Garbage Collection: %s", repo.FullName())).
					RunInDirTimeout(
						time.Duration(setting.Git.Timeout.GC)*time.Second,
						RepoPath(repo.Owner.Name, repo.Name)); err != nil {
					log.Error("Repository garbage collection failed for %v. Stdout: %s\nError: %v", repo, stdout, err)
					return fmt.Errorf("Repository garbage collection failed: Error: %v", err)
				}
				return nil
			})
}

type repoChecker struct {
	querySQL, correctSQL string
	desc                 string
}

func repoStatsCheck(ctx context.Context, checker *repoChecker) {
	results, err := x.Query(checker.querySQL)
	if err != nil {
		log.Error("Select %s: %v", checker.desc, err)
		return
	}
	for _, result := range results {
		select {
		case <-ctx.Done():
			log.Warn("CheckRepoStats: Aborting due to shutdown")
			return
		default:
		}
		id := com.StrTo(result["id"]).MustInt64()
		log.Trace("Updating %s: %d", checker.desc, id)
		_, err = x.Exec(checker.correctSQL, id, id)
		if err != nil {
			log.Error("Update %s[%d]: %v", checker.desc, id, err)
		}
	}
}

// CheckRepoStats checks the repository stats
func CheckRepoStats(ctx context.Context) {
	log.Trace("Doing: CheckRepoStats")

	checkers := []*repoChecker{
		// Repository.NumWatches
		{
			"SELECT repo.id FROM `repository` repo WHERE repo.num_watches!=(SELECT COUNT(*) FROM `watch` WHERE repo_id=repo.id AND mode<>2)",
			"UPDATE `repository` SET num_watches=(SELECT COUNT(*) FROM `watch` WHERE repo_id=? AND mode<>2) WHERE id=?",
			"repository count 'num_watches'",
		},
		// Repository.NumStars
		{
			"SELECT repo.id FROM `repository` repo WHERE repo.num_stars!=(SELECT COUNT(*) FROM `star` WHERE repo_id=repo.id)",
			"UPDATE `repository` SET num_stars=(SELECT COUNT(*) FROM `star` WHERE repo_id=?) WHERE id=?",
			"repository count 'num_stars'",
		},
		// Label.NumIssues
		{
			"SELECT label.id FROM `label` WHERE label.num_issues!=(SELECT COUNT(*) FROM `issue_label` WHERE label_id=label.id)",
			"UPDATE `label` SET num_issues=(SELECT COUNT(*) FROM `issue_label` WHERE label_id=?) WHERE id=?",
			"label count 'num_issues'",
		},
		// User.NumRepos
		{
			"SELECT `user`.id FROM `user` WHERE `user`.num_repos!=(SELECT COUNT(*) FROM `repository` WHERE owner_id=`user`.id)",
			"UPDATE `user` SET num_repos=(SELECT COUNT(*) FROM `repository` WHERE owner_id=?) WHERE id=?",
			"user count 'num_repos'",
		},
		// Issue.NumComments
		{
			"SELECT `issue`.id FROM `issue` WHERE `issue`.num_comments!=(SELECT COUNT(*) FROM `comment` WHERE issue_id=`issue`.id AND type=0)",
			"UPDATE `issue` SET num_comments=(SELECT COUNT(*) FROM `comment` WHERE issue_id=? AND type=0) WHERE id=?",
			"issue count 'num_comments'",
		},
	}
	for i := range checkers {
		select {
		case <-ctx.Done():
			log.Warn("CheckRepoStats: Aborting due to shutdown")
			return
		default:
			repoStatsCheck(ctx, checkers[i])
		}
	}

	// ***** START: Repository.NumClosedIssues *****
	desc := "repository count 'num_closed_issues'"
	results, err := x.Query("SELECT repo.id FROM `repository` repo WHERE repo.num_closed_issues!=(SELECT COUNT(*) FROM `issue` WHERE repo_id=repo.id AND is_closed=? AND is_pull=?)", true, false)
	if err != nil {
		log.Error("Select %s: %v", desc, err)
	} else {
		for _, result := range results {
			select {
			case <-ctx.Done():
				log.Warn("CheckRepoStats: Aborting due to shutdown")
				return
			default:
			}
			id := com.StrTo(result["id"]).MustInt64()
			log.Trace("Updating %s: %d", desc, id)
			_, err = x.Exec("UPDATE `repository` SET num_closed_issues=(SELECT COUNT(*) FROM `issue` WHERE repo_id=? AND is_closed=? AND is_pull=?) WHERE id=?", id, true, false, id)
			if err != nil {
				log.Error("Update %s[%d]: %v", desc, id, err)
			}
		}
	}
	// ***** END: Repository.NumClosedIssues *****

	// ***** START: Repository.NumClosedPulls *****
	desc = "repository count 'num_closed_pulls'"
	results, err = x.Query("SELECT repo.id FROM `repository` repo WHERE repo.num_closed_pulls!=(SELECT COUNT(*) FROM `issue` WHERE repo_id=repo.id AND is_closed=? AND is_pull=?)", true, true)
	if err != nil {
		log.Error("Select %s: %v", desc, err)
	} else {
		for _, result := range results {
			select {
			case <-ctx.Done():
				log.Warn("CheckRepoStats: Aborting due to shutdown")
				return
			default:
			}
			id := com.StrTo(result["id"]).MustInt64()
			log.Trace("Updating %s: %d", desc, id)
			_, err = x.Exec("UPDATE `repository` SET num_closed_pulls=(SELECT COUNT(*) FROM `issue` WHERE repo_id=? AND is_closed=? AND is_pull=?) WHERE id=?", id, true, true, id)
			if err != nil {
				log.Error("Update %s[%d]: %v", desc, id, err)
			}
		}
	}
	// ***** END: Repository.NumClosedPulls *****

	// FIXME: use checker when stop supporting old fork repo format.
	// ***** START: Repository.NumForks *****
	results, err = x.Query("SELECT repo.id FROM `repository` repo WHERE repo.num_forks!=(SELECT COUNT(*) FROM `repository` WHERE fork_id=repo.id)")
	if err != nil {
		log.Error("Select repository count 'num_forks': %v", err)
	} else {
		for _, result := range results {
			select {
			case <-ctx.Done():
				log.Warn("CheckRepoStats: Aborting due to shutdown")
				return
			default:
			}
			id := com.StrTo(result["id"]).MustInt64()
			log.Trace("Updating repository count 'num_forks': %d", id)

			repo, err := GetRepositoryByID(id)
			if err != nil {
				log.Error("GetRepositoryByID[%d]: %v", id, err)
				continue
			}

			rawResult, err := x.Query("SELECT COUNT(*) FROM `repository` WHERE fork_id=?", repo.ID)
			if err != nil {
				log.Error("Select count of forks[%d]: %v", repo.ID, err)
				continue
			}
			repo.NumForks = int(parseCountResult(rawResult))

			if err = UpdateRepository(repo, false); err != nil {
				log.Error("UpdateRepository[%d]: %v", id, err)
				continue
			}
		}
	}
	// ***** END: Repository.NumForks *****
}

// SetArchiveRepoState sets if a repo is archived
func (repo *Repository) SetArchiveRepoState(isArchived bool) (err error) {
	repo.IsArchived = isArchived
	_, err = x.Where("id = ?", repo.ID).Cols("is_archived").Update(repo)
	return
}

// ___________           __
// \_   _____/__________|  | __
//  |    __)/  _ \_  __ \  |/ /
//  |     \(  <_> )  | \/    <
//  \___  / \____/|__|  |__|_ \
//      \/                   \/

// HasForkedRepo checks if given user has already forked a repository with given ID.
func HasForkedRepo(ownerID, repoID int64) (*Repository, bool) {
	repo := new(Repository)
	has, _ := x.
		Where("owner_id=? AND fork_id=?", ownerID, repoID).
		Get(repo)
	return repo, has
}

// CopyLFS copies LFS data from one repo to another
func CopyLFS(newRepo, oldRepo *Repository) error {
	return copyLFS(x, newRepo, oldRepo)
}

func copyLFS(e Engine, newRepo, oldRepo *Repository) error {
	var lfsObjects []*LFSMetaObject
	if err := e.Where("repository_id=?", oldRepo.ID).Find(&lfsObjects); err != nil {
		return err
	}

	for _, v := range lfsObjects {
		v.ID = 0
		v.RepositoryID = newRepo.ID
		if _, err := e.Insert(v); err != nil {
			return err
		}
	}

	return nil
}

// ForkRepository forks a repository
func ForkRepository(doer, owner *User, oldRepo *Repository, name, desc string) (_ *Repository, err error) {
	forkedRepo, err := oldRepo.GetUserFork(owner.ID)
	if err != nil {
		return nil, err
	}
	if forkedRepo != nil {
		return nil, ErrForkAlreadyExist{
			Uname:    owner.Name,
			RepoName: oldRepo.FullName(),
			ForkName: forkedRepo.FullName(),
		}
	}

	repo := &Repository{
		OwnerID:       owner.ID,
		Owner:         owner,
		Name:          name,
		LowerName:     strings.ToLower(name),
		Description:   desc,
		DefaultBranch: oldRepo.DefaultBranch,
		IsPrivate:     oldRepo.IsPrivate,
		IsEmpty:       oldRepo.IsEmpty,
		IsFork:        true,
		ForkID:        oldRepo.ID,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	if err = createRepository(sess, doer, owner, repo); err != nil {
		return nil, err
	}

	if _, err = sess.Exec("UPDATE `repository` SET num_forks=num_forks+1 WHERE id=?", oldRepo.ID); err != nil {
		return nil, err
	}

	repoPath := RepoPath(owner.Name, repo.Name)
	if stdout, err := git.NewCommand(
		"clone", "--bare", oldRepo.repoPath(sess), repoPath).
		SetDescription(fmt.Sprintf("ForkRepository(git clone): %s to %s", oldRepo.FullName(), repo.FullName())).
		RunInDirTimeout(10*time.Minute, ""); err != nil {
		log.Error("Fork Repository (git clone) Failed for %v (from %v):\nStdout: %s\nError: %v", repo, oldRepo, stdout, err)
		return nil, fmt.Errorf("git clone: %v", err)
	}

	if stdout, err := git.NewCommand("update-server-info").
		SetDescription(fmt.Sprintf("ForkRepository(git update-server-info): %s", repo.FullName())).
		RunInDir(repoPath); err != nil {
		log.Error("Fork Repository (git update-server-info) failed for %v:\nStdout: %s\nError: %v", repo, stdout, err)
		return nil, fmt.Errorf("git update-server-info: %v", err)
	}

	if err = createDelegateHooks(repoPath); err != nil {
		return nil, fmt.Errorf("createDelegateHooks: %v", err)
	}

	//Commit repo to get Fork ID
	err = sess.Commit()
	if err != nil {
		return nil, err
	}

	if err = repo.UpdateSize(); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}

	return repo, CopyLFS(repo, oldRepo)
}

// GetForks returns all the forks of the repository
func (repo *Repository) GetForks() ([]*Repository, error) {
	forks := make([]*Repository, 0, repo.NumForks)
	return forks, x.Find(&forks, &Repository{ForkID: repo.ID})
}

// GetUserFork return user forked repository from this repository, if not forked return nil
func (repo *Repository) GetUserFork(userID int64) (*Repository, error) {
	var forkedRepo Repository
	has, err := x.Where("fork_id = ?", repo.ID).And("owner_id = ?", userID).Get(&forkedRepo)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return &forkedRepo, nil
}

// CustomAvatarPath returns repository custom avatar file path.
func (repo *Repository) CustomAvatarPath() string {
	// Avatar empty by default
	if len(repo.Avatar) == 0 {
		return ""
	}
	return filepath.Join(setting.RepositoryAvatarUploadPath, repo.Avatar)
}

// generateRandomAvatar generates a random avatar for repository.
func (repo *Repository) generateRandomAvatar(e Engine) error {
	idToString := fmt.Sprintf("%d", repo.ID)

	seed := idToString
	img, err := avatar.RandomImage([]byte(seed))
	if err != nil {
		return fmt.Errorf("RandomImage: %v", err)
	}

	repo.Avatar = idToString
	if err = os.MkdirAll(filepath.Dir(repo.CustomAvatarPath()), os.ModePerm); err != nil {
		return fmt.Errorf("MkdirAll: %v", err)
	}
	fw, err := os.Create(repo.CustomAvatarPath())
	if err != nil {
		return fmt.Errorf("Create: %v", err)
	}
	defer fw.Close()

	if err = png.Encode(fw, img); err != nil {
		return fmt.Errorf("Encode: %v", err)
	}
	log.Info("New random avatar created for repository: %d", repo.ID)

	if _, err := e.ID(repo.ID).Cols("avatar").NoAutoTime().Update(repo); err != nil {
		return err
	}

	return nil
}

// RemoveRandomAvatars removes the randomly generated avatars that were created for repositories
func RemoveRandomAvatars() error {
	return x.
		Where("id > 0").BufferSize(setting.Database.IterateBufferSize).
		Iterate(new(Repository),
			func(idx int, bean interface{}) error {
				repository := bean.(*Repository)
				stringifiedID := strconv.FormatInt(repository.ID, 10)
				if repository.Avatar == stringifiedID {
					return repository.DeleteAvatar()
				}
				return nil
			})
}

// RelAvatarLink returns a relative link to the repository's avatar.
func (repo *Repository) RelAvatarLink() string {
	return repo.relAvatarLink(x)
}

func (repo *Repository) relAvatarLink(e Engine) string {
	// If no avatar - path is empty
	avatarPath := repo.CustomAvatarPath()
	if len(avatarPath) == 0 || !com.IsFile(avatarPath) {
		switch mode := setting.RepositoryAvatarFallback; mode {
		case "image":
			return setting.RepositoryAvatarFallbackImage
		case "random":
			if err := repo.generateRandomAvatar(e); err != nil {
				log.Error("generateRandomAvatar: %v", err)
			}
		default:
			// default behaviour: do not display avatar
			return ""
		}
	}
	return setting.AppSubURL + "/repo-avatars/" + repo.Avatar
}

// avatarLink returns user avatar absolute link.
func (repo *Repository) avatarLink(e Engine) string {
	link := repo.relAvatarLink(e)
	// link may be empty!
	if len(link) > 0 {
		if link[0] == '/' && link[1] != '/' {
			return setting.AppURL + strings.TrimPrefix(link, setting.AppSubURL)[1:]
		}
	}
	return link
}

// UploadAvatar saves custom avatar for repository.
// FIXME: split uploads to different subdirs in case we have massive number of repos.
func (repo *Repository) UploadAvatar(data []byte) error {
	m, err := avatar.Prepare(data)
	if err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	oldAvatarPath := repo.CustomAvatarPath()

	// Users can upload the same image to other repo - prefix it with ID
	// Then repo will be removed - only it avatar file will be removed
	repo.Avatar = fmt.Sprintf("%d-%x", repo.ID, md5.Sum(data))
	if _, err := sess.ID(repo.ID).Cols("avatar").Update(repo); err != nil {
		return fmt.Errorf("UploadAvatar: Update repository avatar: %v", err)
	}

	if err := os.MkdirAll(setting.RepositoryAvatarUploadPath, os.ModePerm); err != nil {
		return fmt.Errorf("UploadAvatar: Failed to create dir %s: %v", setting.RepositoryAvatarUploadPath, err)
	}

	fw, err := os.Create(repo.CustomAvatarPath())
	if err != nil {
		return fmt.Errorf("UploadAvatar: Create file: %v", err)
	}
	defer fw.Close()

	if err = png.Encode(fw, *m); err != nil {
		return fmt.Errorf("UploadAvatar: Encode png: %v", err)
	}

	if len(oldAvatarPath) > 0 && oldAvatarPath != repo.CustomAvatarPath() {
		if err := os.Remove(oldAvatarPath); err != nil {
			return fmt.Errorf("UploadAvatar: Failed to remove old repo avatar %s: %v", oldAvatarPath, err)
		}
	}

	return sess.Commit()
}

// DeleteAvatar deletes the repos's custom avatar.
func (repo *Repository) DeleteAvatar() error {

	// Avatar not exists
	if len(repo.Avatar) == 0 {
		return nil
	}

	avatarPath := repo.CustomAvatarPath()
	log.Trace("DeleteAvatar[%d]: %s", repo.ID, avatarPath)

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	repo.Avatar = ""
	if _, err := sess.ID(repo.ID).Cols("avatar").Update(repo); err != nil {
		return fmt.Errorf("DeleteAvatar: Update repository avatar: %v", err)
	}

	if _, err := os.Stat(avatarPath); err == nil {
		if err := os.Remove(avatarPath); err != nil {
			return fmt.Errorf("DeleteAvatar: Failed to remove %s: %v", avatarPath, err)
		}
	} else {
		// // Schrodinger: file may or may not exist. See err for details.
		log.Trace("DeleteAvatar[%d]: %v", err)
	}
	return sess.Commit()
}

// GetOriginalURLHostname returns the hostname of a URL or the URL
func (repo *Repository) GetOriginalURLHostname() string {
	u, err := url.Parse(repo.OriginalURL)
	if err != nil {
		return repo.OriginalURL
	}

	return u.Host
}

// GetTreePathLock returns LSF lock for the treePath
func (repo *Repository) GetTreePathLock(treePath string) (*LFSLock, error) {
	if setting.LFS.StartServer {
		locks, err := GetLFSLockByRepoID(repo.ID, 0, 0)
		if err != nil {
			return nil, err
		}
		for _, lock := range locks {
			if lock.Path == treePath {
				return lock, nil
			}
		}
	}
	return nil, nil
}

func updateRepositoryCols(e Engine, repo *Repository, cols ...string) error {
	_, err := e.ID(repo.ID).Cols(cols...).Update(repo)
	return err
}

// UpdateRepositoryCols updates repository's columns
func UpdateRepositoryCols(repo *Repository, cols ...string) error {
	return updateRepositoryCols(x, repo, cols...)
}
