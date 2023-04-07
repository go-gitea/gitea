// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"
	"xorm.io/xorm"
)

type CheckRunStatus int64

const (
	// CheckRunStatusQueued queued
	CheckRunStatusQueued CheckRunStatus = iota
	// CheckRunStatusInProgress in_progress
	CheckRunStatusInProgress
	// CheckRunStatusCompleted completed
	CheckRunStatusCompleted
)

type CheckRunConclusion int64

const (
	// CheckRunConclusionActionRequired unknow status
	CheckRunConclusionUnknow CheckRunConclusion = iota
	// CheckRunConclusionActionRequired action_required
	CheckRunConclusionActionRequired
	// CheckRunConclusionCancelled cancelled
	CheckRunConclusionCancelled
	// CheckRunConclusionFailure failure
	CheckRunConclusionFailure
	// CheckRunConclusionNeutral neutral
	CheckRunConclusionNeutral
	// CheckRunConclusionNeutral success
	CheckRunConclusionSuccess
	// CheckRunConclusionSkipped skipped
	CheckRunConclusionSkipped
	// CheckRunConclusionStale stale
	CheckRunConclusionStale
	// CheckRunConclusionTimedOut timed_out
	CheckRunConclusionTimedOut
)

func (c CheckRunConclusion) toCommitStatusState() api.CommitStatusState {
	if c == CheckRunConclusionFailure {
		return api.CommitStatusFailure
	}

	if c == CheckRunConclusionNeutral {
		return api.CommitStatusNeutral
	}

	if c == CheckRunConclusionSuccess {
		return api.CommitStatusSuccess
	}

	if c == CheckRunConclusionSkipped {
		return api.CommitStatusSkipped
	}

	if c == CheckRunConclusionTimedOut {
		return api.CommitStatusTimedOut
	}

	return api.CommitStatusError
}

// CommitStatus holds a single Status of a single Commit
type CheckRun struct {
	ID         int64                  `xorm:"pk autoincr"`
	RepoID     int64                  `xorm:"INDEX UNIQUE(repo_sha_name)"`
	Repo       *repo_model.Repository `xorm:"-"`
	Status     CheckRunStatus
	Conclusion CheckRunConclusion
	HeadSHA    string           `xorm:"VARCHAR(64) NOT NULL INDEX UNIQUE(repo_sha_name)"`
	DetailsURL string           `xorm:"TEXT"`
	ExternalID string           `xorm:"TEXT"`
	Summary    string           `xorm:"TEXT"` // output -> Title/Summary
	NameHash   string           `xorm:"char(40) INDEX UNIQUE(repo_sha_name)"`
	Name       string           `xorm:"TEXT"`
	Creator    *user_model.User `xorm:"-"`
	CreatorID  int64

	StartedAt   timeutil.TimeStamp
	CompletedAt timeutil.TimeStamp

	Output *CheckRunOutput `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func (c *CheckRun) ToStatus(lang translation.Locale) *CommitStatus {
	stat := &CommitStatus{
		ID:          c.ID,
		Index:       -1,
		RepoID:      c.RepoID,
		Repo:        c.Repo,
		SHA:         c.HeadSHA,
		ContextHash: c.NameHash,
		Context:     c.Name,
		Creator:     c.Creator,
		CreatorID:   c.CreatorID,
		Description: c.Summary,
		TargetURL:   c.DetailsURL,
	}

	if c.Status == CheckRunStatusCompleted {
		stat.State = c.Conclusion.toCommitStatusState()
	} else if c.Status == CheckRunStatusInProgress {
		stat.State = api.CommitStatusRunning
	} else {
		stat.State = api.CommitStatusPending
	}

	if len(stat.Description) > 0 || lang == nil {
		return stat
	}

	if c.Status == CheckRunStatusCompleted {
		stat.Description = lang.Tr("check_runs.conclusion."+string(c.Conclusion.ToAPI()), util.SecToTime(int64(c.CompletedAt-c.StartedAt)))
	} else if c.Status == CheckRunStatusInProgress {
		stat.Description = lang.Tr("check_runs.status.runing", util.SecToTime(int64(timeutil.TimeStampNow()-c.StartedAt)))
	} else {
		stat.Description = lang.Tr("check_runs.status.pending")
	}

	return stat
}

type CheckRunOutput struct {
	ID             int64  `xorm:"pk autoincr"`
	ChekRunID      int64  `xorm:"INDEX"`
	Title          string `xorm:"TEXT"`
	Summary        string `xorm:"TEXT"`
	Text           string `xorm:"TEXT"`
	AnnotationsURL string `xorm:"TEXT"`
	Annotations    []api.CheckRunAnnotation
}

func init() {
	db.RegisterModel(new(CheckRun))
	db.RegisterModel(new(CheckRunOutput))
}

// NewCheckRunOptions holds options for creating a new CheckRun
type NewCheckRunOptions struct {
	Repo        *repo_model.Repository
	Creator     *user_model.User
	HeadSHA     string
	Name        string
	Status      api.CheckRunStatus
	Conclusion  api.CheckRunConclusion
	DetailsURL  string
	ExternalID  string
	StartedAt   timeutil.TimeStamp
	CompletedAt timeutil.TimeStamp
	Output      *api.CheckRunOutput
}

type ErrUnVaildCheckRunOptions struct {
	Err string
}

func (e ErrUnVaildCheckRunOptions) Error() string {
	return "unvaild check run options: " + e.Err
}

func IsErrUnVaildCheckRunOptions(err error) bool {
	_, ok := err.(ErrUnVaildCheckRunOptions)
	return ok
}

func (opts *NewCheckRunOptions) Vaild() error {
	if opts.Repo == nil || opts.Creator == nil {
		return errors.New("`repo` or `creator` not set")
	}

	if len(opts.Name) == 0 {
		return ErrUnVaildCheckRunOptions{Err: "request `name`"}
	}

	if len(opts.HeadSHA) == 0 {
		return ErrUnVaildCheckRunOptions{Err: "request `head_sha`"}
	}

	if len(opts.Status) > 0 && opts.Status != api.CheckRunStatusQueued && opts.StartedAt == timeutil.TimeStamp(0) {
		return ErrUnVaildCheckRunOptions{Err: "request `started_at` if staus isn't `queued`"}
	}

	if opts.Status != api.CheckRunStatusCompleted {
		return nil
	}

	if opts.Conclusion == "" {
		return ErrUnVaildCheckRunOptions{Err: "request `conclusion` if staus is `completed`"}
	}

	if opts.CompletedAt == timeutil.TimeStamp(0) {
		opts.CompletedAt = timeutil.TimeStampNow()
	}

	return nil
}

func isCheckRunExist(ctx context.Context, repoID int64, headSHA, nameHash string) (bool, error) {
	return db.GetEngine(ctx).Cols("id").Exist(&CheckRun{
		RepoID:   repoID,
		HeadSHA:  headSHA,
		NameHash: nameHash,
	})
}

type ErrCheckRunExist struct {
	RepoID  int64
	HeadSHA string
	Name    string
}

func (e ErrCheckRunExist) Error() string {
	return fmt.Sprintf("check run with  name already created [repo_id: %d, head_sha: %s, name: %s]", e.RepoID, e.HeadSHA, e.Name)
}

func IsErrCheckRunExist(err error) bool {
	_, ok := err.(ErrCheckRunExist)

	return ok
}

func CreateCheckRun(ctx context.Context, opts *NewCheckRunOptions) (*CheckRun, error) {
	err := opts.Vaild()
	if err != nil {
		return nil, err
	}

	repoPath := opts.Repo.RepoPath()

	if _, err := git.NewIDFromString(opts.HeadSHA); err != nil {
		return nil, fmt.Errorf("CreateCheckRun[%s, %s]: invalid head_sha: %w", repoPath, opts.HeadSHA, err)
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	nameHash := hashCommitStatusContext(opts.Name)
	exist, err := isCheckRunExist(ctx, opts.Repo.ID, opts.HeadSHA, nameHash)
	if err != nil {
		return nil, err
	} else if exist {
		return nil, ErrCheckRunExist{RepoID: opts.Repo.ID, HeadSHA: opts.HeadSHA, Name: opts.Name}
	}

	checkRun := &CheckRun{
		RepoID:      opts.Repo.ID,
		Repo:        opts.Repo,
		Status:      toCheckRunStatus(opts.Status),
		Conclusion:  toCheckRunConclusion(opts.Conclusion),
		HeadSHA:     opts.HeadSHA,
		DetailsURL:  opts.DetailsURL,
		ExternalID:  opts.ExternalID,
		NameHash:    nameHash,
		Name:        opts.Name,
		Creator:     opts.Creator,
		CreatorID:   opts.Creator.ID,
		StartedAt:   opts.StartedAt,
		CompletedAt: opts.CompletedAt,
	}

	_, err = db.GetEngine(ctx).Insert(checkRun)
	if err != nil {
		return nil, err
	}

	// TODO: update output
	// if opts.Output != nil {
	// }

	return checkRun, committer.Commit()
}

func toCheckRunStatus(status api.CheckRunStatus) CheckRunStatus {
	if status == api.CheckRunStatusInProgress {
		return CheckRunStatusInProgress
	}

	if status == api.CheckRunStatusCompleted {
		return CheckRunStatusCompleted
	}

	return CheckRunStatusQueued
}

func (status CheckRunStatus) ToAPI() api.CheckRunStatus {
	if status == CheckRunStatusInProgress {
		return api.CheckRunStatusInProgress
	}

	if status == CheckRunStatusCompleted {
		return api.CheckRunStatusCompleted
	}

	return api.CheckRunStatusQueued
}

func toCheckRunConclusion(conclusion api.CheckRunConclusion) CheckRunConclusion {
	if conclusion == api.CheckRunConclusionActionRequired {
		return CheckRunConclusionActionRequired
	}
	if conclusion == api.CheckRunConclusionCancelled {
		return CheckRunConclusionCancelled
	}
	if conclusion == api.CheckRunConclusionFailure {
		return CheckRunConclusionFailure
	}
	if conclusion == api.CheckRunConclusionNeutral {
		return CheckRunConclusionNeutral
	}
	if conclusion == api.CheckRunConclusionSuccess {
		return CheckRunConclusionSuccess
	}
	if conclusion == api.CheckRunConclusionSkipped {
		return CheckRunConclusionSkipped
	}
	if conclusion == api.CheckRunConclusionStale {
		return CheckRunConclusionStale
	}
	if conclusion == api.CheckRunConclusionTimedOut {
		return CheckRunConclusionTimedOut
	}

	return CheckRunConclusionUnknow
}

func (conclusion CheckRunConclusion) ToAPI() api.CheckRunConclusion {
	if conclusion == CheckRunConclusionActionRequired {
		return api.CheckRunConclusionActionRequired
	}
	if conclusion == CheckRunConclusionCancelled {
		return api.CheckRunConclusionCancelled
	}
	if conclusion == CheckRunConclusionFailure {
		return api.CheckRunConclusionFailure
	}
	if conclusion == CheckRunConclusionNeutral {
		return api.CheckRunConclusionNeutral
	}
	if conclusion == CheckRunConclusionSuccess {
		return api.CheckRunConclusionSuccess
	}
	if conclusion == CheckRunConclusionSkipped {
		return api.CheckRunConclusionSkipped
	}
	if conclusion == CheckRunConclusionStale {
		return api.CheckRunConclusionStale
	}
	if conclusion == CheckRunConclusionTimedOut {
		return api.CheckRunConclusionTimedOut
	}

	return api.CheckRunConclusionNeutral
}

type ErrCheckRunNotExist struct {
	RepoID int64
	ID     int64
}

func (e ErrCheckRunNotExist) Error() string {
	return fmt.Sprintf("can't find check run [repo_id: %d, id: %d]", e.RepoID, e.ID)
}

func IsErrCheckRunNotExist(err error) bool {
	_, ok := err.(ErrCheckRunNotExist)
	return ok
}

// GetCheckRunByID get check run by id
func GetCheckRunByRepoIDAndID(ctx context.Context, repoID, id int64) (*CheckRun, error) {
	checkRun := &CheckRun{}

	exist, err := db.GetEngine(ctx).Where("id = ? AND repo_id = ?", id, repoID).Get(checkRun)
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrCheckRunNotExist{RepoID: repoID, ID: id}
	}

	return checkRun, nil
}

// GetLatestCheckRuns returns all check runs with a unique context for a given commit.
func GetLatestCheckRuns(ctx context.Context, repoID int64, sha string, listOptions db.ListOptions) ([]*CheckRun, int64, error) {
	ids := make([]int64, 0, 10)
	sess := db.GetEngine(ctx).Table(&CheckRun{}).
		Where("repo_id = ?", repoID).And("head_sha = ?", sha).
		Select("max( id ) as id").
		GroupBy("name_hash").OrderBy("max( id ) desc")

	sess = db.SetSessionPagination(sess, &listOptions)

	count, err := sess.FindAndCount(&ids)
	if err != nil {
		return nil, count, err
	}
	checkRuns := make([]*CheckRun, 0, len(ids))
	if len(ids) == 0 {
		return checkRuns, count, nil
	}
	return checkRuns, count, db.GetEngine(ctx).In("id", ids).Find(&checkRuns)
}

func CheckRunAppendToCommitStatus(statuses []*CommitStatus, checkRuns []*CheckRun, lang translation.Locale) []*CommitStatus {
	results := make(map[string]*CommitStatus)

	for _, checkRun := range checkRuns {
		results[checkRun.NameHash] = checkRun.ToStatus(lang)
	}

	for _, status := range statuses {
		if otherState, ok := results[status.ContextHash]; ok {
			if status.State.NoBetterThan(otherState.State) {
				results[status.ContextHash] = status
			}
		} else {
			results[status.ContextHash] = status
		}
	}

	resultsList := make([]*CommitStatus, 0, len(results))
	for _, result := range results {
		resultsList = append(resultsList, result)
	}

	return resultsList
}

// UpdateCheckRunOptions holds options for update a CheckRun
type UpdateCheckRunOptions struct {
	Repo        *repo_model.Repository
	Creator     *user_model.User
	Name        string
	Status      api.CheckRunStatus
	Conclusion  api.CheckRunConclusion
	DetailsURL  *string
	ExternalID  *string
	StartedAt   timeutil.TimeStamp
	CompletedAt timeutil.TimeStamp
	Output      *api.CheckRunOutput
}

func (opts *UpdateCheckRunOptions) Vaild(ck *CheckRun) error {
	if opts.Repo == nil || opts.Creator == nil {
		return errors.New("`repo` or `creator` not set")
	}

	if len(opts.Status) > 0 && opts.Status != api.CheckRunStatusQueued && opts.StartedAt == timeutil.TimeStamp(0) && ck.StartedAt == timeutil.TimeStamp(0) {
		return ErrUnVaildCheckRunOptions{Err: "request `started_at` if staus isn't `queued`"}
	}

	if opts.Status != api.CheckRunStatusCompleted {
		return nil
	}

	if opts.Conclusion == "" && ck.Conclusion == CheckRunConclusionUnknow {
		return ErrUnVaildCheckRunOptions{Err: "request `conclusion` if staus is `completed`"}
	}

	if opts.CompletedAt == timeutil.TimeStamp(0) {
		opts.CompletedAt = timeutil.TimeStampNow()
	}

	return nil
}

func (c *CheckRun) Update(ctx context.Context, opts UpdateCheckRunOptions) error {
	if err := opts.Vaild(c); err != nil {
		return err
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if len(opts.Name) > 0 && opts.Name != c.Name {
		nameHash := hashCommitStatusContext(opts.Name)
		exist, err := isCheckRunExist(ctx, opts.Repo.ID, c.HeadSHA, nameHash)
		if err != nil {
			return err
		} else if exist {
			return ErrCheckRunExist{RepoID: opts.Repo.ID, HeadSHA: c.HeadSHA, Name: opts.Name}
		}

		c.Name = opts.Name
		c.NameHash = nameHash
	}

	if len(opts.Status) > 0 {
		c.Status = toCheckRunStatus(opts.Status)
	}

	if len(opts.Conclusion) > 0 {
		c.Conclusion = toCheckRunConclusion(opts.Conclusion)
	}

	if opts.DetailsURL != nil {
		c.DetailsURL = *opts.DetailsURL
	}

	if opts.ExternalID != nil {
		c.ExternalID = *opts.ExternalID
	}

	if opts.StartedAt != timeutil.TimeStamp(0) {
		c.StartedAt = opts.StartedAt
	}

	if opts.CompletedAt != timeutil.TimeStamp(0) {
		c.CompletedAt = opts.CompletedAt
	}

	if c.Status != CheckRunStatusCompleted {
		c.Conclusion = CheckRunConclusionUnknow
		c.CompletedAt = 0
	}

	if c.Status == CheckRunStatusQueued {
		c.StartedAt = 0
	}

	_, err = db.GetEngine(ctx).ID(c.ID).Cols("name", "name_hash", "started_at", "completed_at", "status", "conclusion", "details_url", "external_id", "updated_unix").Update(c)
	if err != nil {
		return err
	}

	return committer.Commit()
}

type CheckRunOptions struct {
	db.ListOptions
	Status     string
	Conclusion string
	SortType   string
}

// GetCheckRuns returns all check runs for a given commit.
func GetCheckRuns(ctx context.Context, repo *repo_model.Repository, headSHA string, opts *CheckRunOptions) ([]*CheckRun, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.Page = setting.ItemsPerPage
	}

	countSession := listCheckRunStatement(ctx, repo, headSHA, opts)
	countSession = db.SetSessionPagination(countSession, opts)
	maxResults, err := countSession.Count(new(CheckRun))
	if err != nil {
		log.Error("Count CheckRuns: %v", err)
		return nil, maxResults, err
	}

	checkRuns := make([]*CheckRun, 0, opts.PageSize)
	findSession := listCheckRunStatement(ctx, repo, headSHA, opts)
	findSession = db.SetSessionPagination(findSession, opts)
	sortCheckRunsSession(findSession, opts.SortType)
	return checkRuns, maxResults, findSession.Find(&checkRuns)
}

func sortCheckRunsSession(sess *xorm.Session, sortType string) {
	switch sortType {
	case "oldest":
		sess.Asc("created_unix")
	case "recentupdate":
		sess.Desc("updated_unix")
	case "leastupdate":
		sess.Asc("updated_unix")
	default:
		sess.Desc("created_unix")
	}
}

func listCheckRunStatement(ctx context.Context, repo *repo_model.Repository, headSHA string, opts *CheckRunOptions) *xorm.Session {
	sess := db.GetEngine(ctx).Where("repo_id = ?", repo.ID).And("head_sha = ?", headSHA)
	switch opts.Status {
	case "queued", "in_progress", "completed":
		sess.And("status = ?", toCheckRunStatus(api.CheckRunStatus(opts.Status)))
	}
	switch opts.Conclusion {
	case "action_required", "cancelled", "failure", "neutral", "success", "skipped", "stale", "timed_out":
		sess.And("conclusion = ?", toCheckRunConclusion(api.CheckRunConclusion(opts.Status)))
	}
	return sess
}
