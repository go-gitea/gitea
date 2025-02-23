// Copyright 2017 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/translation"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// CommitStatus holds a single Status of a single Commit
type CommitStatus struct {
	ID          int64                  `xorm:"pk autoincr"`
	Index       int64                  `xorm:"INDEX UNIQUE(repo_sha_index)"`
	RepoID      int64                  `xorm:"INDEX UNIQUE(repo_sha_index)"`
	Repo        *repo_model.Repository `xorm:"-"`
	State       api.CommitStatusState  `xorm:"VARCHAR(7) NOT NULL"`
	SHA         string                 `xorm:"VARCHAR(64) NOT NULL INDEX UNIQUE(repo_sha_index)"`
	TargetURL   string                 `xorm:"TEXT"`
	Description string                 `xorm:"TEXT"`
	ContextHash string                 `xorm:"VARCHAR(64) index"`
	Context     string                 `xorm:"TEXT"`
	Creator     *user_model.User       `xorm:"-"`
	CreatorID   int64

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(CommitStatus))
	db.RegisterModel(new(CommitStatusIndex))
}

func postgresGetCommitStatusIndex(ctx context.Context, repoID int64, sha string) (int64, error) {
	res, err := db.GetEngine(ctx).Query("INSERT INTO `commit_status_index` (repo_id, sha, max_index) "+
		"VALUES (?,?,1) ON CONFLICT (repo_id, sha) DO UPDATE SET max_index = `commit_status_index`.max_index+1 RETURNING max_index",
		repoID, sha)
	if err != nil {
		return 0, err
	}
	if len(res) == 0 {
		return 0, db.ErrGetResourceIndexFailed
	}
	return strconv.ParseInt(string(res[0]["max_index"]), 10, 64)
}

func mysqlGetCommitStatusIndex(ctx context.Context, repoID int64, sha string) (int64, error) {
	if _, err := db.GetEngine(ctx).Exec("INSERT INTO `commit_status_index` (repo_id, sha, max_index) "+
		"VALUES (?,?,1) ON DUPLICATE KEY UPDATE max_index = max_index+1",
		repoID, sha); err != nil {
		return 0, err
	}

	var idx int64
	_, err := db.GetEngine(ctx).SQL("SELECT max_index FROM `commit_status_index` WHERE repo_id = ? AND sha = ?",
		repoID, sha).Get(&idx)
	if err != nil {
		return 0, err
	}
	if idx == 0 {
		return 0, errors.New("cannot get the correct index")
	}
	return idx, nil
}

func mssqlGetCommitStatusIndex(ctx context.Context, repoID int64, sha string) (int64, error) {
	if _, err := db.GetEngine(ctx).Exec(`
MERGE INTO commit_status_index WITH (HOLDLOCK) AS target
USING (SELECT ? AS repo_id, ? AS sha) AS source
(repo_id, sha)
ON target.repo_id = source.repo_id AND target.sha = source.sha
WHEN MATCHED
	THEN UPDATE
			SET max_index = max_index + 1
WHEN NOT MATCHED
	THEN INSERT (repo_id, sha, max_index)
			VALUES (?, ?, 1);
`, repoID, sha, repoID, sha); err != nil {
		return 0, err
	}

	var idx int64
	_, err := db.GetEngine(ctx).SQL("SELECT max_index FROM `commit_status_index` WHERE repo_id = ? AND sha = ?",
		repoID, sha).Get(&idx)
	if err != nil {
		return 0, err
	}
	if idx == 0 {
		return 0, errors.New("cannot get the correct index")
	}
	return idx, nil
}

// GetNextCommitStatusIndex retried 3 times to generate a resource index
func GetNextCommitStatusIndex(ctx context.Context, repoID int64, sha string) (int64, error) {
	_, err := git.NewIDFromString(sha)
	if err != nil {
		return 0, git.ErrInvalidSHA{SHA: sha}
	}

	switch {
	case setting.Database.Type.IsPostgreSQL():
		return postgresGetCommitStatusIndex(ctx, repoID, sha)
	case setting.Database.Type.IsMySQL():
		return mysqlGetCommitStatusIndex(ctx, repoID, sha)
	case setting.Database.Type.IsMSSQL():
		return mssqlGetCommitStatusIndex(ctx, repoID, sha)
	}

	e := db.GetEngine(ctx)

	// try to update the max_index to next value, and acquire the write-lock for the record
	res, err := e.Exec("UPDATE `commit_status_index` SET max_index=max_index+1 WHERE repo_id=? AND sha=?", repoID, sha)
	if err != nil {
		return 0, fmt.Errorf("update failed: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if affected == 0 {
		// this slow path is only for the first time of creating a resource index
		_, errIns := e.Exec("INSERT INTO `commit_status_index` (repo_id, sha, max_index) VALUES (?, ?, 0)", repoID, sha)
		res, err = e.Exec("UPDATE `commit_status_index` SET max_index=max_index+1 WHERE repo_id=? AND sha=?", repoID, sha)
		if err != nil {
			return 0, fmt.Errorf("update2 failed: %w", err)
		}
		affected, err = res.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("RowsAffected failed: %w", err)
		}
		// if the update still can not update any records, the record must not exist and there must be some errors (insert error)
		if affected == 0 {
			if errIns == nil {
				return 0, errors.New("impossible error when GetNextCommitStatusIndex, insert and update both succeeded but no record is updated")
			}
			return 0, fmt.Errorf("insert failed: %w", errIns)
		}
	}

	// now, the new index is in database (protected by the transaction and write-lock)
	var newIdx int64
	has, err := e.SQL("SELECT max_index FROM `commit_status_index` WHERE repo_id=? AND sha=?", repoID, sha).Get(&newIdx)
	if err != nil {
		return 0, fmt.Errorf("select failed: %w", err)
	}
	if !has {
		return 0, errors.New("impossible error when GetNextCommitStatusIndex, upsert succeeded but no record can be selected")
	}
	return newIdx, nil
}

func (status *CommitStatus) loadRepository(ctx context.Context) (err error) {
	if status.Repo == nil {
		status.Repo, err = repo_model.GetRepositoryByID(ctx, status.RepoID)
		if err != nil {
			return fmt.Errorf("getRepositoryByID [%d]: %w", status.RepoID, err)
		}
	}
	return nil
}

func (status *CommitStatus) loadCreator(ctx context.Context) (err error) {
	if status.Creator == nil && status.CreatorID > 0 {
		status.Creator, err = user_model.GetUserByID(ctx, status.CreatorID)
		if err != nil {
			return fmt.Errorf("getUserByID [%d]: %w", status.CreatorID, err)
		}
	}
	return nil
}

func (status *CommitStatus) loadAttributes(ctx context.Context) (err error) {
	if err := status.loadRepository(ctx); err != nil {
		return err
	}
	return status.loadCreator(ctx)
}

// APIURL returns the absolute APIURL to this commit-status.
func (status *CommitStatus) APIURL(ctx context.Context) string {
	_ = status.loadAttributes(ctx)
	return status.Repo.APIURL() + "/statuses/" + url.PathEscape(status.SHA)
}

// LocaleString returns the locale string name of the Status
func (status *CommitStatus) LocaleString(lang translation.Locale) string {
	return lang.TrString("repo.commitstatus." + status.State.String())
}

// HideActionsURL set `TargetURL` to an empty string if the status comes from Gitea Actions
func (status *CommitStatus) HideActionsURL(ctx context.Context) {
	if status.RepoID == 0 {
		return
	}

	if status.Repo == nil {
		if err := status.loadRepository(ctx); err != nil {
			log.Error("loadRepository: %v", err)
			return
		}
	}

	prefix := fmt.Sprintf("%s/actions", status.Repo.Link())
	if strings.HasPrefix(status.TargetURL, prefix) {
		status.TargetURL = ""
	}
}

// CalcCommitStatus returns commit status state via some status, the commit statues should order by id desc
func CalcCommitStatus(statuses []*CommitStatus) *CommitStatus {
	var lastStatus *CommitStatus
	state := api.CommitStatusSuccess
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
	RepoID   int64
	SHA      string
	State    string
	SortType string
}

func (opts *CommitStatusOptions) ToConds() builder.Cond {
	var cond builder.Cond = builder.Eq{
		"repo_id": opts.RepoID,
		"sha":     opts.SHA,
	}

	switch opts.State {
	case "pending", "success", "error", "failure", "warning":
		cond = cond.And(builder.Eq{
			"state": opts.State,
		})
	}

	return cond
}

func (opts *CommitStatusOptions) ToOrders() string {
	switch opts.SortType {
	case "oldest":
		return "created_unix ASC"
	case "recentupdate":
		return "updated_unix DESC"
	case "leastupdate":
		return "updated_unix ASC"
	case "leastindex":
		return "`index` DESC"
	case "highestindex":
		return "`index` ASC"
	default:
		return "created_unix DESC"
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
func GetLatestCommitStatus(ctx context.Context, repoID int64, sha string, listOptions db.ListOptions) ([]*CommitStatus, int64, error) {
	getBase := func() *xorm.Session {
		return db.GetEngine(ctx).Table(&CommitStatus{}).
			Where("repo_id = ?", repoID).And("sha = ?", sha)
	}
	indices := make([]int64, 0, 10)
	sess := getBase().Select("max( `index` ) as `index`").
		GroupBy("context_hash").OrderBy("max( `index` ) desc")
	if !listOptions.IsListAll() {
		sess = db.SetSessionPagination(sess, &listOptions)
	}
	count, err := sess.FindAndCount(&indices)
	if err != nil {
		return nil, count, err
	}
	statuses := make([]*CommitStatus, 0, len(indices))
	if len(indices) == 0 {
		return statuses, count, nil
	}
	return statuses, count, getBase().And(builder.In("`index`", indices)).Find(&statuses)
}

// GetLatestCommitStatusForPairs returns all statuses with a unique context for a given list of repo-sha pairs
func GetLatestCommitStatusForPairs(ctx context.Context, repoSHAs []RepoSHA) (map[int64][]*CommitStatus, error) {
	type result struct {
		Index  int64
		RepoID int64
		SHA    string
	}

	results := make([]result, 0, len(repoSHAs))

	getBase := func() *xorm.Session {
		return db.GetEngine(ctx).Table(&CommitStatus{})
	}

	// Create a disjunction of conditions for each repoID and SHA pair
	conds := make([]builder.Cond, 0, len(repoSHAs))
	for _, repoSHA := range repoSHAs {
		conds = append(conds, builder.Eq{"repo_id": repoSHA.RepoID, "sha": repoSHA.SHA})
	}
	sess := getBase().Where(builder.Or(conds...)).
		Select("max( `index` ) as `index`, repo_id, sha").
		GroupBy("context_hash, repo_id, sha").OrderBy("max( `index` ) desc")

	err := sess.Find(&results)
	if err != nil {
		return nil, err
	}

	repoStatuses := make(map[int64][]*CommitStatus)

	if len(results) > 0 {
		statuses := make([]*CommitStatus, 0, len(results))

		conds = make([]builder.Cond, 0, len(results))
		for _, result := range results {
			cond := builder.Eq{
				"`index`": result.Index,
				"repo_id": result.RepoID,
				"sha":     result.SHA,
			}
			conds = append(conds, cond)
		}
		err = getBase().Where(builder.Or(conds...)).Find(&statuses)
		if err != nil {
			return nil, err
		}

		// Group the statuses by repo ID
		for _, status := range statuses {
			repoStatuses[status.RepoID] = append(repoStatuses[status.RepoID], status)
		}
	}

	return repoStatuses, nil
}

// GetLatestCommitStatusForRepoCommitIDs returns all statuses with a unique context for a given list of repo-sha pairs
func GetLatestCommitStatusForRepoCommitIDs(ctx context.Context, repoID int64, commitIDs []string) (map[string][]*CommitStatus, error) {
	type result struct {
		Index int64
		SHA   string
	}

	getBase := func() *xorm.Session {
		return db.GetEngine(ctx).Table(&CommitStatus{}).Where("repo_id = ?", repoID)
	}
	results := make([]result, 0, len(commitIDs))

	conds := make([]builder.Cond, 0, len(commitIDs))
	for _, sha := range commitIDs {
		conds = append(conds, builder.Eq{"sha": sha})
	}
	sess := getBase().And(builder.Or(conds...)).
		Select("max( `index` ) as `index`, sha").
		GroupBy("context_hash, sha").OrderBy("max( `index` ) desc")

	err := sess.Find(&results)
	if err != nil {
		return nil, err
	}

	repoStatuses := make(map[string][]*CommitStatus)

	if len(results) > 0 {
		statuses := make([]*CommitStatus, 0, len(results))

		conds = make([]builder.Cond, 0, len(results))
		for _, result := range results {
			conds = append(conds, builder.Eq{"`index`": result.Index, "sha": result.SHA})
		}
		err = getBase().And(builder.Or(conds...)).Find(&statuses)
		if err != nil {
			return nil, err
		}

		// Group the statuses by commit
		for _, status := range statuses {
			repoStatuses[status.SHA] = append(repoStatuses[status.SHA], status)
		}
	}

	return repoStatuses, nil
}

// FindRepoRecentCommitStatusContexts returns repository's recent commit status contexts
func FindRepoRecentCommitStatusContexts(ctx context.Context, repoID int64, before time.Duration) ([]string, error) {
	start := timeutil.TimeStampNow().AddDuration(-before)

	var contexts []string
	if err := db.GetEngine(ctx).Table("commit_status").
		Where("repo_id = ?", repoID).And("updated_unix >= ?", start).
		Cols("context").Distinct().Find(&contexts); err != nil {
		return nil, err
	}

	return contexts, nil
}

// NewCommitStatusOptions holds options for creating a CommitStatus
type NewCommitStatusOptions struct {
	Repo         *repo_model.Repository
	Creator      *user_model.User
	SHA          git.ObjectID
	CommitStatus *CommitStatus
}

// NewCommitStatus save commit statuses into database
func NewCommitStatus(ctx context.Context, opts NewCommitStatusOptions) error {
	if opts.Repo == nil {
		return fmt.Errorf("NewCommitStatus[nil, %s]: no repository specified", opts.SHA)
	}

	repoPath := opts.Repo.RepoPath()
	if opts.Creator == nil {
		return fmt.Errorf("NewCommitStatus[%s, %s]: no user specified", repoPath, opts.SHA)
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return fmt.Errorf("NewCommitStatus[repo_id: %d, user_id: %d, sha: %s]: %w", opts.Repo.ID, opts.Creator.ID, opts.SHA, err)
	}
	defer committer.Close()

	// Get the next Status Index
	idx, err := GetNextCommitStatusIndex(ctx, opts.Repo.ID, opts.SHA.String())
	if err != nil {
		return fmt.Errorf("generate commit status index failed: %w", err)
	}

	opts.CommitStatus.Description = strings.TrimSpace(opts.CommitStatus.Description)
	opts.CommitStatus.Context = strings.TrimSpace(opts.CommitStatus.Context)
	opts.CommitStatus.TargetURL = strings.TrimSpace(opts.CommitStatus.TargetURL)
	opts.CommitStatus.SHA = opts.SHA.String()
	opts.CommitStatus.CreatorID = opts.Creator.ID
	opts.CommitStatus.RepoID = opts.Repo.ID
	opts.CommitStatus.Index = idx
	log.Debug("NewCommitStatus[%s, %s]: %d", repoPath, opts.SHA, opts.CommitStatus.Index)

	opts.CommitStatus.ContextHash = hashCommitStatusContext(opts.CommitStatus.Context)

	// Insert new CommitStatus
	if _, err = db.GetEngine(ctx).Insert(opts.CommitStatus); err != nil {
		return fmt.Errorf("insert CommitStatus[%s, %s]: %w", repoPath, opts.SHA, err)
	}

	return committer.Commit()
}

// SignCommitWithStatuses represents a commit with validation of signature and status state.
type SignCommitWithStatuses struct {
	Status   *CommitStatus
	Statuses []*CommitStatus
	*asymkey_model.SignCommit
}

// hashCommitStatusContext hash context
func hashCommitStatusContext(context string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(context)))
}

// CommitStatusesHideActionsURL hide Gitea Actions urls
func CommitStatusesHideActionsURL(ctx context.Context, statuses []*CommitStatus) {
	idToRepos := make(map[int64]*repo_model.Repository)
	for _, status := range statuses {
		if status == nil {
			continue
		}

		if status.Repo == nil {
			status.Repo = idToRepos[status.RepoID]
		}
		status.HideActionsURL(ctx)
		idToRepos[status.RepoID] = status.Repo
	}
}
