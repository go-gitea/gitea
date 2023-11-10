// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"html/template"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrUserDoesNotHaveAccessToRepo represents an error where the user doesn't has access to a given repo.
type ErrUserDoesNotHaveAccessToRepo struct {
	UserID   int64
	RepoName string
}

// IsErrUserDoesNotHaveAccessToRepo checks if an error is a ErrRepoFileAlreadyExists.
func IsErrUserDoesNotHaveAccessToRepo(err error) bool {
	_, ok := err.(ErrUserDoesNotHaveAccessToRepo)
	return ok
}

func (err ErrUserDoesNotHaveAccessToRepo) Error() string {
	return fmt.Sprintf("user doesn't have access to repo [user_id: %d, repo_name: %s]", err.UserID, err.RepoName)
}

func (err ErrUserDoesNotHaveAccessToRepo) Unwrap() error {
	return util.ErrPermissionDenied
}

type ErrRepoIsArchived struct {
	Repo *Repository
}

func (err ErrRepoIsArchived) Error() string {
	return fmt.Sprintf("%s is archived", err.Repo.LogString())
}

var (
	reservedRepoNames    = []string{".", "..", "-"}
	reservedRepoPatterns = []string{"*.git", "*.wiki", "*.rss", "*.atom"}
)

// IsUsableRepoName returns true when repository is usable
func IsUsableRepoName(name string) error {
	if db.AlphaDashDotPattern.MatchString(name) {
		// Note: usually this error is normally caught up earlier in the UI
		return db.ErrNameCharsNotAllowed{Name: name}
	}
	return db.IsUsableName(reservedRepoNames, reservedRepoPatterns, name)
}

// TrustModelType defines the types of trust model for this repository
type TrustModelType int

// kinds of TrustModel
const (
	DefaultTrustModel TrustModelType = iota // default trust model
	CommitterTrustModel
	CollaboratorTrustModel
	CollaboratorCommitterTrustModel
)

// String converts a TrustModelType to a string
func (t TrustModelType) String() string {
	switch t {
	case DefaultTrustModel:
		return "default"
	case CommitterTrustModel:
		return "committer"
	case CollaboratorTrustModel:
		return "collaborator"
	case CollaboratorCommitterTrustModel:
		return "collaboratorcommitter"
	}
	return "default"
}

// ToTrustModel converts a string to a TrustModelType
func ToTrustModel(model string) TrustModelType {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "default":
		return DefaultTrustModel
	case "collaborator":
		return CollaboratorTrustModel
	case "committer":
		return CommitterTrustModel
	case "collaboratorcommitter":
		return CollaboratorCommitterTrustModel
	}
	return DefaultTrustModel
}

// RepositoryStatus defines the status of repository
type RepositoryStatus int

// all kinds of RepositoryStatus
const (
	RepositoryReady           RepositoryStatus = iota // a normal repository
	RepositoryBeingMigrated                           // repository is migrating
	RepositoryPendingTransfer                         // repository pending in ownership transfer state
	RepositoryBroken                                  // repository is in a permanently broken state
)

// Repository represents a git repository.
type Repository struct {
	ID                  int64 `xorm:"pk autoincr"`
	OwnerID             int64 `xorm:"UNIQUE(s) index"`
	OwnerName           string
	Owner               *user_model.User   `xorm:"-"`
	LowerName           string             `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name                string             `xorm:"INDEX NOT NULL"`
	Description         string             `xorm:"TEXT"`
	Website             string             `xorm:"VARCHAR(2048)"`
	OriginalServiceType api.GitServiceType `xorm:"index"`
	OriginalURL         string             `xorm:"VARCHAR(2048)"`
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
	NumProjects         int `xorm:"NOT NULL DEFAULT 0"`
	NumClosedProjects   int `xorm:"NOT NULL DEFAULT 0"`
	NumOpenProjects     int `xorm:"-"`
	NumActionRuns       int `xorm:"NOT NULL DEFAULT 0"`
	NumClosedActionRuns int `xorm:"NOT NULL DEFAULT 0"`
	NumOpenActionRuns   int `xorm:"-"`

	IsPrivate  bool `xorm:"INDEX"`
	IsEmpty    bool `xorm:"INDEX"`
	IsArchived bool `xorm:"INDEX"`
	IsMirror   bool `xorm:"INDEX"`

	Status RepositoryStatus `xorm:"NOT NULL DEFAULT 0"`

	RenderingMetas         map[string]string `xorm:"-"`
	DocumentRenderingMetas map[string]string `xorm:"-"`
	Units                  []*RepoUnit       `xorm:"-"`
	PrimaryLanguage        *LanguageStat     `xorm:"-"`

	IsFork                          bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	ForkID                          int64              `xorm:"INDEX"`
	BaseRepo                        *Repository        `xorm:"-"`
	IsTemplate                      bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	TemplateID                      int64              `xorm:"INDEX"`
	Size                            int64              `xorm:"NOT NULL DEFAULT 0"`
	GitSize                         int64              `xorm:"NOT NULL DEFAULT 0"`
	LFSSize                         int64              `xorm:"NOT NULL DEFAULT 0"`
	CodeIndexerStatus               *RepoIndexerStatus `xorm:"-"`
	StatsIndexerStatus              *RepoIndexerStatus `xorm:"-"`
	IsFsckEnabled                   bool               `xorm:"NOT NULL DEFAULT true"`
	CloseIssuesViaCommitInAnyBranch bool               `xorm:"NOT NULL DEFAULT false"`
	Topics                          []string           `xorm:"TEXT JSON"`

	TrustModel TrustModelType

	// Avatar: ID(10-20)-md5(32) - must fit into 64 symbols
	Avatar string `xorm:"VARCHAR(64)"`

	CreatedUnix  timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix  timeutil.TimeStamp `xorm:"INDEX updated"`
	ArchivedUnix timeutil.TimeStamp `xorm:"DEFAULT 0"`
}

func init() {
	db.RegisterModel(new(Repository))
}

// SanitizedOriginalURL returns a sanitized OriginalURL
func (repo *Repository) SanitizedOriginalURL() string {
	if repo.OriginalURL == "" {
		return ""
	}
	u, _ := util.SanitizeURL(repo.OriginalURL)
	return u
}

// text representations to be returned in SizeDetail.Name
const (
	SizeDetailNameGit = "git"
	SizeDetailNameLFS = "lfs"
)

type SizeDetail struct {
	Name string
	Size int64
}

// SizeDetails forms a struct with various size details about repository
func (repo *Repository) SizeDetails() []SizeDetail {
	sizeDetails := []SizeDetail{
		{
			Name: SizeDetailNameGit,
			Size: repo.GitSize,
		},
		{
			Name: SizeDetailNameLFS,
			Size: repo.LFSSize,
		},
	}
	return sizeDetails
}

// SizeDetailsString returns a concatenation of all repository size details as a string
func (repo *Repository) SizeDetailsString() string {
	var str strings.Builder
	sizeDetails := repo.SizeDetails()
	for _, detail := range sizeDetails {
		str.WriteString(fmt.Sprintf("%s: %s, ", detail.Name, base.FileSize(detail.Size)))
	}
	return strings.TrimSuffix(str.String(), ", ")
}

func (repo *Repository) LogString() string {
	if repo == nil {
		return "<Repository nil>"
	}
	return fmt.Sprintf("<Repository %d:%s/%s>", repo.ID, repo.OwnerName, repo.Name)
}

// IsBeingMigrated indicates that repository is being migrated
func (repo *Repository) IsBeingMigrated() bool {
	return repo.Status == RepositoryBeingMigrated
}

// IsBeingCreated indicates that repository is being migrated or forked
func (repo *Repository) IsBeingCreated() bool {
	return repo.IsBeingMigrated()
}

// IsBroken indicates that repository is broken
func (repo *Repository) IsBroken() bool {
	return repo.Status == RepositoryBroken
}

// MarkAsBrokenEmpty marks the repo as broken and empty
func (repo *Repository) MarkAsBrokenEmpty() {
	repo.Status = RepositoryBroken
	repo.IsEmpty = true
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (repo *Repository) AfterLoad() {
	repo.NumOpenIssues = repo.NumIssues - repo.NumClosedIssues
	repo.NumOpenPulls = repo.NumPulls - repo.NumClosedPulls
	repo.NumOpenMilestones = repo.NumMilestones - repo.NumClosedMilestones
	repo.NumOpenProjects = repo.NumProjects - repo.NumClosedProjects
	repo.NumOpenActionRuns = repo.NumActionRuns - repo.NumClosedActionRuns
}

// LoadAttributes loads attributes of the repository.
func (repo *Repository) LoadAttributes(ctx context.Context) error {
	// Load owner
	if err := repo.LoadOwner(ctx); err != nil {
		return fmt.Errorf("load owner: %w", err)
	}

	// Load primary language
	stats := make(LanguageStatList, 0, 1)
	if err := db.GetEngine(ctx).
		Where("`repo_id` = ? AND `is_primary` = ? AND `language` != ?", repo.ID, true, "other").
		Find(&stats); err != nil {
		return fmt.Errorf("find primary languages: %w", err)
	}
	stats.LoadAttributes()
	for _, st := range stats {
		if st.RepoID == repo.ID {
			repo.PrimaryLanguage = st
			break
		}
	}
	return nil
}

// FullName returns the repository full name
func (repo *Repository) FullName() string {
	return repo.OwnerName + "/" + repo.Name
}

// HTMLURL returns the repository HTML URL
func (repo *Repository) HTMLURL() string {
	return setting.AppURL + url.PathEscape(repo.OwnerName) + "/" + url.PathEscape(repo.Name)
}

// CommitLink make link to by commit full ID
// note: won't check whether it's an right id
func (repo *Repository) CommitLink(commitID string) (result string) {
	if commitID == "" || commitID == "0000000000000000000000000000000000000000" {
		result = ""
	} else {
		result = repo.Link() + "/commit/" + url.PathEscape(commitID)
	}
	return result
}

// APIURL returns the repository API URL
func (repo *Repository) APIURL() string {
	return setting.AppURL + "api/v1/repos/" + url.PathEscape(repo.OwnerName) + "/" + url.PathEscape(repo.Name)
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

// LoadUnits loads repo units into repo.Units
func (repo *Repository) LoadUnits(ctx context.Context) (err error) {
	if repo.Units != nil {
		return nil
	}

	repo.Units, err = getUnitsByRepoID(ctx, repo.ID)
	if log.IsTrace() {
		unitTypeStrings := make([]string, len(repo.Units))
		for i, unit := range repo.Units {
			unitTypeStrings[i] = unit.Type.String()
		}
		log.Trace("repo.Units, ID=%d, Types: [%s]", repo.ID, strings.Join(unitTypeStrings, ", "))
	}

	return err
}

// UnitEnabled if this repository has the given unit enabled
func (repo *Repository) UnitEnabled(ctx context.Context, tp unit.Type) bool {
	if err := repo.LoadUnits(ctx); err != nil {
		log.Warn("Error loading repository (ID: %d) units: %s", repo.ID, err.Error())
	}
	for _, unit := range repo.Units {
		if unit.Type == tp {
			return true
		}
	}
	return false
}

// MustGetUnit always returns a RepoUnit object
func (repo *Repository) MustGetUnit(ctx context.Context, tp unit.Type) *RepoUnit {
	ru, err := repo.GetUnit(ctx, tp)
	if err == nil {
		return ru
	}

	if tp == unit.TypeExternalWiki {
		return &RepoUnit{
			Type:   tp,
			Config: new(ExternalWikiConfig),
		}
	} else if tp == unit.TypeExternalTracker {
		return &RepoUnit{
			Type:   tp,
			Config: new(ExternalTrackerConfig),
		}
	} else if tp == unit.TypePullRequests {
		return &RepoUnit{
			Type:   tp,
			Config: new(PullRequestsConfig),
		}
	} else if tp == unit.TypeIssues {
		return &RepoUnit{
			Type:   tp,
			Config: new(IssuesConfig),
		}
	} else if tp == unit.TypeActions {
		return &RepoUnit{
			Type:   tp,
			Config: new(ActionsConfig),
		}
	}

	return &RepoUnit{
		Type:   tp,
		Config: new(UnitConfig),
	}
}

// GetUnit returns a RepoUnit object
func (repo *Repository) GetUnit(ctx context.Context, tp unit.Type) (*RepoUnit, error) {
	if err := repo.LoadUnits(ctx); err != nil {
		return nil, err
	}
	for _, unit := range repo.Units {
		if unit.Type == tp {
			return unit, nil
		}
	}
	return nil, ErrUnitTypeNotExist{tp}
}

// LoadOwner loads owner user
func (repo *Repository) LoadOwner(ctx context.Context) (err error) {
	if repo.Owner != nil {
		return nil
	}

	repo.Owner, err = user_model.GetUserByID(ctx, repo.OwnerID)
	return err
}

// MustOwner always returns a valid *user_model.User object to avoid
// conceptually impossible error handling.
// It creates a fake object that contains error details
// when error occurs.
func (repo *Repository) MustOwner(ctx context.Context) *user_model.User {
	if err := repo.LoadOwner(ctx); err != nil {
		return &user_model.User{
			Name:     "error",
			FullName: err.Error(),
		}
	}

	return repo.Owner
}

// ComposeMetas composes a map of metas for properly rendering issue links and external issue trackers.
func (repo *Repository) ComposeMetas(ctx context.Context) map[string]string {
	if len(repo.RenderingMetas) == 0 {
		metas := map[string]string{
			"user":     repo.OwnerName,
			"repo":     repo.Name,
			"repoPath": repo.RepoPath(),
			"mode":     "comment",
		}

		unit, err := repo.GetUnit(ctx, unit.TypeExternalTracker)
		if err == nil {
			metas["format"] = unit.ExternalTrackerConfig().ExternalTrackerFormat
			switch unit.ExternalTrackerConfig().ExternalTrackerStyle {
			case markup.IssueNameStyleAlphanumeric:
				metas["style"] = markup.IssueNameStyleAlphanumeric
			case markup.IssueNameStyleRegexp:
				metas["style"] = markup.IssueNameStyleRegexp
				metas["regexp"] = unit.ExternalTrackerConfig().ExternalTrackerRegexpPattern
			default:
				metas["style"] = markup.IssueNameStyleNumeric
			}
		}

		repo.MustOwner(ctx)
		if repo.Owner.IsOrganization() {
			teams := make([]string, 0, 5)
			_ = db.GetEngine(ctx).Table("team_repo").
				Join("INNER", "team", "team.id = team_repo.team_id").
				Where("team_repo.repo_id = ?", repo.ID).
				Select("team.lower_name").
				OrderBy("team.lower_name").
				Find(&teams)
			metas["teams"] = "," + strings.Join(teams, ",") + ","
			metas["org"] = strings.ToLower(repo.OwnerName)
		}

		repo.RenderingMetas = metas
	}
	return repo.RenderingMetas
}

// ComposeDocumentMetas composes a map of metas for properly rendering documents
func (repo *Repository) ComposeDocumentMetas(ctx context.Context) map[string]string {
	if len(repo.DocumentRenderingMetas) == 0 {
		metas := map[string]string{}
		for k, v := range repo.ComposeMetas(ctx) {
			metas[k] = v
		}
		metas["mode"] = "document"
		repo.DocumentRenderingMetas = metas
	}
	return repo.DocumentRenderingMetas
}

// GetBaseRepo populates repo.BaseRepo for a fork repository and
// returns an error on failure (NOTE: no error is returned for
// non-fork repositories, and BaseRepo will be left untouched)
func (repo *Repository) GetBaseRepo(ctx context.Context) (err error) {
	if !repo.IsFork {
		return nil
	}

	repo.BaseRepo, err = GetRepositoryByID(ctx, repo.ForkID)
	return err
}

// IsGenerated returns whether _this_ repository was generated from a template
func (repo *Repository) IsGenerated() bool {
	return repo.TemplateID != 0
}

// RepoPath returns repository path by given user and repository name.
func RepoPath(userName, repoName string) string { //revive:disable-line:exported
	return filepath.Join(user_model.UserPath(userName), strings.ToLower(repoName)+".git")
}

// RepoPath returns the repository path
func (repo *Repository) RepoPath() string {
	return RepoPath(repo.OwnerName, repo.Name)
}

// Link returns the repository relative url
func (repo *Repository) Link() string {
	return setting.AppSubURL + "/" + url.PathEscape(repo.OwnerName) + "/" + url.PathEscape(repo.Name)
}

// ComposeCompareURL returns the repository comparison URL
func (repo *Repository) ComposeCompareURL(oldCommitID, newCommitID string) string {
	return fmt.Sprintf("%s/%s/compare/%s...%s", url.PathEscape(repo.OwnerName), url.PathEscape(repo.Name), util.PathEscapeSegments(oldCommitID), util.PathEscapeSegments(newCommitID))
}

func (repo *Repository) ComposeBranchCompareURL(baseRepo *Repository, branchName string) string {
	if baseRepo == nil {
		baseRepo = repo
	}
	var cmpBranchEscaped string
	if repo.ID != baseRepo.ID {
		cmpBranchEscaped = fmt.Sprintf("%s/%s:", url.PathEscape(repo.OwnerName), url.PathEscape(repo.Name))
	}
	cmpBranchEscaped = fmt.Sprintf("%s%s", cmpBranchEscaped, util.PathEscapeSegments(branchName))
	return fmt.Sprintf("%s/compare/%s...%s", baseRepo.Link(), util.PathEscapeSegments(baseRepo.DefaultBranch), cmpBranchEscaped)
}

// IsOwnedBy returns true when user owns this repository
func (repo *Repository) IsOwnedBy(userID int64) bool {
	return repo.OwnerID == userID
}

// CanCreateBranch returns true if repository meets the requirements for creating new branches.
func (repo *Repository) CanCreateBranch() bool {
	return !repo.IsMirror
}

// CanEnablePulls returns true if repository meets the requirements of accepting pulls.
func (repo *Repository) CanEnablePulls() bool {
	return !repo.IsMirror && !repo.IsEmpty
}

// AllowsPulls returns true if repository meets the requirements of accepting pulls and has them enabled.
func (repo *Repository) AllowsPulls(ctx context.Context) bool {
	return repo.CanEnablePulls() && repo.UnitEnabled(ctx, unit.TypePullRequests)
}

// CanEnableEditor returns true if repository meets the requirements of web editor.
func (repo *Repository) CanEnableEditor() bool {
	return !repo.IsMirror
}

// DescriptionHTML does special handles to description and return HTML string.
func (repo *Repository) DescriptionHTML(ctx context.Context) template.HTML {
	desc, err := markup.RenderDescriptionHTML(&markup.RenderContext{
		Ctx:       ctx,
		URLPrefix: repo.HTMLURL(),
		// Don't use Metas to speedup requests
	}, repo.Description)
	if err != nil {
		log.Error("Failed to render description for %s (ID: %d): %v", repo.Name, repo.ID, err)
		return template.HTML(markup.Sanitize(repo.Description))
	}
	return template.HTML(markup.Sanitize(desc))
}

// CloneLink represents different types of clone URLs of repository.
type CloneLink struct {
	SSH   string
	HTTPS string
}

// ComposeHTTPSCloneURL returns HTTPS clone URL based on given owner and repository name.
func ComposeHTTPSCloneURL(owner, repo string) string {
	return fmt.Sprintf("%s%s/%s.git", setting.AppURL, url.PathEscape(owner), url.PathEscape(repo))
}

func ComposeSSHCloneURL(ownerName, repoName string) string {
	sshUser := setting.SSH.User

	// if we have a ipv6 literal we need to put brackets around it
	// for the git cloning to work.
	sshDomain := setting.SSH.Domain
	ip := net.ParseIP(setting.SSH.Domain)
	if ip != nil && ip.To4() == nil {
		sshDomain = "[" + setting.SSH.Domain + "]"
	}

	if setting.SSH.Port != 22 {
		return fmt.Sprintf("ssh://%s@%s/%s/%s.git", sshUser,
			net.JoinHostPort(setting.SSH.Domain, strconv.Itoa(setting.SSH.Port)),
			url.PathEscape(ownerName),
			url.PathEscape(repoName))
	}
	if setting.Repository.UseCompatSSHURI {
		return fmt.Sprintf("ssh://%s@%s/%s/%s.git", sshUser, sshDomain, url.PathEscape(ownerName), url.PathEscape(repoName))
	}
	return fmt.Sprintf("%s@%s:%s/%s.git", sshUser, sshDomain, url.PathEscape(ownerName), url.PathEscape(repoName))
}

func (repo *Repository) cloneLink(isWiki bool) *CloneLink {
	repoName := repo.Name
	if isWiki {
		repoName += ".wiki"
	}

	cl := new(CloneLink)
	cl.SSH = ComposeSSHCloneURL(repo.OwnerName, repoName)
	cl.HTTPS = ComposeHTTPSCloneURL(repo.OwnerName, repoName)
	return cl
}

// CloneLink returns clone URLs of repository.
func (repo *Repository) CloneLink() (cl *CloneLink) {
	return repo.cloneLink(false)
}

// GetOriginalURLHostname returns the hostname of a URL or the URL
func (repo *Repository) GetOriginalURLHostname() string {
	u, err := url.Parse(repo.OriginalURL)
	if err != nil {
		return repo.OriginalURL
	}

	return u.Host
}

// GetTrustModel will get the TrustModel for the repo or the default trust model
func (repo *Repository) GetTrustModel() TrustModelType {
	trustModel := repo.TrustModel
	if trustModel == DefaultTrustModel {
		trustModel = ToTrustModel(setting.Repository.Signing.DefaultTrustModel)
		if trustModel == DefaultTrustModel {
			return CollaboratorTrustModel
		}
	}
	return trustModel
}

// MustNotBeArchived returns ErrRepoIsArchived if the repo is archived
func (repo *Repository) MustNotBeArchived() error {
	if repo.IsArchived {
		return ErrRepoIsArchived{Repo: repo}
	}
	return nil
}

// __________                           .__  __
// \______   \ ____ ______   ____  _____|__|/  |_  ___________ ___.__.
//  |       _// __ \\____ \ /  _ \/  ___/  \   __\/  _ \_  __ <   |  |
//  |    |   \  ___/|  |_> >  <_> )___ \|  ||  | (  <_> )  | \/\___  |
//  |____|_  /\___  >   __/ \____/____  >__||__|  \____/|__|   / ____|
//         \/     \/|__|              \/                       \/

// ErrRepoNotExist represents a "RepoNotExist" kind of error.
type ErrRepoNotExist struct {
	ID        int64
	UID       int64
	OwnerName string
	Name      string
}

// IsErrRepoNotExist checks if an error is a ErrRepoNotExist.
func IsErrRepoNotExist(err error) bool {
	_, ok := err.(ErrRepoNotExist)
	return ok
}

func (err ErrRepoNotExist) Error() string {
	return fmt.Sprintf("repository does not exist [id: %d, uid: %d, owner_name: %s, name: %s]",
		err.ID, err.UID, err.OwnerName, err.Name)
}

// Unwrap unwraps this error as a ErrNotExist error
func (err ErrRepoNotExist) Unwrap() error {
	return util.ErrNotExist
}

// GetRepositoryByOwnerAndName returns the repository by given owner name and repo name
func GetRepositoryByOwnerAndName(ctx context.Context, ownerName, repoName string) (*Repository, error) {
	var repo Repository
	has, err := db.GetEngine(ctx).Table("repository").Select("repository.*").
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
func GetRepositoryByName(ctx context.Context, ownerID int64, name string) (*Repository, error) {
	repo := &Repository{
		OwnerID:   ownerID,
		LowerName: strings.ToLower(name),
	}
	has, err := db.GetEngine(ctx).Get(repo)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRepoNotExist{0, ownerID, "", name}
	}
	return repo, err
}

// getRepositoryURLPathSegments returns segments (owner, reponame) extracted from a url
func getRepositoryURLPathSegments(repoURL string) []string {
	if strings.HasPrefix(repoURL, setting.AppURL) {
		return strings.Split(strings.TrimPrefix(repoURL, setting.AppURL), "/")
	}

	sshURLVariants := [4]string{
		setting.SSH.Domain + ":",
		setting.SSH.User + "@" + setting.SSH.Domain + ":",
		"git+ssh://" + setting.SSH.Domain + "/",
		"git+ssh://" + setting.SSH.User + "@" + setting.SSH.Domain + "/",
	}

	for _, sshURL := range sshURLVariants {
		if strings.HasPrefix(repoURL, sshURL) {
			return strings.Split(strings.TrimPrefix(repoURL, sshURL), "/")
		}
	}

	return nil
}

// GetRepositoryByURL returns the repository by given url
func GetRepositoryByURL(ctx context.Context, repoURL string) (*Repository, error) {
	// possible urls for git:
	//  https://my.domain/sub-path/<owner>/<repo>.git
	//  https://my.domain/sub-path/<owner>/<repo>
	//  git+ssh://user@my.domain/<owner>/<repo>.git
	//  git+ssh://user@my.domain/<owner>/<repo>
	//  user@my.domain:<owner>/<repo>.git
	//  user@my.domain:<owner>/<repo>

	pathSegments := getRepositoryURLPathSegments(repoURL)

	if len(pathSegments) != 2 {
		return nil, fmt.Errorf("unknown or malformed repository URL")
	}

	ownerName := pathSegments[0]
	repoName := strings.TrimSuffix(pathSegments[1], ".git")
	return GetRepositoryByOwnerAndName(ctx, ownerName, repoName)
}

// GetRepositoryByID returns the repository by given id if exists.
func GetRepositoryByID(ctx context.Context, id int64) (*Repository, error) {
	repo := new(Repository)
	has, err := db.GetEngine(ctx).ID(id).Get(repo)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRepoNotExist{id, 0, "", ""}
	}
	return repo, nil
}

// GetRepositoriesMapByIDs returns the repositories by given id slice.
func GetRepositoriesMapByIDs(ctx context.Context, ids []int64) (map[int64]*Repository, error) {
	repos := make(map[int64]*Repository, len(ids))
	return repos, db.GetEngine(ctx).In("id", ids).Find(&repos)
}

// IsRepositoryModelOrDirExist returns true if the repository with given name under user has already existed.
func IsRepositoryModelOrDirExist(ctx context.Context, u *user_model.User, repoName string) (bool, error) {
	has, err := IsRepositoryModelExist(ctx, u, repoName)
	if err != nil {
		return false, err
	}
	isDir, err := util.IsDir(RepoPath(u.Name, repoName))
	return has || isDir, err
}

func IsRepositoryModelExist(ctx context.Context, u *user_model.User, repoName string) (bool, error) {
	return db.GetEngine(ctx).Get(&Repository{
		OwnerID:   u.ID,
		LowerName: strings.ToLower(repoName),
	})
}

// GetTemplateRepo populates repo.TemplateRepo for a generated repository and
// returns an error on failure (NOTE: no error is returned for
// non-generated repositories, and TemplateRepo will be left untouched)
func GetTemplateRepo(ctx context.Context, repo *Repository) (*Repository, error) {
	if !repo.IsGenerated() {
		return nil, nil
	}

	return GetRepositoryByID(ctx, repo.TemplateID)
}

// TemplateRepo returns the repository, which is template of this repository
func (repo *Repository) TemplateRepo(ctx context.Context) *Repository {
	repo, err := GetTemplateRepo(ctx, repo)
	if err != nil {
		log.Error("TemplateRepo: %v", err)
		return nil
	}
	return repo
}

type CountRepositoryOptions struct {
	OwnerID int64
	Private util.OptionalBool
}

// CountRepositories returns number of repositories.
// Argument private only takes effect when it is false,
// set it true to count all repositories.
func CountRepositories(ctx context.Context, opts CountRepositoryOptions) (int64, error) {
	sess := db.GetEngine(ctx).Where("id > 0")

	if opts.OwnerID > 0 {
		sess.And("owner_id = ?", opts.OwnerID)
	}
	if !opts.Private.IsNone() {
		sess.And("is_private=?", opts.Private.IsTrue())
	}

	count, err := sess.Count(new(Repository))
	if err != nil {
		return 0, fmt.Errorf("countRepositories: %w", err)
	}
	return count, nil
}

// UpdateRepoIssueNumbers updates one of a repositories amount of (open|closed) (issues|PRs) with the current count
func UpdateRepoIssueNumbers(ctx context.Context, repoID int64, isPull, isClosed bool) error {
	field := "num_"
	if isClosed {
		field += "closed_"
	}
	if isPull {
		field += "pulls"
	} else {
		field += "issues"
	}

	subQuery := builder.Select("count(*)").
		From("issue").Where(builder.Eq{
		"repo_id": repoID,
		"is_pull": isPull,
	}.And(builder.If(isClosed, builder.Eq{"is_closed": isClosed})))

	// builder.Update(cond) will generate SQL like UPDATE ... SET cond
	query := builder.Update(builder.Eq{field: subQuery}).
		From("repository").
		Where(builder.Eq{"id": repoID})
	_, err := db.Exec(ctx, query)
	return err
}

// CountNullArchivedRepository counts the number of repositories with is_archived is null
func CountNullArchivedRepository(ctx context.Context) (int64, error) {
	return db.GetEngine(ctx).Where(builder.IsNull{"is_archived"}).Count(new(Repository))
}

// FixNullArchivedRepository sets is_archived to false where it is null
func FixNullArchivedRepository(ctx context.Context) (int64, error) {
	return db.GetEngine(ctx).Where(builder.IsNull{"is_archived"}).Cols("is_archived").NoAutoTime().Update(&Repository{
		IsArchived: false,
	})
}

// UpdateRepositoryOwnerName updates the owner name of all repositories owned by the user
func UpdateRepositoryOwnerName(ctx context.Context, oldUserName, newUserName string) error {
	if _, err := db.GetEngine(ctx).Exec("UPDATE `repository` SET owner_name=? WHERE owner_name=?", newUserName, oldUserName); err != nil {
		return fmt.Errorf("change repo owner name: %w", err)
	}
	return nil
}
