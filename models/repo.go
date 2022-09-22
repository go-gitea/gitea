// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"
	"strconv"

	_ "image/jpeg" // Needed for jpeg support

	admin_model "code.gitea.io/gitea/models/admin"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// NewRepoContext creates a new repository context
func NewRepoContext() {
	unit.LoadUnitConfig()
}

// CheckRepoUnitUser check whether user could visit the unit of this repository
func CheckRepoUnitUser(ctx context.Context, repo *repo_model.Repository, user *user_model.User, unitType unit.Type) bool {
	if user != nil && user.IsAdmin {
		return true
	}
	perm, err := access_model.GetUserRepoPermission(ctx, repo, user)
	if err != nil {
		log.Error("GetUserRepoPermission(): %v", err)
		return false
	}

	return perm.CanRead(unitType)
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

// CreateRepository creates a repository for the user/organization.
func CreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository, overwriteOrAdopt bool) (err error) {
	if err = repo_model.IsUsableRepoName(repo.Name); err != nil {
		return err
	}

	has, err := repo_model.IsRepositoryExist(ctx, u, repo.Name)
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
				Config: &repo_model.PullRequestsConfig{AllowMerge: true, AllowRebase: true, AllowRebaseMerge: true, AllowSquash: true, DefaultMergeStyle: repo_model.MergeStyle(setting.Repository.PullRequest.DefaultMergeStyle), AllowRebaseUpdate: true},
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
	if err = user_model.UpdateUserCols(ctx, u, "last_repo_visibility"); err != nil {
		return fmt.Errorf("updateUser: %v", err)
	}

	if _, err = db.GetEngine(ctx).Incr("num_repos").ID(u.ID).Update(new(user_model.User)); err != nil {
		return fmt.Errorf("increment user total_repos: %v", err)
	}
	u.NumRepos++

	// Give access to all members in teams with access to all repositories.
	if u.IsOrganization() {
		teams, err := organization.FindOrgTeams(ctx, u.ID)
		if err != nil {
			return fmt.Errorf("loadTeams: %v", err)
		}
		for _, t := range teams {
			if t.IncludesAllRepositories {
				if err := addRepository(ctx, t, repo); err != nil {
					return fmt.Errorf("addRepository: %v", err)
				}
			}
		}

		if isAdmin, err := access_model.IsUserRepoAdmin(ctx, repo, doer); err != nil {
			return fmt.Errorf("IsUserRepoAdminCtx: %v", err)
		} else if !isAdmin {
			// Make creator repo admin if it wasn't assigned automatically
			if err = addCollaborator(ctx, repo, doer); err != nil {
				return fmt.Errorf("AddCollaborator: %v", err)
			}
			if err = repo_model.ChangeCollaborationAccessModeCtx(ctx, repo, doer.ID, perm.AccessModeAdmin); err != nil {
				return fmt.Errorf("ChangeCollaborationAccessMode: %v", err)
			}
		}
	} else if err = access_model.RecalculateAccesses(ctx, repo); err != nil {
		// Organization automatically called this in addRepository method.
		return fmt.Errorf("recalculateAccesses: %v", err)
	}

	if setting.Service.AutoWatchNewRepos {
		if err = repo_model.WatchRepo(ctx, doer.ID, repo.ID, true); err != nil {
			return fmt.Errorf("watchRepo: %v", err)
		}
	}

	if err = webhook.CopyDefaultWebhooksToRepo(ctx, repo.ID); err != nil {
		return fmt.Errorf("copyDefaultWebhooksToRepo: %v", err)
	}

	return nil
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
	org, err := user_model.GetUserByIDCtx(ctx, uid)
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
	needRewriteKeysFile := len(deployKeys) > 0
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
		teams, err := organization.FindOrgTeams(ctx, org.ID)
		if err != nil {
			return err
		}
		for _, t := range teams {
			if !organization.HasTeamRepo(ctx, t.OrgID, t.ID, repoID) {
				continue
			} else if err = removeRepository(ctx, t, repo, false); err != nil {
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

	if _, err := db.Exec(ctx, "UPDATE `user` SET num_stars=num_stars-1 WHERE id IN (SELECT `uid` FROM `star` WHERE repo_id = ?)", repo.ID); err != nil {
		return err
	}

	if err := db.DeleteBeans(ctx,
		&access_model.Access{RepoID: repo.ID},
		&Action{RepoID: repo.ID},
		&repo_model.Collaboration{RepoID: repoID},
		&issues_model.Comment{RefRepoID: repoID},
		&git_model.CommitStatus{RepoID: repoID},
		&git_model.DeletedBranch{RepoID: repoID},
		&webhook.HookTask{RepoID: repoID},
		&git_model.LFSLock{RepoID: repoID},
		&repo_model.LanguageStat{RepoID: repoID},
		&issues_model.Milestone{RepoID: repoID},
		&repo_model.Mirror{RepoID: repoID},
		&Notification{RepoID: repoID},
		&git_model.ProtectedBranch{RepoID: repoID},
		&git_model.ProtectedTag{RepoID: repoID},
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
	if err := issues_model.DeleteLabelsByRepoID(ctx, repoID); err != nil {
		return err
	}

	// Delete Pulls and related objects
	if err := issues_model.DeletePullsByBaseRepoID(ctx, repoID); err != nil {
		return err
	}

	// Delete Issues and related objects
	var attachmentPaths []string
	if attachmentPaths, err = issues_model.DeleteIssuesByRepoID(ctx, repoID); err != nil {
		return err
	}

	// Delete issue index
	if err := db.DeleteResouceIndex(ctx, "issue_index", repoID); err != nil {
		return err
	}

	if repo.IsFork {
		if _, err := db.Exec(ctx, "UPDATE `repository` SET num_forks=num_forks-1 WHERE id=?", repo.ForkID); err != nil {
			return fmt.Errorf("decrease fork count: %v", err)
		}
	}

	if _, err := db.Exec(ctx, "UPDATE `user` SET num_repos=num_repos-1 WHERE id=?", uid); err != nil {
		return err
	}

	if len(repo.Topics) > 0 {
		if err := repo_model.RemoveTopicsFromRepo(ctx, repo.ID); err != nil {
			return err
		}
	}

	projects, _, err := project_model.GetProjects(ctx, project_model.SearchOptions{
		RepoID: repoID,
	})
	if err != nil {
		return fmt.Errorf("get projects: %v", err)
	}
	for i := range projects {
		if err := project_model.DeleteProjectByIDCtx(ctx, projects[i].ID); err != nil {
			return fmt.Errorf("delete project [%d]: %v", projects[i].ID, err)
		}
	}

	// Remove LFS objects
	var lfsObjects []*git_model.LFSMetaObject
	if err = sess.Where("repository_id=?", repoID).Find(&lfsObjects); err != nil {
		return err
	}

	lfsPaths := make([]string, 0, len(lfsObjects))
	for _, v := range lfsObjects {
		count, err := db.CountByBean(ctx, &git_model.LFSMetaObject{Pointer: lfs.Pointer{Oid: v.Oid}})
		if err != nil {
			return err
		}
		if count > 1 {
			continue
		}

		lfsPaths = append(lfsPaths, v.RelativePath())
	}

	if _, err := db.DeleteByBean(ctx, &git_model.LFSMetaObject{RepositoryID: repoID}); err != nil {
		return err
	}

	// Remove archives
	var archives []*repo_model.RepoArchiver
	if err = sess.Where("repo_id=?", repoID).Find(&archives); err != nil {
		return err
	}

	archivePaths := make([]string, 0, len(archives))
	for _, v := range archives {
		archivePaths = append(archivePaths, v.RelativePath())
	}

	if _, err := db.DeleteByBean(ctx, &repo_model.RepoArchiver{RepoID: repoID}); err != nil {
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

	newAttachmentPaths := make([]string, 0, len(newAttachments))
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
	for _, archive := range archivePaths {
		admin_model.RemoveStorageWithNotice(db.DefaultContext, storage.RepoArchives, "Delete repo archive file", archive)
	}

	// Remove lfs objects
	for _, lfsObj := range lfsPaths {
		admin_model.RemoveStorageWithNotice(db.DefaultContext, storage.LFS, "Delete orphaned LFS file", lfsObj)
	}

	// Remove issue attachment files.
	for _, attachment := range attachmentPaths {
		admin_model.RemoveStorageWithNotice(db.DefaultContext, storage.Attachments, "Delete issue attachment", attachment)
	}

	// Remove release attachment files.
	for _, releaseAttachment := range releaseAttachments {
		admin_model.RemoveStorageWithNotice(db.DefaultContext, storage.Attachments, "Delete release attachment", releaseAttachment)
	}

	// Remove attachment with no issue_id and release_id.
	for _, newAttachment := range newAttachmentPaths {
		admin_model.RemoveStorageWithNotice(db.DefaultContext, storage.Attachments, "Delete issue attachment", newAttachment)
	}

	if len(repo.Avatar) > 0 {
		if err := storage.RepoAvatars.Delete(repo.CustomAvatarRelativePath()); err != nil {
			return fmt.Errorf("Failed to remove %s: %v", repo.Avatar, err)
		}
	}

	return nil
}

type repoChecker struct {
	querySQL   func(ctx context.Context) ([]map[string][]byte, error)
	correctSQL func(ctx context.Context, id int64) error
	desc       string
}

func repoStatsCheck(ctx context.Context, checker *repoChecker) {
	results, err := checker.querySQL(ctx)
	if err != nil {
		log.Error("Select %s: %v", checker.desc, err)
		return
	}
	for _, result := range results {
		id, _ := strconv.ParseInt(string(result["id"]), 10, 64)
		select {
		case <-ctx.Done():
			log.Warn("CheckRepoStats: Cancelled before checking %s for with id=%d", checker.desc, id)
			return
		default:
		}
		log.Trace("Updating %s: %d", checker.desc, id)
		err = checker.correctSQL(ctx, id)
		if err != nil {
			log.Error("Update %s[%d]: %v", checker.desc, id, err)
		}
	}
}

func StatsCorrectSQL(ctx context.Context, sql string, id int64) error {
	_, err := db.GetEngine(ctx).Exec(sql, id, id)
	return err
}

func repoStatsCorrectNumWatches(ctx context.Context, id int64) error {
	return StatsCorrectSQL(ctx, "UPDATE `repository` SET num_watches=(SELECT COUNT(*) FROM `watch` WHERE repo_id=? AND mode<>2) WHERE id=?", id)
}

func repoStatsCorrectNumStars(ctx context.Context, id int64) error {
	return StatsCorrectSQL(ctx, "UPDATE `repository` SET num_stars=(SELECT COUNT(*) FROM `star` WHERE repo_id=?) WHERE id=?", id)
}

func labelStatsCorrectNumIssues(ctx context.Context, id int64) error {
	return StatsCorrectSQL(ctx, "UPDATE `label` SET num_issues=(SELECT COUNT(*) FROM `issue_label` WHERE label_id=?) WHERE id=?", id)
}

func labelStatsCorrectNumIssuesRepo(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `label` SET num_issues=(SELECT COUNT(*) FROM `issue_label` WHERE label_id=id) WHERE repo_id=?", id)
	return err
}

func labelStatsCorrectNumClosedIssues(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `label` SET num_closed_issues=(SELECT COUNT(*) FROM `issue_label`,`issue` WHERE `issue_label`.label_id=`label`.id AND `issue_label`.issue_id=`issue`.id AND `issue`.is_closed=?) WHERE `label`.id=?", true, id)
	return err
}

func labelStatsCorrectNumClosedIssuesRepo(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `label` SET num_closed_issues=(SELECT COUNT(*) FROM `issue_label`,`issue` WHERE `issue_label`.label_id=`label`.id AND `issue_label`.issue_id=`issue`.id AND `issue`.is_closed=?) WHERE `label`.repo_id=?", true, id)
	return err
}

var milestoneStatsQueryNumIssues = "SELECT `milestone`.id FROM `milestone` WHERE `milestone`.num_closed_issues!=(SELECT COUNT(*) FROM `issue` WHERE `issue`.milestone_id=`milestone`.id AND `issue`.is_closed=?) OR `milestone`.num_issues!=(SELECT COUNT(*) FROM `issue` WHERE `issue`.milestone_id=`milestone`.id)"

func milestoneStatsCorrectNumIssuesRepo(ctx context.Context, id int64) error {
	e := db.GetEngine(ctx)
	results, err := e.Query(milestoneStatsQueryNumIssues+" AND `milestone`.repo_id = ?", true, id)
	if err != nil {
		return err
	}
	for _, result := range results {
		id, _ := strconv.ParseInt(string(result["id"]), 10, 64)
		err = issues_model.UpdateMilestoneCounters(ctx, id)
		if err != nil {
			return err
		}
	}
	return nil
}

func userStatsCorrectNumRepos(ctx context.Context, id int64) error {
	return StatsCorrectSQL(ctx, "UPDATE `user` SET num_repos=(SELECT COUNT(*) FROM `repository` WHERE owner_id=?) WHERE id=?", id)
}

func repoStatsCorrectIssueNumComments(ctx context.Context, id int64) error {
	return StatsCorrectSQL(ctx, "UPDATE `issue` SET num_comments=(SELECT COUNT(*) FROM `comment` WHERE issue_id=? AND type=0) WHERE id=?", id)
}

func repoStatsCorrectNumIssues(ctx context.Context, id int64) error {
	return repoStatsCorrectNum(ctx, id, false, "num_issues")
}

func repoStatsCorrectNumPulls(ctx context.Context, id int64) error {
	return repoStatsCorrectNum(ctx, id, true, "num_pulls")
}

func repoStatsCorrectNum(ctx context.Context, id int64, isPull bool, field string) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `repository` SET "+field+"=(SELECT COUNT(*) FROM `issue` WHERE repo_id=? AND is_pull=?) WHERE id=?", id, isPull, id)
	return err
}

func repoStatsCorrectNumClosedIssues(ctx context.Context, id int64) error {
	return repo_model.StatsCorrectNumClosed(ctx, id, false, "num_closed_issues")
}

func repoStatsCorrectNumClosedPulls(ctx context.Context, id int64) error {
	return repo_model.StatsCorrectNumClosed(ctx, id, true, "num_closed_pulls")
}

func statsQuery(args ...interface{}) func(context.Context) ([]map[string][]byte, error) {
	return func(ctx context.Context) ([]map[string][]byte, error) {
		return db.GetEngine(ctx).Query(args...)
	}
}

// CheckRepoStats checks the repository stats
func CheckRepoStats(ctx context.Context) error {
	log.Trace("Doing: CheckRepoStats")

	checkers := []*repoChecker{
		// Repository.NumWatches
		{
			statsQuery("SELECT repo.id FROM `repository` repo WHERE repo.num_watches!=(SELECT COUNT(*) FROM `watch` WHERE repo_id=repo.id AND mode<>2)"),
			repoStatsCorrectNumWatches,
			"repository count 'num_watches'",
		},
		// Repository.NumStars
		{
			statsQuery("SELECT repo.id FROM `repository` repo WHERE repo.num_stars!=(SELECT COUNT(*) FROM `star` WHERE repo_id=repo.id)"),
			repoStatsCorrectNumStars,
			"repository count 'num_stars'",
		},
		// Repository.NumClosedIssues
		{
			statsQuery("SELECT repo.id FROM `repository` repo WHERE repo.num_closed_issues!=(SELECT COUNT(*) FROM `issue` WHERE repo_id=repo.id AND is_closed=? AND is_pull=?)", true, false),
			repoStatsCorrectNumClosedIssues,
			"repository count 'num_closed_issues'",
		},
		// Repository.NumClosedPulls
		{
			statsQuery("SELECT repo.id FROM `repository` repo WHERE repo.num_closed_issues!=(SELECT COUNT(*) FROM `issue` WHERE repo_id=repo.id AND is_closed=? AND is_pull=?)", true, true),
			repoStatsCorrectNumClosedPulls,
			"repository count 'num_closed_pulls'",
		},
		// Label.NumIssues
		{
			statsQuery("SELECT label.id FROM `label` WHERE label.num_issues!=(SELECT COUNT(*) FROM `issue_label` WHERE label_id=label.id)"),
			labelStatsCorrectNumIssues,
			"label count 'num_issues'",
		},
		// Label.NumClosedIssues
		{
			statsQuery("SELECT `label`.id FROM `label` WHERE `label`.num_closed_issues!=(SELECT COUNT(*) FROM `issue_label`,`issue` WHERE `issue_label`.label_id=`label`.id AND `issue_label`.issue_id=`issue`.id AND `issue`.is_closed=?)", true),
			labelStatsCorrectNumClosedIssues,
			"label count 'num_closed_issues'",
		},
		// Milestone.Num{,Closed}Issues
		{
			statsQuery(milestoneStatsQueryNumIssues, true),
			issues_model.UpdateMilestoneCounters,
			"milestone count 'num_closed_issues' and 'num_issues'",
		},
		// User.NumRepos
		{
			statsQuery("SELECT `user`.id FROM `user` WHERE `user`.num_repos!=(SELECT COUNT(*) FROM `repository` WHERE owner_id=`user`.id)"),
			userStatsCorrectNumRepos,
			"user count 'num_repos'",
		},
		// Issue.NumComments
		{
			statsQuery("SELECT `issue`.id FROM `issue` WHERE `issue`.num_comments!=(SELECT COUNT(*) FROM `comment` WHERE issue_id=`issue`.id AND type=0)"),
			repoStatsCorrectIssueNumComments,
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

	// FIXME: use checker when stop supporting old fork repo format.
	// ***** START: Repository.NumForks *****
	e := db.GetEngine(ctx)
	results, err := e.Query("SELECT repo.id FROM `repository` repo WHERE repo.num_forks!=(SELECT COUNT(*) FROM `repository` WHERE fork_id=repo.id)")
	if err != nil {
		log.Error("Select repository count 'num_forks': %v", err)
	} else {
		for _, result := range results {
			id, _ := strconv.ParseInt(string(result["id"]), 10, 64)
			select {
			case <-ctx.Done():
				log.Warn("CheckRepoStats: Cancelled")
				return db.ErrCancelledf("during repository count 'num_fork' for repo ID %d", id)
			default:
			}
			log.Trace("Updating repository count 'num_forks': %d", id)

			repo, err := repo_model.GetRepositoryByID(id)
			if err != nil {
				log.Error("repo_model.GetRepositoryByID[%d]: %v", id, err)
				continue
			}

			_, err = e.SQL("SELECT COUNT(*) FROM `repository` WHERE fork_id=?", repo.ID).Get(&repo.NumForks)
			if err != nil {
				log.Error("Select count of forks[%d]: %v", repo.ID, err)
				continue
			}

			if _, err = e.ID(repo.ID).Cols("num_forks").Update(repo); err != nil {
				log.Error("UpdateRepository[%d]: %v", id, err)
				continue
			}
		}
	}
	// ***** END: Repository.NumForks *****
	return nil
}

func UpdateRepoStats(ctx context.Context, id int64) error {
	var err error

	for _, f := range []func(ctx context.Context, id int64) error{
		repoStatsCorrectNumWatches,
		repoStatsCorrectNumStars,
		repoStatsCorrectNumIssues,
		repoStatsCorrectNumPulls,
		repoStatsCorrectNumClosedIssues,
		repoStatsCorrectNumClosedPulls,
		labelStatsCorrectNumIssuesRepo,
		labelStatsCorrectNumClosedIssuesRepo,
		milestoneStatsCorrectNumIssuesRepo,
	} {
		err = f(ctx, id)
		if err != nil {
			return err
		}
	}
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

// DeleteDeployKey delete deploy keys
func DeleteDeployKey(ctx context.Context, doer *user_model.User, id int64) error {
	key, err := asymkey_model.GetDeployKeyByID(ctx, id)
	if err != nil {
		if asymkey_model.IsErrDeployKeyNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetDeployKeyByID: %v", err)
	}

	// Check if user has access to delete this key.
	if !doer.IsAdmin {
		repo, err := repo_model.GetRepositoryByIDCtx(ctx, key.RepoID)
		if err != nil {
			return fmt.Errorf("GetRepositoryByID: %v", err)
		}
		has, err := access_model.IsUserRepoAdmin(ctx, repo, doer)
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

	if _, err := db.DeleteByBean(ctx, &asymkey_model.DeployKey{
		ID: key.ID,
	}); err != nil {
		return fmt.Errorf("delete deploy key [%d]: %v", key.ID, err)
	}

	// Check if this is the last reference to same key content.
	has, err := asymkey_model.IsDeployKeyExistByKeyID(ctx, key.KeyID)
	if err != nil {
		return err
	} else if !has {
		if err = asymkey_model.DeletePublicKeys(ctx, key.KeyID); err != nil {
			return err
		}
	}

	return nil
}
