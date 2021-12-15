// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	_ "image/jpeg" // Needed for jpeg support

	admin_model "code.gitea.io/gitea/models/admin"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
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
		isDir, err := util.IsDir(customPath)
		if err != nil {
			log.Fatal("Failed to get custom %s files: %v", t, err)
		}
		if isDir {
			customFiles, err := util.StatDir(customPath)
			if err != nil {
				log.Fatal("Failed to get custom %s files: %v", t, err)
			}

			for _, f := range customFiles {
				if !util.IsStringInSlice(f, files, true) {
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
		if util.IsStringInSlice(name, Licenses, true) {
			sortedLicenses = append(sortedLicenses, name)
		}
	}
	for _, name := range Licenses {
		if !util.IsStringInSlice(name, setting.Repository.PreferredLicenses, true) {
			sortedLicenses = append(sortedLicenses, name)
		}
	}
	Licenses = sortedLicenses
}

// NewRepoContext creates a new repository context
func NewRepoContext() {
	loadRepoConfig()
	unit.LoadUnitConfig()

	admin_model.RemoveAllWithNotice(db.DefaultContext, "Clean up repository temporary data", filepath.Join(setting.AppDataPath, "tmp"))
}

// CheckRepoUnitUser check whether user could visit the unit of this repository
func CheckRepoUnitUser(repo *repo_model.Repository, user *user_model.User, unitType unit.Type) bool {
	return checkRepoUnitUser(db.DefaultContext, repo, user, unitType)
}

func checkRepoUnitUser(ctx context.Context, repo *repo_model.Repository, user *user_model.User, unitType unit.Type) bool {
	if user.IsAdmin {
		return true
	}
	perm, err := getUserRepoPermission(ctx, repo, user)
	if err != nil {
		log.Error("getUserRepoPermission(): %v", err)
		return false
	}

	return perm.CanRead(unitType)
}

func getRepoAssignees(ctx context.Context, repo *repo_model.Repository) (_ []*user_model.User, err error) {
	if err = repo.GetOwner(ctx); err != nil {
		return nil, err
	}

	e := db.GetEngine(ctx)
	accesses := make([]*Access, 0, 10)
	if err = e.
		Where("repo_id = ? AND mode >= ?", repo.ID, perm.AccessModeWrite).
		Find(&accesses); err != nil {
		return nil, err
	}

	// Leave a seat for owner itself to append later, but if owner is an organization
	// and just waste 1 unit is cheaper than re-allocate memory once.
	users := make([]*user_model.User, 0, len(accesses)+1)
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

// GetRepoAssignees returns all users that have write access and can be assigned to issues
// of the repository,
func GetRepoAssignees(repo *repo_model.Repository) (_ []*user_model.User, err error) {
	return getRepoAssignees(db.DefaultContext, repo)
}

func getReviewers(ctx context.Context, repo *repo_model.Repository, doerID, posterID int64) ([]*user_model.User, error) {
	// Get the owner of the repository - this often already pre-cached and if so saves complexity for the following queries
	if err := repo.GetOwner(ctx); err != nil {
		return nil, err
	}

	var users []*user_model.User
	e := db.GetEngine(ctx)

	if repo.IsPrivate || repo.Owner.Visibility == api.VisibleTypePrivate {
		// This a private repository:
		// Anyone who can read the repository is a requestable reviewer
		if err := e.
			SQL("SELECT * FROM `user` WHERE id in (SELECT user_id FROM `access` WHERE repo_id = ? AND mode >= ? AND user_id NOT IN ( ?, ?)) ORDER BY name",
				repo.ID, perm.AccessModeRead,
				doerID, posterID).
			Find(&users); err != nil {
			return nil, err
		}

		return users, nil
	}

	// This is a "public" repository:
	// Any user that has read access, is a watcher or organization member can be requested to review
	if err := e.
		SQL("SELECT * FROM `user` WHERE id IN ( "+
			"SELECT user_id FROM `access` WHERE repo_id = ? AND mode >= ? "+
			"UNION "+
			"SELECT user_id FROM `watch` WHERE repo_id = ? AND mode IN (?, ?) "+
			"UNION "+
			"SELECT uid AS user_id FROM `org_user` WHERE org_id = ? "+
			") AND id NOT IN (?, ?) ORDER BY name",
			repo.ID, perm.AccessModeRead,
			repo.ID, repo_model.WatchModeNormal, repo_model.WatchModeAuto,
			repo.OwnerID,
			doerID, posterID).
		Find(&users); err != nil {
		return nil, err
	}

	return users, nil
}

// GetReviewers get all users can be requested to review:
// * for private repositories this returns all users that have read access or higher to the repository.
// * for public repositories this returns all users that have read access or higher to the repository,
// all repo watchers and all organization members.
// TODO: may be we should have a busy choice for users to block review request to them.
func GetReviewers(repo *repo_model.Repository, doerID, posterID int64) ([]*user_model.User, error) {
	return getReviewers(db.DefaultContext, repo, doerID, posterID)
}

// GetReviewerTeams get all teams can be requested to review
func GetReviewerTeams(repo *repo_model.Repository) ([]*Team, error) {
	if err := repo.GetOwner(db.DefaultContext); err != nil {
		return nil, err
	}
	if !repo.Owner.IsOrganization() {
		return nil, nil
	}

	teams, err := GetTeamsWithAccessToRepo(repo.OwnerID, repo.ID, perm.AccessModeRead)
	if err != nil {
		return nil, err
	}

	return teams, err
}

func updateRepoSize(e db.Engine, repo *repo_model.Repository) error {
	size, err := util.GetDirectorySize(repo.RepoPath())
	if err != nil {
		return fmt.Errorf("updateSize: %v", err)
	}

	lfsSize, err := e.Where("repository_id = ?", repo.ID).SumInt(new(LFSMetaObject), "size")
	if err != nil {
		return fmt.Errorf("updateSize: GetLFSMetaObjects: %v", err)
	}

	repo.Size = size + lfsSize
	_, err = e.ID(repo.ID).Cols("size").NoAutoTime().Update(repo)
	return err
}

// UpdateRepoSize updates the repository size, calculating it using util.GetDirectorySize
func UpdateRepoSize(ctx context.Context, repo *repo_model.Repository) error {
	return updateRepoSize(db.GetEngine(ctx), repo)
}

// CanUserForkRepo returns true if specified user can fork repository.
func CanUserForkRepo(user *user_model.User, repo *repo_model.Repository) (bool, error) {
	if user == nil {
		return false, nil
	}
	if repo.OwnerID != user.ID && !repo_model.HasForkedRepo(user.ID, repo.ID) {
		return true, nil
	}
	ownedOrgs, err := GetOrgsCanCreateRepoByUserID(user.ID)
	if err != nil {
		return false, err
	}
	for _, org := range ownedOrgs {
		if repo.OwnerID != org.ID && !repo_model.HasForkedRepo(org.ID, repo.ID) {
			return true, nil
		}
	}
	return false, nil
}

// GetForksByUserAndOrgs return forked repos of the user and owned orgs
func GetForksByUserAndOrgs(user *user_model.User, repo *repo_model.Repository) ([]*repo_model.Repository, error) {
	var repoList []*repo_model.Repository
	if user == nil {
		return repoList, nil
	}
	var forkedRepo *repo_model.Repository
	forkedRepo, err := repo_model.GetUserFork(repo.ID, user.ID)
	if err != nil {
		return repoList, err
	}
	if forkedRepo != nil {
		repoList = append(repoList, forkedRepo)
	}
	canCreateRepos, err := GetOrgsCanCreateRepoByUserID(user.ID)
	if err != nil {
		return repoList, err
	}
	for _, org := range canCreateRepos {
		forkedRepo, err := repo_model.GetUserFork(repo.ID, org.ID)
		if err != nil {
			return repoList, err
		}
		if forkedRepo != nil {
			repoList = append(repoList, forkedRepo)
		}
	}
	return repoList, nil
}

// CanUserDelete returns true if user could delete the repository
func CanUserDelete(repo *repo_model.Repository, user *user_model.User) (bool, error) {
	if user.IsAdmin || user.ID == repo.OwnerID {
		return true, nil
	}

	if err := repo.GetOwner(db.DefaultContext); err != nil {
		return false, err
	}

	if repo.Owner.IsOrganization() {
		isOwner, err := OrgFromUser(repo.Owner).IsOwnedBy(user.ID)
		if err != nil {
			return false, err
		} else if isOwner {
			return true, nil
		}
	}

	return false, nil
}

// getUsersWithAccessMode returns users that have at least given access mode to the repository.
func getUsersWithAccessMode(ctx context.Context, repo *repo_model.Repository, mode perm.AccessMode) (_ []*user_model.User, err error) {
	if err = repo.GetOwner(ctx); err != nil {
		return nil, err
	}

	e := db.GetEngine(ctx)
	accesses := make([]*Access, 0, 10)
	if err = e.Where("repo_id = ? AND mode >= ?", repo.ID, mode).Find(&accesses); err != nil {
		return nil, err
	}

	// Leave a seat for owner itself to append later, but if owner is an organization
	// and just waste 1 unit is cheaper than re-allocate memory once.
	users := make([]*user_model.User, 0, len(accesses)+1)
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

// SetRepoReadBy sets repo to be visited by given user.
func SetRepoReadBy(repoID, userID int64) error {
	return setRepoNotificationStatusReadIfUnread(db.GetEngine(db.DefaultContext), userID, repoID)
}

// CreateRepoOptions contains the create repository options
type CreateRepoOptions struct {
	Name           string
	Description    string
	OriginalURL    string
	GitServiceType api.GitServiceType
	Gitignores     string
	IssueLabels    string
	License        string
	Readme         string
	DefaultBranch  string
	IsPrivate      bool
	IsMirror       bool
	IsTemplate     bool
	AutoInit       bool
	Status         repo_model.RepositoryStatus
	TrustModel     repo_model.TrustModelType
	MirrorInterval string
}

// GetRepoInitFile returns repository init files
func GetRepoInitFile(tp, name string) ([]byte, error) {
	cleanedName := strings.TrimLeft(path.Clean("/"+name), "/")
	relPath := path.Join("options", tp, cleanedName)

	// Use custom file when available.
	customPath := path.Join(setting.CustomPath, relPath)
	isFile, err := util.IsFile(customPath)
	if err != nil {
		log.Error("Unable to check if %s is a file. Error: %v", customPath, err)
	}
	if isFile {
		return os.ReadFile(customPath)
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

// CreateRepository creates a repository for the user/organization.
func CreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository, overwriteOrAdopt bool) (err error) {
	if err = repo_model.IsUsableRepoName(repo.Name); err != nil {
		return err
	}

	has, err := repo_model.IsRepositoryExistCtx(ctx, u, repo.Name)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return repo_model.ErrRepoAlreadyExist{
			Uname: u.Name,
			Name:  repo.Name,
		}
	}

	repoPath := repo_model.RepoPath(u.Name, repo.Name)
	isExist, err := util.IsExist(repoPath)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", repoPath, err)
		return err
	}
	if !overwriteOrAdopt && isExist {
		log.Error("Files already exist in %s and we are not going to adopt or delete.", repoPath)
		return repo_model.ErrRepoFilesAlreadyExist{
			Uname: u.Name,
			Name:  repo.Name,
		}
	}

	if err = db.Insert(ctx, repo); err != nil {
		return err
	}
	if err = repo_model.DeleteRedirect(ctx, u.ID, repo.Name); err != nil {
		return err
	}

	// insert units for repo
	units := make([]repo_model.RepoUnit, 0, len(unit.DefaultRepoUnits))
	for _, tp := range unit.DefaultRepoUnits {
		if tp == unit.TypeIssues {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
				Config: &repo_model.IssuesConfig{
					EnableTimetracker:                setting.Service.DefaultEnableTimetracking,
					AllowOnlyContributorsToTrackTime: setting.Service.DefaultAllowOnlyContributorsToTrackTime,
					EnableDependencies:               setting.Service.DefaultEnableDependencies,
				},
			})
		} else if tp == unit.TypePullRequests {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
				Config: &repo_model.PullRequestsConfig{AllowMerge: true, AllowRebase: true, AllowRebaseMerge: true, AllowSquash: true, DefaultMergeStyle: repo_model.MergeStyleMerge},
			})
		} else {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
			})
		}
	}

	if err = db.Insert(ctx, units); err != nil {
		return err
	}

	// Remember visibility preference.
	u.LastRepoVisibility = repo.IsPrivate
	if err = user_model.UpdateUserColsEngine(db.GetEngine(ctx), u, "last_repo_visibility"); err != nil {
		return fmt.Errorf("updateUser: %v", err)
	}

	if _, err = db.GetEngine(ctx).Incr("num_repos").ID(u.ID).Update(new(user_model.User)); err != nil {
		return fmt.Errorf("increment user total_repos: %v", err)
	}
	u.NumRepos++

	// Give access to all members in teams with access to all repositories.
	if u.IsOrganization() {
		teams, err := OrgFromUser(u).loadTeams(db.GetEngine(ctx))
		if err != nil {
			return fmt.Errorf("loadTeams: %v", err)
		}
		for _, t := range teams {
			if t.IncludesAllRepositories {
				if err := t.addRepository(ctx, repo); err != nil {
					return fmt.Errorf("addRepository: %v", err)
				}
			}
		}

		if isAdmin, err := isUserRepoAdmin(db.GetEngine(ctx), repo, doer); err != nil {
			return fmt.Errorf("isUserRepoAdmin: %v", err)
		} else if !isAdmin {
			// Make creator repo admin if it wan't assigned automatically
			if err = addCollaborator(ctx, repo, doer); err != nil {
				return fmt.Errorf("AddCollaborator: %v", err)
			}
			if err = changeCollaborationAccessMode(db.GetEngine(ctx), repo, doer.ID, perm.AccessModeAdmin); err != nil {
				return fmt.Errorf("ChangeCollaborationAccessMode: %v", err)
			}
		}
	} else if err = recalculateAccesses(ctx, repo); err != nil {
		// Organization automatically called this in addRepository method.
		return fmt.Errorf("recalculateAccesses: %v", err)
	}

	if setting.Service.AutoWatchNewRepos {
		if err = repo_model.WatchRepoCtx(ctx, doer.ID, repo.ID, true); err != nil {
			return fmt.Errorf("watchRepo: %v", err)
		}
	}

	if err = webhook.CopyDefaultWebhooksToRepo(ctx, repo.ID); err != nil {
		return fmt.Errorf("copyDefaultWebhooksToRepo: %v", err)
	}

	return nil
}

// CheckDaemonExportOK creates/removes git-daemon-export-ok for git-daemon...
func CheckDaemonExportOK(ctx context.Context, repo *repo_model.Repository) error {
	if err := repo.GetOwner(ctx); err != nil {
		return err
	}

	// Create/Remove git-daemon-export-ok for git-daemon...
	daemonExportFile := path.Join(repo.RepoPath(), `git-daemon-export-ok`)

	isExist, err := util.IsExist(daemonExportFile)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", daemonExportFile, err)
		return err
	}

	isPublic := !repo.IsPrivate && repo.Owner.Visibility == api.VisibleTypePublic
	if !isPublic && isExist {
		if err = util.Remove(daemonExportFile); err != nil {
			log.Error("Failed to remove %s: %v", daemonExportFile, err)
		}
	} else if isPublic && !isExist {
		if f, err := os.Create(daemonExportFile); err != nil {
			log.Error("Failed to create %s: %v", daemonExportFile, err)
		} else {
			f.Close()
		}
	}

	return nil
}

// IncrementRepoForkNum increment repository fork number
func IncrementRepoForkNum(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `repository` SET num_forks=num_forks+1 WHERE id=?", repoID)
	return err
}

// DecrementRepoForkNum decrement repository fork number
func DecrementRepoForkNum(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `repository` SET num_forks=num_forks-1 WHERE id=?", repoID)
	return err
}

func updateRepository(ctx context.Context, repo *repo_model.Repository, visibilityChanged bool) (err error) {
	repo.LowerName = strings.ToLower(repo.Name)

	if utf8.RuneCountInString(repo.Description) > 255 {
		repo.Description = string([]rune(repo.Description)[:255])
	}
	if utf8.RuneCountInString(repo.Website) > 255 {
		repo.Website = string([]rune(repo.Website)[:255])
	}

	e := db.GetEngine(ctx)

	if _, err = e.ID(repo.ID).AllCols().Update(repo); err != nil {
		return fmt.Errorf("update: %v", err)
	}

	if err = updateRepoSize(e, repo); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}

	if visibilityChanged {
		if err = repo.GetOwner(ctx); err != nil {
			return fmt.Errorf("getOwner: %v", err)
		}
		if repo.Owner.IsOrganization() {
			// Organization repository need to recalculate access table when visibility is changed.
			if err = recalculateTeamAccesses(ctx, repo, 0); err != nil {
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
		if err := CheckDaemonExportOK(db.WithEngine(ctx, e), repo); err != nil {
			return err
		}

		forkRepos, err := repo_model.GetRepositoriesByForkID(ctx, repo.ID)
		if err != nil {
			return fmt.Errorf("getRepositoriesByForkID: %v", err)
		}
		for i := range forkRepos {
			forkRepos[i].IsPrivate = repo.IsPrivate || repo.Owner.Visibility == api.VisibleTypePrivate
			if err = updateRepository(ctx, forkRepos[i], true); err != nil {
				return fmt.Errorf("updateRepository[%d]: %v", forkRepos[i].ID, err)
			}
		}
	}

	return nil
}

// UpdateRepositoryCtx updates a repository with db context
func UpdateRepositoryCtx(ctx context.Context, repo *repo_model.Repository, visibilityChanged bool) error {
	return updateRepository(ctx, repo, visibilityChanged)
}

// UpdateRepository updates a repository
func UpdateRepository(repo *repo_model.Repository, visibilityChanged bool) (err error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = updateRepository(ctx, repo, visibilityChanged); err != nil {
		return fmt.Errorf("updateRepository: %v", err)
	}

	return committer.Commit()
}

// DeleteRepository deletes a repository for a user or organization.
// make sure if you call this func to close open sessions (sqlite will otherwise get a deadlock)
func DeleteRepository(doer *user_model.User, uid, repoID int64) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	// In case is a organization.
	org, err := user_model.GetUserByIDEngine(sess, uid)
	if err != nil {
		return err
	}

	repo := &repo_model.Repository{OwnerID: uid}
	has, err := sess.ID(repoID).Get(repo)
	if err != nil {
		return err
	} else if !has {
		return repo_model.ErrRepoNotExist{
			ID:        repoID,
			UID:       uid,
			OwnerName: "",
			Name:      "",
		}
	}

	// Delete Deploy Keys
	deployKeys, err := asymkey_model.ListDeployKeys(ctx, &asymkey_model.ListDeployKeysOptions{RepoID: repoID})
	if err != nil {
		return fmt.Errorf("listDeployKeys: %v", err)
	}
	var needRewriteKeysFile = len(deployKeys) > 0
	for _, dKey := range deployKeys {
		if err := DeleteDeployKey(ctx, doer, dKey.ID); err != nil {
			return fmt.Errorf("deleteDeployKeys: %v", err)
		}
	}

	if cnt, err := sess.ID(repoID).Delete(&repo_model.Repository{}); err != nil {
		return err
	} else if cnt != 1 {
		return repo_model.ErrRepoNotExist{
			ID:        repoID,
			UID:       uid,
			OwnerName: "",
			Name:      "",
		}
	}

	if org.IsOrganization() {
		teams, err := OrgFromUser(org).loadTeams(sess)
		if err != nil {
			return err
		}
		for _, t := range teams {
			if !t.hasRepository(sess, repoID) {
				continue
			} else if err = t.removeRepository(ctx, repo, false); err != nil {
				return err
			}
		}
	}

	attachments := make([]*repo_model.Attachment, 0, 20)
	if err = sess.Join("INNER", "`release`", "`release`.id = `attachment`.release_id").
		Where("`release`.repo_id = ?", repoID).
		Find(&attachments); err != nil {
		return err
	}
	releaseAttachments := make([]string, 0, len(attachments))
	for i := 0; i < len(attachments); i++ {
		releaseAttachments = append(releaseAttachments, attachments[i].RelativePath())
	}

	if _, err := sess.Exec("UPDATE `user` SET num_stars=num_stars-1 WHERE id IN (SELECT `uid` FROM `star` WHERE repo_id = ?)", repo.ID); err != nil {
		return err
	}

	if err := deleteBeans(sess,
		&Access{RepoID: repo.ID},
		&Action{RepoID: repo.ID},
		&Collaboration{RepoID: repoID},
		&Comment{RefRepoID: repoID},
		&CommitStatus{RepoID: repoID},
		&DeletedBranch{RepoID: repoID},
		&webhook.HookTask{RepoID: repoID},
		&LFSLock{RepoID: repoID},
		&repo_model.LanguageStat{RepoID: repoID},
		&Milestone{RepoID: repoID},
		&repo_model.Mirror{RepoID: repoID},
		&Notification{RepoID: repoID},
		&ProtectedBranch{RepoID: repoID},
		&ProtectedTag{RepoID: repoID},
		&PullRequest{BaseRepoID: repoID},
		&repo_model.PushMirror{RepoID: repoID},
		&Release{RepoID: repoID},
		&repo_model.RepoIndexerStatus{RepoID: repoID},
		&repo_model.Redirect{RedirectRepoID: repoID},
		&repo_model.RepoUnit{RepoID: repoID},
		&repo_model.Star{RepoID: repoID},
		&Task{RepoID: repoID},
		&repo_model.Watch{RepoID: repoID},
		&webhook.Webhook{RepoID: repoID},
	); err != nil {
		return fmt.Errorf("deleteBeans: %v", err)
	}

	// Delete Labels and related objects
	if err := deleteLabelsByRepoID(sess, repoID); err != nil {
		return err
	}

	// Delete Issues and related objects
	var attachmentPaths []string
	if attachmentPaths, err = deleteIssuesByRepoID(sess, repoID); err != nil {
		return err
	}

	// Delete issue index
	if err := db.DeleteResouceIndex(sess, "issue_index", repoID); err != nil {
		return err
	}

	if repo.IsFork {
		if _, err := sess.Exec("UPDATE `repository` SET num_forks=num_forks-1 WHERE id=?", repo.ForkID); err != nil {
			return fmt.Errorf("decrease fork count: %v", err)
		}
	}

	if _, err := sess.Exec("UPDATE `user` SET num_repos=num_repos-1 WHERE id=?", uid); err != nil {
		return err
	}

	if len(repo.Topics) > 0 {
		if err := repo_model.RemoveTopicsFromRepo(ctx, repo.ID); err != nil {
			return err
		}
	}

	projects, _, err := getProjects(sess, ProjectSearchOptions{
		RepoID: repoID,
	})
	if err != nil {
		return fmt.Errorf("get projects: %v", err)
	}
	for i := range projects {
		if err := deleteProjectByID(sess, projects[i].ID); err != nil {
			return fmt.Errorf("delete project [%d]: %v", projects[i].ID, err)
		}
	}

	// Remove LFS objects
	var lfsObjects []*LFSMetaObject
	if err = sess.Where("repository_id=?", repoID).Find(&lfsObjects); err != nil {
		return err
	}

	var lfsPaths = make([]string, 0, len(lfsObjects))
	for _, v := range lfsObjects {
		count, err := sess.Count(&LFSMetaObject{Pointer: lfs.Pointer{Oid: v.Oid}})
		if err != nil {
			return err
		}
		if count > 1 {
			continue
		}

		lfsPaths = append(lfsPaths, v.RelativePath())
	}

	if _, err := sess.Delete(&LFSMetaObject{RepositoryID: repoID}); err != nil {
		return err
	}

	// Remove archives
	var archives []*repo_model.RepoArchiver
	if err = sess.Where("repo_id=?", repoID).Find(&archives); err != nil {
		return err
	}

	var archivePaths = make([]string, 0, len(archives))
	for _, v := range archives {
		p, _ := v.RelativePath()
		archivePaths = append(archivePaths, p)
	}

	if _, err := sess.Delete(&repo_model.RepoArchiver{RepoID: repoID}); err != nil {
		return err
	}

	if repo.NumForks > 0 {
		if _, err = sess.Exec("UPDATE `repository` SET fork_id=0,is_fork=? WHERE fork_id=?", false, repo.ID); err != nil {
			log.Error("reset 'fork_id' and 'is_fork': %v", err)
		}
	}

	// Get all attachments with both issue_id and release_id are zero
	var newAttachments []*repo_model.Attachment
	if err := sess.Where(builder.Eq{
		"repo_id":    repo.ID,
		"issue_id":   0,
		"release_id": 0,
	}).Find(&newAttachments); err != nil {
		return err
	}

	var newAttachmentPaths = make([]string, 0, len(newAttachments))
	for _, attach := range newAttachments {
		newAttachmentPaths = append(newAttachmentPaths, attach.RelativePath())
	}

	if _, err := sess.Where("repo_id=?", repo.ID).Delete(new(repo_model.Attachment)); err != nil {
		return err
	}

	if err = committer.Commit(); err != nil {
		return err
	}

	committer.Close()

	if needRewriteKeysFile {
		if err := asymkey_model.RewriteAllPublicKeys(); err != nil {
			log.Error("RewriteAllPublicKeys failed: %v", err)
		}
	}

	// We should always delete the files after the database transaction succeed. If
	// we delete the file but the database rollback, the repository will be broken.

	// Remove repository files.
	repoPath := repo.RepoPath()
	admin_model.RemoveAllWithNotice(db.DefaultContext, "Delete repository files", repoPath)

	// Remove wiki files
	if repo.HasWiki() {
		admin_model.RemoveAllWithNotice(db.DefaultContext, "Delete repository wiki", repo.WikiPath())
	}

	// Remove archives
	for i := range archivePaths {
		admin_model.RemoveStorageWithNotice(db.DefaultContext, storage.RepoArchives, "Delete repo archive file", archivePaths[i])
	}

	// Remove lfs objects
	for i := range lfsPaths {
		admin_model.RemoveStorageWithNotice(db.DefaultContext, storage.LFS, "Delete orphaned LFS file", lfsPaths[i])
	}

	// Remove issue attachment files.
	for i := range attachmentPaths {
		admin_model.RemoveStorageWithNotice(db.DefaultContext, storage.Attachments, "Delete issue attachment", attachmentPaths[i])
	}

	// Remove release attachment files.
	for i := range releaseAttachments {
		admin_model.RemoveStorageWithNotice(db.DefaultContext, storage.Attachments, "Delete release attachment", releaseAttachments[i])
	}

	// Remove attachment with no issue_id and release_id.
	for i := range newAttachmentPaths {
		admin_model.RemoveStorageWithNotice(db.DefaultContext, storage.Attachments, "Delete issue attachment", attachmentPaths[i])
	}

	if len(repo.Avatar) > 0 {
		if err := storage.RepoAvatars.Delete(repo.CustomAvatarRelativePath()); err != nil {
			return fmt.Errorf("Failed to remove %s: %v", repo.Avatar, err)
		}
	}

	return nil
}

type repoChecker struct {
	querySQL, correctSQL string
	desc                 string
}

func repoStatsCheck(ctx context.Context, checker *repoChecker) {
	results, err := db.GetEngine(db.DefaultContext).Query(checker.querySQL)
	if err != nil {
		log.Error("Select %s: %v", checker.desc, err)
		return
	}
	for _, result := range results {
		id, _ := strconv.ParseInt(string(result["id"]), 10, 64)
		select {
		case <-ctx.Done():
			log.Warn("CheckRepoStats: Cancelled before checking %s for Repo[%d]", checker.desc, id)
			return
		default:
		}
		log.Trace("Updating %s: %d", checker.desc, id)
		_, err = db.GetEngine(db.DefaultContext).Exec(checker.correctSQL, id, id)
		if err != nil {
			log.Error("Update %s[%d]: %v", checker.desc, id, err)
		}
	}
}

// CheckRepoStats checks the repository stats
func CheckRepoStats(ctx context.Context) error {
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
	for _, checker := range checkers {
		select {
		case <-ctx.Done():
			log.Warn("CheckRepoStats: Cancelled before %s", checker.desc)
			return db.ErrCancelledf("before checking %s", checker.desc)
		default:
			repoStatsCheck(ctx, checker)
		}
	}

	// ***** START: Repository.NumClosedIssues *****
	desc := "repository count 'num_closed_issues'"
	results, err := db.GetEngine(db.DefaultContext).Query("SELECT repo.id FROM `repository` repo WHERE repo.num_closed_issues!=(SELECT COUNT(*) FROM `issue` WHERE repo_id=repo.id AND is_closed=? AND is_pull=?)", true, false)
	if err != nil {
		log.Error("Select %s: %v", desc, err)
	} else {
		for _, result := range results {
			id, _ := strconv.ParseInt(string(result["id"]), 10, 64)
			select {
			case <-ctx.Done():
				log.Warn("CheckRepoStats: Cancelled during %s for repo ID %d", desc, id)
				return db.ErrCancelledf("during %s for repo ID %d", desc, id)
			default:
			}
			log.Trace("Updating %s: %d", desc, id)
			_, err = db.GetEngine(db.DefaultContext).Exec("UPDATE `repository` SET num_closed_issues=(SELECT COUNT(*) FROM `issue` WHERE repo_id=? AND is_closed=? AND is_pull=?) WHERE id=?", id, true, false, id)
			if err != nil {
				log.Error("Update %s[%d]: %v", desc, id, err)
			}
		}
	}
	// ***** END: Repository.NumClosedIssues *****

	// ***** START: Repository.NumClosedPulls *****
	desc = "repository count 'num_closed_pulls'"
	results, err = db.GetEngine(db.DefaultContext).Query("SELECT repo.id FROM `repository` repo WHERE repo.num_closed_pulls!=(SELECT COUNT(*) FROM `issue` WHERE repo_id=repo.id AND is_closed=? AND is_pull=?)", true, true)
	if err != nil {
		log.Error("Select %s: %v", desc, err)
	} else {
		for _, result := range results {
			id, _ := strconv.ParseInt(string(result["id"]), 10, 64)
			select {
			case <-ctx.Done():
				log.Warn("CheckRepoStats: Cancelled")
				return db.ErrCancelledf("during %s for repo ID %d", desc, id)
			default:
			}
			log.Trace("Updating %s: %d", desc, id)
			_, err = db.GetEngine(db.DefaultContext).Exec("UPDATE `repository` SET num_closed_pulls=(SELECT COUNT(*) FROM `issue` WHERE repo_id=? AND is_closed=? AND is_pull=?) WHERE id=?", id, true, true, id)
			if err != nil {
				log.Error("Update %s[%d]: %v", desc, id, err)
			}
		}
	}
	// ***** END: Repository.NumClosedPulls *****

	// FIXME: use checker when stop supporting old fork repo format.
	// ***** START: Repository.NumForks *****
	results, err = db.GetEngine(db.DefaultContext).Query("SELECT repo.id FROM `repository` repo WHERE repo.num_forks!=(SELECT COUNT(*) FROM `repository` WHERE fork_id=repo.id)")
	if err != nil {
		log.Error("Select repository count 'num_forks': %v", err)
	} else {
		for _, result := range results {
			id, _ := strconv.ParseInt(string(result["id"]), 10, 64)
			select {
			case <-ctx.Done():
				log.Warn("CheckRepoStats: Cancelled")
				return db.ErrCancelledf("during %s for repo ID %d", desc, id)
			default:
			}
			log.Trace("Updating repository count 'num_forks': %d", id)

			repo, err := repo_model.GetRepositoryByID(id)
			if err != nil {
				log.Error("repo_model.GetRepositoryByID[%d]: %v", id, err)
				continue
			}

			rawResult, err := db.GetEngine(db.DefaultContext).Query("SELECT COUNT(*) FROM `repository` WHERE fork_id=?", repo.ID)
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
	return nil
}

func updateUserStarNumbers(users []user_model.User) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	for _, user := range users {
		if _, err = db.Exec(ctx, "UPDATE `user` SET num_stars=(SELECT COUNT(*) FROM `star` WHERE uid=?) WHERE id=?", user.ID, user.ID); err != nil {
			return err
		}
	}

	return committer.Commit()
}

// DoctorUserStarNum recalculate Stars number for all user
func DoctorUserStarNum() (err error) {
	const batchSize = 100

	for start := 0; ; start += batchSize {
		users := make([]user_model.User, 0, batchSize)
		if err = db.GetEngine(db.DefaultContext).Limit(batchSize, start).Where("type = ?", 0).Cols("id").Find(&users); err != nil {
			return
		}
		if len(users) == 0 {
			break
		}

		if err = updateUserStarNumbers(users); err != nil {
			return
		}
	}

	log.Debug("recalculate Stars number for all user finished")

	return
}

// LinkedRepository returns the linked repo if any
func LinkedRepository(a *repo_model.Attachment) (*repo_model.Repository, unit.Type, error) {
	if a.IssueID != 0 {
		iss, err := GetIssueByID(a.IssueID)
		if err != nil {
			return nil, unit.TypeIssues, err
		}
		repo, err := repo_model.GetRepositoryByID(iss.RepoID)
		unitType := unit.TypeIssues
		if iss.IsPull {
			unitType = unit.TypePullRequests
		}
		return repo, unitType, err
	} else if a.ReleaseID != 0 {
		rel, err := GetReleaseByID(a.ReleaseID)
		if err != nil {
			return nil, unit.TypeReleases, err
		}
		repo, err := repo_model.GetRepositoryByID(rel.RepoID)
		return repo, unit.TypeReleases, err
	}
	return nil, -1, nil
}

// DeleteDeployKey delete deploy keys
func DeleteDeployKey(ctx context.Context, doer *user_model.User, id int64) error {
	key, err := asymkey_model.GetDeployKeyByID(ctx, id)
	if err != nil {
		if asymkey_model.IsErrDeployKeyNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetDeployKeyByID: %v", err)
	}

	sess := db.GetEngine(ctx)

	// Check if user has access to delete this key.
	if !doer.IsAdmin {
		repo, err := repo_model.GetRepositoryByIDCtx(ctx, key.RepoID)
		if err != nil {
			return fmt.Errorf("GetRepositoryByID: %v", err)
		}
		has, err := isUserRepoAdmin(sess, repo, doer)
		if err != nil {
			return fmt.Errorf("GetUserRepoPermission: %v", err)
		} else if !has {
			return asymkey_model.ErrKeyAccessDenied{
				UserID: doer.ID,
				KeyID:  key.ID,
				Note:   "deploy",
			}
		}
	}

	if _, err = sess.ID(key.ID).Delete(new(asymkey_model.DeployKey)); err != nil {
		return fmt.Errorf("delete deploy key [%d]: %v", key.ID, err)
	}

	// Check if this is the last reference to same key content.
	has, err := sess.
		Where("key_id = ?", key.KeyID).
		Get(new(asymkey_model.DeployKey))
	if err != nil {
		return err
	} else if !has {
		if err = asymkey_model.DeletePublicKeys(ctx, key.KeyID); err != nil {
			return err
		}
	}

	return nil
}
