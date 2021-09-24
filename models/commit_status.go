// Copyright 2017 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"crypto/sha1"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

// CommitStatus holds a single Status of a single Commit
type CommitStatus struct {
	ID          int64                 `xorm:"pk autoincr"`
	Index       int64                 `xorm:"INDEX UNIQUE(repo_sha_index)"`
	RepoID      int64                 `xorm:"INDEX UNIQUE(repo_sha_index)"`
	Repo        *Repository           `xorm:"-"`
	State       api.CommitStatusState `xorm:"VARCHAR(7) NOT NULL"`
	SHA         string                `xorm:"VARCHAR(64) NOT NULL INDEX UNIQUE(repo_sha_index)"`
	TargetURL   string                `xorm:"TEXT"`
	Description string                `xorm:"TEXT"`
	ContextHash string                `xorm:"char(40) index"`
	Context     string                `xorm:"TEXT"`
	Creator     *User                 `xorm:"-"`
	CreatorID   int64

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(CommitStatus))
	db.RegisterModel(new(CommitStatusIndex))
}

// upsertCommitStatusIndex the function will not return until it acquires the lock or receives an error.
func upsertCommitStatusIndex(e db.Engine, repoID int64, sha string) (err error) {
	// An atomic UPSERT operation (INSERT/UPDATE) is the only operation
	// that ensures that the key is actually locked.
	switch {
	case setting.Database.UseSQLite3 || setting.Database.UsePostgreSQL:
		_, err = e.Exec("INSERT INTO `commit_status_index` (repo_id, sha, max_index) "+
			"VALUES (?,?,1) ON CONFLICT (repo_id,sha) DO UPDATE SET max_index = `commit_status_index`.max_index+1",
			repoID, sha)
	case setting.Database.UseMySQL:
		_, err = e.Exec("INSERT INTO `commit_status_index` (repo_id, sha, max_index) "+
			"VALUES (?,?,1) ON DUPLICATE KEY UPDATE max_index = max_index+1",
			repoID, sha)
	case setting.Database.UseMSSQL:
		// https://weblogs.sqlteam.com/dang/2009/01/31/upsert-race-condition-with-merge/
		_, err = e.Exec("MERGE `commit_status_index` WITH (HOLDLOCK) as target "+
			"USING (SELECT ? AS repo_id, ? AS sha) AS src "+
			"ON src.repo_id = target.repo_id AND src.sha = target.sha "+
			"WHEN MATCHED THEN UPDATE SET target.max_index = target.max_index+1 "+
			"WHEN NOT MATCHED THEN INSERT (repo_id, sha, max_index) "+
			"VALUES (src.repo_id, src.sha, 1);",
			repoID, sha)
	default:
		return fmt.Errorf("database type not supported")
	}
	return
}

// GetNextCommitStatusIndex retried 3 times to generate a resource index
func GetNextCommitStatusIndex(repoID int64, sha string) (int64, error) {
	for i := 0; i < db.MaxDupIndexAttempts; i++ {
		idx, err := getNextCommitStatusIndex(repoID, sha)
		if err == db.ErrResouceOutdated {
			continue
		}
		if err != nil {
			return 0, err
		}
		return idx, nil
	}
	return 0, db.ErrGetResourceIndexFailed
}

// getNextCommitStatusIndex return the next index
func getNextCommitStatusIndex(repoID int64, sha string) (int64, error) {
	ctx, commiter, err := db.TxContext()
	if err != nil {
		return 0, err
	}
	defer commiter.Close()

	var preIdx int64
	_, err = ctx.Engine().SQL("SELECT max_index FROM `commit_status_index` WHERE repo_id = ? AND sha = ?", repoID, sha).Get(&preIdx)
	if err != nil {
		return 0, err
	}

	if err := upsertCommitStatusIndex(ctx.Engine(), repoID, sha); err != nil {
		return 0, err
	}

	var curIdx int64
	has, err := ctx.Engine().SQL("SELECT max_index FROM `commit_status_index` WHERE repo_id = ? AND sha = ? AND max_index=?", repoID, sha, preIdx+1).Get(&curIdx)
	if err != nil {
		return 0, err
	}
	if !has {
		return 0, db.ErrResouceOutdated
	}
	if err := commiter.Commit(); err != nil {
		return 0, err
	}
	return curIdx, nil
}

func (status *CommitStatus) loadAttributes(e db.Engine) (err error) {
	if status.Repo == nil {
		status.Repo, err = getRepositoryByID(e, status.RepoID)
		if err != nil {
			return fmt.Errorf("getRepositoryByID [%d]: %v", status.RepoID, err)
		}
	}
	if status.Creator == nil && status.CreatorID > 0 {
		status.Creator, err = getUserByID(e, status.CreatorID)
		if err != nil {
			return fmt.Errorf("getUserByID [%d]: %v", status.CreatorID, err)
		}
	}
	return nil
}

// APIURL returns the absolute APIURL to this commit-status.
func (status *CommitStatus) APIURL() string {
	_ = status.loadAttributes(db.GetEngine(db.DefaultContext))
	return fmt.Sprintf("%sapi/v1/repos/%s/statuses/%s",
		setting.AppURL, status.Repo.FullName(), status.SHA)
}

// CalcCommitStatus returns commit status state via some status, the commit statues should order by id desc
func CalcCommitStatus(statuses []*CommitStatus) *CommitStatus {
	var lastStatus *CommitStatus
	var state api.CommitStatusState
	for _, status := range statuses {
		if status.State.NoBetterThan(state) {
			state = status.State
			lastStatus = status
		}
	}
	if lastStatus == nil {
		if len(statuses) > 0 {
			lastStatus = statuses[0]
		} else {
			lastStatus = &CommitStatus{}
		}
	}
	return lastStatus
}

// CommitStatusOptions holds the options for query commit statuses
type CommitStatusOptions struct {
	db.ListOptions
	State    string
	SortType string
}

// GetCommitStatuses returns all statuses for a given commit.
func GetCommitStatuses(repo *Repository, sha string, opts *CommitStatusOptions) ([]*CommitStatus, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.Page = ItemsPerPage
	}

	countSession := listCommitStatusesStatement(repo, sha, opts)
	countSession = db.SetSessionPagination(countSession, opts)
	maxResults, err := countSession.Count(new(CommitStatus))
	if err != nil {
		log.Error("Count PRs: %v", err)
		return nil, maxResults, err
	}

	statuses := make([]*CommitStatus, 0, opts.PageSize)
	findSession := listCommitStatusesStatement(repo, sha, opts)
	findSession = db.SetSessionPagination(findSession, opts)
	sortCommitStatusesSession(findSession, opts.SortType)
	return statuses, maxResults, findSession.Find(&statuses)
}

func listCommitStatusesStatement(repo *Repository, sha string, opts *CommitStatusOptions) *xorm.Session {
	sess := db.GetEngine(db.DefaultContext).Where("repo_id = ?", repo.ID).And("sha = ?", sha)
	switch opts.State {
	case "pending", "success", "error", "failure", "warning":
		sess.And("state = ?", opts.State)
	}
	return sess
}

func sortCommitStatusesSession(sess *xorm.Session, sortType string) {
	switch sortType {
	case "oldest":
		sess.Asc("created_unix")
	case "recentupdate":
		sess.Desc("updated_unix")
	case "leastupdate":
		sess.Asc("updated_unix")
	case "leastindex":
		sess.Desc("index")
	case "highestindex":
		sess.Asc("index")
	default:
		sess.Desc("created_unix")
	}
}

// CommitStatusIndex represents a table for commit status index
type CommitStatusIndex struct {
	ID       int64
	RepoID   int64  `xorm:"unique(repo_sha)"`
	SHA      string `xorm:"unique(repo_sha)"`
	MaxIndex int64  `xorm:"index"`
}

// GetLatestCommitStatus returns all statuses with a unique context for a given commit.
func GetLatestCommitStatus(repoID int64, sha string, listOptions db.ListOptions) ([]*CommitStatus, error) {
	return getLatestCommitStatus(db.GetEngine(db.DefaultContext), repoID, sha, listOptions)
}

func getLatestCommitStatus(e db.Engine, repoID int64, sha string, listOptions db.ListOptions) ([]*CommitStatus, error) {
	ids := make([]int64, 0, 10)
	sess := e.Table(&CommitStatus{}).
		Where("repo_id = ?", repoID).And("sha = ?", sha).
		Select("max( id ) as id").
		GroupBy("context_hash").OrderBy("max( id ) desc")

	sess = db.SetSessionPagination(sess, &listOptions)

	err := sess.Find(&ids)
	if err != nil {
		return nil, err
	}
	statuses := make([]*CommitStatus, 0, len(ids))
	if len(ids) == 0 {
		return statuses, nil
	}
	return statuses, e.In("id", ids).Find(&statuses)
}

// FindRepoRecentCommitStatusContexts returns repository's recent commit status contexts
func FindRepoRecentCommitStatusContexts(repoID int64, before time.Duration) ([]string, error) {
	start := timeutil.TimeStampNow().AddDuration(-before)
	ids := make([]int64, 0, 10)
	if err := db.GetEngine(db.DefaultContext).Table("commit_status").
		Where("repo_id = ?", repoID).
		And("updated_unix >= ?", start).
		Select("max( id ) as id").
		GroupBy("context_hash").OrderBy("max( id ) desc").
		Find(&ids); err != nil {
		return nil, err
	}

	contexts := make([]string, 0, len(ids))
	if len(ids) == 0 {
		return contexts, nil
	}
	return contexts, db.GetEngine(db.DefaultContext).Select("context").Table("commit_status").In("id", ids).Find(&contexts)
}

// NewCommitStatusOptions holds options for creating a CommitStatus
type NewCommitStatusOptions struct {
	Repo         *Repository
	Creator      *User
	SHA          string
	CommitStatus *CommitStatus
}

// NewCommitStatus save commit statuses into database
func NewCommitStatus(opts NewCommitStatusOptions) error {
	if opts.Repo == nil {
		return fmt.Errorf("NewCommitStatus[nil, %s]: no repository specified", opts.SHA)
	}

	repoPath := opts.Repo.RepoPath()
	if opts.Creator == nil {
		return fmt.Errorf("NewCommitStatus[%s, %s]: no user specified", repoPath, opts.SHA)
	}

	// Get the next Status Index
	idx, err := GetNextCommitStatusIndex(opts.Repo.ID, opts.SHA)
	if err != nil {
		return fmt.Errorf("generate commit status index failed: %v", err)
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return fmt.Errorf("NewCommitStatus[repo_id: %d, user_id: %d, sha: %s]: %v", opts.Repo.ID, opts.Creator.ID, opts.SHA, err)
	}
	defer committer.Close()

	opts.CommitStatus.Description = strings.TrimSpace(opts.CommitStatus.Description)
	opts.CommitStatus.Context = strings.TrimSpace(opts.CommitStatus.Context)
	opts.CommitStatus.TargetURL = strings.TrimSpace(opts.CommitStatus.TargetURL)
	opts.CommitStatus.SHA = opts.SHA
	opts.CommitStatus.CreatorID = opts.Creator.ID
	opts.CommitStatus.RepoID = opts.Repo.ID
	opts.CommitStatus.Index = idx
	log.Debug("NewCommitStatus[%s, %s]: %d", repoPath, opts.SHA, opts.CommitStatus.Index)

	opts.CommitStatus.ContextHash = hashCommitStatusContext(opts.CommitStatus.Context)

	// Insert new CommitStatus
	if _, err = db.GetEngine(ctx).Insert(opts.CommitStatus); err != nil {
		return fmt.Errorf("Insert CommitStatus[%s, %s]: %v", repoPath, opts.SHA, err)
	}

	return committer.Commit()
}

// SignCommitWithStatuses represents a commit with validation of signature and status state.
type SignCommitWithStatuses struct {
	Status   *CommitStatus
	Statuses []*CommitStatus
	*SignCommit
}

// ParseCommitsWithStatus checks commits latest statuses and calculates its worst status state
func ParseCommitsWithStatus(oldCommits []*SignCommit, repo *Repository) []*SignCommitWithStatuses {
	newCommits := make([]*SignCommitWithStatuses, 0, len(oldCommits))

	for _, c := range oldCommits {
		commit := &SignCommitWithStatuses{
			SignCommit: c,
		}
		statuses, err := GetLatestCommitStatus(repo.ID, commit.ID.String(), db.ListOptions{})
		if err != nil {
			log.Error("GetLatestCommitStatus: %v", err)
		} else {
			commit.Statuses = statuses
			commit.Status = CalcCommitStatus(statuses)
		}

		newCommits = append(newCommits, commit)
	}
	return newCommits
}

// hashCommitStatusContext hash context
func hashCommitStatusContext(context string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(context)))
}
