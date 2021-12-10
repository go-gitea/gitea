// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

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

	IsPrivate  bool `xorm:"INDEX"`
	IsEmpty    bool `xorm:"INDEX"`
	IsArchived bool `xorm:"INDEX"`
	IsMirror   bool `xorm:"INDEX"`
	*Mirror    `xorm:"-"`
	Status     RepositoryStatus `xorm:"NOT NULL DEFAULT 0"`

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
	CodeIndexerStatus               *RepoIndexerStatus `xorm:"-"`
	StatsIndexerStatus              *RepoIndexerStatus `xorm:"-"`
	IsFsckEnabled                   bool               `xorm:"NOT NULL DEFAULT true"`
	CloseIssuesViaCommitInAnyBranch bool               `xorm:"NOT NULL DEFAULT false"`
	Topics                          []string           `xorm:"TEXT JSON"`

	TrustModel TrustModelType

	// Avatar: ID(10-20)-md5(32) - must fit into 64 symbols
	Avatar string `xorm:"VARCHAR(64)"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(Repository))
}

// SanitizedOriginalURL returns a sanitized OriginalURL
func (repo *Repository) SanitizedOriginalURL() string {
	if repo.OriginalURL == "" {
		return ""
	}
	u, err := url.Parse(repo.OriginalURL)
	if err != nil {
		return ""
	}
	u.User = nil
	return u.String()
}

// ColorFormat returns a colored string to represent this repo
func (repo *Repository) ColorFormat(s fmt.State) {
	log.ColorFprintf(s, "%d:%s/%s",
		log.NewColoredIDValue(repo.ID),
		repo.OwnerName,
		repo.Name)
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

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (repo *Repository) AfterLoad() {
	// FIXME: use models migration to solve all at once.
	if len(repo.DefaultBranch) == 0 {
		repo.DefaultBranch = setting.Repository.DefaultBranch
	}

	repo.NumOpenIssues = repo.NumIssues - repo.NumClosedIssues
	repo.NumOpenPulls = repo.NumPulls - repo.NumClosedPulls
	repo.NumOpenMilestones = repo.NumMilestones - repo.NumClosedMilestones
	repo.NumOpenProjects = repo.NumProjects - repo.NumClosedProjects
}

// MustOwner always returns a valid *user_model.User object to avoid
// conceptually impossible error handling.
// It creates a fake object that contains error details
// when error occurs.
func (repo *Repository) MustOwner() *user_model.User {
	return repo.mustOwner(db.DefaultContext)
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
		result = repo.HTMLURL() + "/commit/" + url.PathEscape(commitID)
	}
	return
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

	repo.Units, err = getUnitsByRepoID(db.GetEngine(ctx), repo.ID)
	log.Trace("repo.Units: %-+v", repo.Units)
	return err
}

// UnitEnabled if this repository has the given unit enabled
func (repo *Repository) UnitEnabled(tp unit.Type) bool {
	if err := repo.LoadUnits(db.DefaultContext); err != nil {
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
func (repo *Repository) MustGetUnit(tp unit.Type) *RepoUnit {
	ru, err := repo.GetUnit(tp)
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
	}
	return &RepoUnit{
		Type:   tp,
		Config: new(UnitConfig),
	}
}

// GetUnit returns a RepoUnit object
func (repo *Repository) GetUnit(tp unit.Type) (*RepoUnit, error) {
	return repo.getUnit(db.DefaultContext, tp)
}

func (repo *Repository) getUnit(ctx context.Context, tp unit.Type) (*RepoUnit, error) {
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

// GetOwner returns the repository owner
func (repo *Repository) GetOwner(ctx context.Context) (err error) {
	if repo.Owner != nil {
		return nil
	}

	repo.Owner, err = user_model.GetUserByIDEngine(db.GetEngine(ctx), repo.OwnerID)
	return err
}

func (repo *Repository) mustOwner(ctx context.Context) *user_model.User {
	if err := repo.GetOwner(ctx); err != nil {
		return &user_model.User{
			Name:     "error",
			FullName: err.Error(),
		}
	}

	return repo.Owner
}

// ComposeMetas composes a map of metas for properly rendering issue links and external issue trackers.
func (repo *Repository) ComposeMetas() map[string]string {
	if len(repo.RenderingMetas) == 0 {
		metas := map[string]string{
			"user":     repo.OwnerName,
			"repo":     repo.Name,
			"repoPath": repo.RepoPath(),
			"mode":     "comment",
		}

		unit, err := repo.GetUnit(unit.TypeExternalTracker)
		if err == nil {
			metas["format"] = unit.ExternalTrackerConfig().ExternalTrackerFormat
			switch unit.ExternalTrackerConfig().ExternalTrackerStyle {
			case markup.IssueNameStyleAlphanumeric:
				metas["style"] = markup.IssueNameStyleAlphanumeric
			default:
				metas["style"] = markup.IssueNameStyleNumeric
			}
		}

		repo.MustOwner()
		if repo.Owner.IsOrganization() {
			teams := make([]string, 0, 5)
			_ = db.GetEngine(db.DefaultContext).Table("team_repo").
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
func (repo *Repository) ComposeDocumentMetas() map[string]string {
	if len(repo.DocumentRenderingMetas) == 0 {
		metas := map[string]string{}
		for k, v := range repo.ComposeMetas() {
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
func (repo *Repository) GetBaseRepo() (err error) {
	return repo.getBaseRepo(db.GetEngine(db.DefaultContext))
}

func (repo *Repository) getBaseRepo(e db.Engine) (err error) {
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

// RepoPath returns repository path by given user and repository name.
func RepoPath(userName, repoName string) string { //revive:disable-line:exported
	return filepath.Join(user_model.UserPath(userName), strings.ToLower(repoName)+".git")
}

// RepoPath returns the repository path
func (repo *Repository) RepoPath() string {
	return RepoPath(repo.OwnerName, repo.Name)
}

// GitConfigPath returns the path to a repository's git config/ directory
func GitConfigPath(repoPath string) string {
	return filepath.Join(repoPath, "config")
}

// GitConfigPath returns the repository git config path
func (repo *Repository) GitConfigPath() string {
	return GitConfigPath(repo.RepoPath())
}

// Link returns the repository link
func (repo *Repository) Link() string {
	return setting.AppSubURL + "/" + url.PathEscape(repo.OwnerName) + "/" + url.PathEscape(repo.Name)
}

// ComposeCompareURL returns the repository comparison URL
func (repo *Repository) ComposeCompareURL(oldCommitID, newCommitID string) string {
	return fmt.Sprintf("%s/%s/compare/%s...%s", url.PathEscape(repo.OwnerName), url.PathEscape(repo.Name), util.PathEscapeSegments(oldCommitID), util.PathEscapeSegments(newCommitID))
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
func (repo *Repository) AllowsPulls() bool {
	return repo.CanEnablePulls() && repo.UnitEnabled(unit.TypePullRequests)
}

// CanEnableEditor returns true if repository meets the requirements of web editor.
func (repo *Repository) CanEnableEditor() bool {
	return !repo.IsMirror
}

// DescriptionHTML does special handles to description and return HTML string.
func (repo *Repository) DescriptionHTML() template.HTML {
	desc, err := markup.RenderDescriptionHTML(&markup.RenderContext{
		URLPrefix: repo.HTMLURL(),
		Metas:     repo.ComposeMetas(),
	}, repo.Description)
	if err != nil {
		log.Error("Failed to render description for %s (ID: %d): %v", repo.Name, repo.ID, err)
		return template.HTML(markup.Sanitize(repo.Description))
	}
	return template.HTML(markup.Sanitize(string(desc)))
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

func (repo *Repository) cloneLink(isWiki bool) *CloneLink {
	repoName := repo.Name
	if isWiki {
		repoName += ".wiki"
	}

	sshUser := setting.RunUser
	if setting.SSH.StartBuiltinServer {
		sshUser = setting.SSH.BuiltinServerUser
	}

	cl := new(CloneLink)

	// if we have a ipv6 literal we need to put brackets around it
	// for the git cloning to work.
	sshDomain := setting.SSH.Domain
	ip := net.ParseIP(setting.SSH.Domain)
	if ip != nil && ip.To4() == nil {
		sshDomain = "[" + setting.SSH.Domain + "]"
	}

	if setting.SSH.Port != 22 {
		cl.SSH = fmt.Sprintf("ssh://%s@%s/%s/%s.git", sshUser, net.JoinHostPort(setting.SSH.Domain, strconv.Itoa(setting.SSH.Port)), url.PathEscape(repo.OwnerName), url.PathEscape(repoName))
	} else if setting.Repository.UseCompatSSHURI {
		cl.SSH = fmt.Sprintf("ssh://%s@%s/%s/%s.git", sshUser, sshDomain, url.PathEscape(repo.OwnerName), url.PathEscape(repoName))
	} else {
		cl.SSH = fmt.Sprintf("%s@%s:%s/%s.git", sshUser, sshDomain, url.PathEscape(repo.OwnerName), url.PathEscape(repoName))
	}
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

// GetRepositoryByOwnerAndName returns the repository by given ownername and reponame.
func GetRepositoryByOwnerAndName(ownerName, repoName string) (*Repository, error) {
	return GetRepositoryByOwnerAndNameCtx(db.DefaultContext, ownerName, repoName)
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

// GetRepositoryByOwnerAndNameCtx returns the repository by given owner name and repo name
func GetRepositoryByOwnerAndNameCtx(ctx context.Context, ownerName, repoName string) (*Repository, error) {
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
func GetRepositoryByName(ownerID int64, name string) (*Repository, error) {
	repo := &Repository{
		OwnerID:   ownerID,
		LowerName: strings.ToLower(name),
	}
	has, err := db.GetEngine(db.DefaultContext).Get(repo)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRepoNotExist{0, ownerID, "", name}
	}
	return repo, err
}

func getRepositoryByID(e db.Engine, id int64) (*Repository, error) {
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
	return getRepositoryByID(db.GetEngine(db.DefaultContext), id)
}

// GetRepositoryByIDCtx returns the repository by given id if exists.
func GetRepositoryByIDCtx(ctx context.Context, id int64) (*Repository, error) {
	return getRepositoryByID(db.GetEngine(ctx), id)
}

// GetRepositoriesMapByIDs returns the repositories by given id slice.
func GetRepositoriesMapByIDs(ids []int64) (map[int64]*Repository, error) {
	repos := make(map[int64]*Repository, len(ids))
	return repos, db.GetEngine(db.DefaultContext).In("id", ids).Find(&repos)
}

// IsRepositoryExistCtx returns true if the repository with given name under user has already existed.
func IsRepositoryExistCtx(ctx context.Context, u *user_model.User, repoName string) (bool, error) {
	has, err := db.GetEngine(ctx).Get(&Repository{
		OwnerID:   u.ID,
		LowerName: strings.ToLower(repoName),
	})
	if err != nil {
		return false, err
	}
	isDir, err := util.IsDir(RepoPath(u.Name, repoName))
	return has && isDir, err
}

// IsRepositoryExist returns true if the repository with given name under user has already existed.
func IsRepositoryExist(u *user_model.User, repoName string) (bool, error) {
	return IsRepositoryExistCtx(db.DefaultContext, u, repoName)
}

// GetTemplateRepo populates repo.TemplateRepo for a generated repository and
// returns an error on failure (NOTE: no error is returned for
// non-generated repositories, and TemplateRepo will be left untouched)
func GetTemplateRepo(repo *Repository) (*Repository, error) {
	return getTemplateRepo(db.GetEngine(db.DefaultContext), repo)
}

func getTemplateRepo(e db.Engine, repo *Repository) (*Repository, error) {
	if !repo.IsGenerated() {
		return nil, nil
	}

	return getRepositoryByID(e, repo.TemplateID)
}

func countRepositories(userID int64, private bool) int64 {
	sess := db.GetEngine(db.DefaultContext).Where("id > 0")

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

// GetUserMirrorRepositories returns a list of mirror repositories of given user.
func GetUserMirrorRepositories(userID int64) ([]*Repository, error) {
	repos := make([]*Repository, 0, 10)
	return repos, db.GetEngine(db.DefaultContext).
		Where("owner_id = ?", userID).
		And("is_mirror = ?", true).
		Find(&repos)
}

func getRepositoryCount(e db.Engine, ownerID int64) (int64, error) {
	return e.Count(&Repository{OwnerID: ownerID})
}

func getPublicRepositoryCount(e db.Engine, u *user_model.User) (int64, error) {
	return e.Where("is_private = ?", false).Count(&Repository{OwnerID: u.ID})
}

func getPrivateRepositoryCount(e db.Engine, u *user_model.User) (int64, error) {
	return e.Where("is_private = ?", true).Count(&Repository{OwnerID: u.ID})
}

// GetRepositoryCount returns the total number of repositories of user.
func GetRepositoryCount(ctx context.Context, ownerID int64) (int64, error) {
	return getRepositoryCount(db.GetEngine(ctx), ownerID)
}

// GetPublicRepositoryCount returns the total number of public repositories of user.
func GetPublicRepositoryCount(u *user_model.User) (int64, error) {
	return getPublicRepositoryCount(db.GetEngine(db.DefaultContext), u)
}

// GetPrivateRepositoryCount returns the total number of private repositories of user.
func GetPrivateRepositoryCount(u *user_model.User) (int64, error) {
	return getPrivateRepositoryCount(db.GetEngine(db.DefaultContext), u)
}
