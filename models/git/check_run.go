// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"fmt"
	"path"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
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
	"github.com/nektos/act/pkg/jobparser"
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
	Summary    string           `xorm:"-"` // output -> Title/Summary
	NameHash   string           `xorm:"char(40) INDEX UNIQUE(repo_sha_name)"`
	Name       string           `xorm:"TEXT"`
	Creator    *user_model.User `xorm:"-"`
	CreatorID  int64

	StartedAt   timeutil.TimeStamp
	CompletedAt timeutil.TimeStamp

	Output       *CheckRunOutput `xorm:"-"`
	outputLoaded bool            `xorm:"-"`

	ActionRunID    int64
	ActionRun      *actions_model.ActionRun `xorm:"-"`
	ActionJobIndex int64

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func (status *CheckRun) LoadAttributes(ctx context.Context) (err error) {
	if status.Repo == nil {
		status.Repo, err = repo_model.GetRepositoryByID(ctx, status.RepoID)
		if err != nil {
			return fmt.Errorf("getRepositoryByID [%d]: %w", status.RepoID, err)
		}
	}

	if status.ActionRunID > 0 && status.Creator == nil {
		status.Creator = user_model.NewActionsUser()
	} else if status.Creator == nil && status.CreatorID > 0 {
		status.Creator, err = user_model.GetUserByID(ctx, status.CreatorID)
		if err != nil {
			return fmt.Errorf("getUserByID [%d]: %w", status.CreatorID, err)
		}
	}

	return nil
}

func (c *CheckRun) HasOutputData(ctx context.Context) bool {
	_ = c.LoadOutput(ctx)

	if c.Output == nil {
		return false
	}

	if len(c.Output.Title) > 0 {
		return true
	}

	if len(c.Output.Summary) > 0 {
		return true
	}

	if len(c.Output.Annotations) > 0 {
		return true
	}

	return false
}

func (c *CheckRun) LoadOutput(ctx context.Context) error {
	if c.outputLoaded {
		return nil
	}

	output := &CheckRunOutput{}
	exist, err := db.GetEngine(ctx).Where("check_run_id = ?", c.ID).Get(output)
	if err != nil {
		return err
	}

	c.outputLoaded = true
	if exist {
		c.Output = output
	}

	return nil
}

func (c *CheckRun) updateOutput(ctx context.Context, output *api.CheckRunOutput) error {
	if output == nil {
		return nil
	}

	err := c.LoadOutput(ctx)
	if err != nil {
		return err
	}

	needCreate := c.Output == nil
	if needCreate {
		c.Output = &CheckRunOutput{
			CheckRunID: c.ID,
		}
	}

	if output.Title != nil {
		c.Output.Title = *output.Title
	}

	if output.Summary != nil {
		c.Output.Summary = *output.Summary
	}

	if output.AnnotationsURL != nil {
		c.Output.AnnotationsURL = *output.AnnotationsURL
	}

	if output.Text != nil {
		c.Output.Text = *output.Text
	}

	if needCreate {
		annotations := make([]*api.CheckRunAnnotation, 0, len(output.Annotations))

		for _, a := range output.Annotations {
			if a.DeleteMark != nil && *a.DeleteMark {
				continue
			}

			a.AppendMark = nil
			a.DeleteMark = nil

			annotations = append(annotations, a)
		}

		c.Output.Annotations = annotations
	} else {
		annotationsMap := make(map[string]*api.CheckRunAnnotation)
		for _, a := range c.Output.Annotations {
			annotationsMap[a.ID()] = a
		}

		for _, a := range output.Annotations {
			_, exist := annotationsMap[a.ID()]
			if exist && a.DeleteMark != nil && *a.DeleteMark {
				delete(annotationsMap, a.ID())
				continue
			}

			if exist && a.AppendMark != nil && *a.AppendMark {
				continue
			}

			a.AppendMark = nil
			a.DeleteMark = nil
			annotationsMap[a.ID()] = a
		}

		c.Output.Annotations = make([]*api.CheckRunAnnotation, 0, len(annotationsMap))
		for _, a := range annotationsMap {
			c.Output.Annotations = append(c.Output.Annotations, a)
		}
	}

	for i, a := range c.Output.Annotations {
		c.Output.Annotations[i].Index = hashCommitStatusContext(a.Title + a.Message)
	}

	if needCreate {
		_, err = db.GetEngine(ctx).Insert(c.Output)
		return err
	}

	_, err = db.GetEngine(ctx).ID(c.Output.ID).Cols("title", "summary", "text", "annotations_url", "annotations").Update(c.Output)

	return err
}

func (c *CheckRun) GetTargetURL() string {
	if c.ActionRunID > 0 {
		c.ActionRun = &actions_model.ActionRun{
			ID:     c.ActionRunID,
			Repo:   c.Repo,
			RepoID: c.RepoID,
		}

		return fmt.Sprintf("%s/jobs/%d", c.ActionRun.Link(), c.ActionJobIndex)
	}

	return c.DetailsURL
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
		TargetURL:   c.GetTargetURL(),
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
	ID             int64                     `xorm:"pk autoincr"`
	CheckRunID     int64                     `xorm:"INDEX"`
	Title          string                    `xorm:"TEXT"`
	Summary        string                    `xorm:"TEXT"`
	Text           string                    `xorm:"TEXT"`
	AnnotationsURL string                    `xorm:"TEXT"`
	Annotations    []*api.CheckRunAnnotation `xorm:"JSON TEXT"`
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

	err = checkRun.updateOutput(ctx, opts.Output)
	if err != nil {
		return nil, err
	}

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
		if _, ok := results[status.ContextHash]; !ok {
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

	if opts.CompletedAt == timeutil.TimeStamp(0) && ck.CompletedAt == timeutil.TimeStamp(0) {
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

	err = c.updateOutput(ctx, opts.Output)
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

// FindRepoRecentCheckRunNames returns repository's recent check run names
func FindRepoRecentCheckRunNames(ctx context.Context, repoID int64, before time.Duration) ([]string, error) {
	start := timeutil.TimeStampNow().AddDuration(-before)
	ids := make([]int64, 0, 10)
	if err := db.GetEngine(ctx).Table("check_run").
		Where("repo_id = ?", repoID).
		And("updated_unix >= ?", start).
		Select("max( id ) as id").
		GroupBy("name_hash").OrderBy("max( id ) desc").
		Find(&ids); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	return names, db.GetEngine(ctx).Select("name").Table("check_run").In("id", ids).Find(&names)
}

func UpdatCheckRunForAction(ctx context.Context, job *actions_model.ActionRunJob, event, sha string) error {
	run := job.Run
	repo := run.Repo

	// TODO: store workflow name as a field in ActionRun to avoid parsing
	runName := path.Base(run.WorkflowID)
	if wfs, err := jobparser.Parse(job.WorkflowPayload); err == nil && len(wfs) > 0 {
		runName = wfs[0].Name
	}

	name := fmt.Sprintf("%s / %s (%s)", runName, job.Name, event)
	nameHash := hashCommitStatusContext(name)

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	checkRun := &CheckRun{}
	exist, err := db.GetEngine(ctx).
		Where("repo_id = ? AND head_sha = ? AND name_hash = ?",
			repo.ID, sha, nameHash).
		Get(checkRun)
	if err != nil {
		return err
	}

	if !exist {
		checkRun.Repo = repo
		checkRun.RepoID = repo.ID
		checkRun.Creator = user_model.NewActionsUser()
		checkRun.CreatorID = checkRun.Creator.ID
		checkRun.HeadSHA = sha
		checkRun.Name = name
		checkRun.NameHash = nameHash
	}

	index, err := getIndexOfJob(ctx, job)
	if err != nil {
		return fmt.Errorf("getIndexOfJob: %w", err)
	}
	checkRun.ActionRunID = job.RunID
	checkRun.ActionJobIndex = int64(index)

	checkRun.Status = actionJobToCheckRunStatus(job)
	checkRun.Conclusion = actionJobToCheckRunConclusion(job)

	if checkRun.Status == CheckRunStatusQueued {
		checkRun.StartedAt = 0
	} else {
		checkRun.StartedAt = job.Started
	}

	if checkRun.Status != CheckRunStatusCompleted {
		checkRun.CompletedAt = 0
	} else {
		checkRun.CompletedAt = job.Stopped
	}

	if !exist {
		_, err = db.GetEngine(ctx).Insert(checkRun)
	} else {
		_, err = db.GetEngine(ctx).ID(checkRun.ID).
			Cols("started_at", "completed_at", "status", "conclusion", "updated_unix",
				"action_run_id", "action_job_index").
			Update(checkRun)
	}
	if err != nil {
		return err
	}

	return committer.Commit()
}

func getIndexOfJob(ctx context.Context, job *actions_model.ActionRunJob) (int, error) {
	// TODO: store job index as a field in ActionRunJob to avoid this
	jobs, err := actions_model.GetRunJobsByRunID(ctx, job.RunID)
	if err != nil {
		return 0, err
	}
	for i, v := range jobs {
		if v.ID == job.ID {
			return i, nil
		}
	}
	return 0, nil
}

func actionJobToCheckRunStatus(job *actions_model.ActionRunJob) CheckRunStatus {
	switch job.Status {
	case actions_model.StatusSuccess, actions_model.StatusSkipped, actions_model.StatusFailure, actions_model.StatusCancelled:
		return CheckRunStatusCompleted
	case actions_model.StatusWaiting, actions_model.StatusBlocked:
		return CheckRunStatusQueued
	case actions_model.StatusRunning:
		return CheckRunStatusInProgress
	default:
		return CheckRunStatusQueued
	}
}

func actionJobToCheckRunConclusion(job *actions_model.ActionRunJob) CheckRunConclusion {
	switch job.Status {
	case actions_model.StatusSuccess:
		return CheckRunConclusionSuccess
	case actions_model.StatusSkipped:
		return CheckRunConclusionSkipped
	case actions_model.StatusFailure:
		return CheckRunConclusionFailure
	case actions_model.StatusCancelled:
		return CheckRunConclusionCancelled
	default:
		return CheckRunConclusionUnknow
	}
}
