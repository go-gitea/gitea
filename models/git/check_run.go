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
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/translation"
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
		stat.Description = lang.Tr("check_runs.conclusion."+string(c.Conclusion.ToAPI()), c.CompletedAt-c.StartedAt)
	} else if c.Status == CheckRunStatusInProgress {
		stat.Description = lang.Tr("check_runs.status.runing", timeutil.TimeStampNow()-c.StartedAt)
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

func (opts *NewCheckRunOptions) Vaild() error {
	if opts.Repo == nil || opts.Creator == nil {
		return errors.New("`repo` or `creater` not set")
	}

	if len(opts.Name) == 0 {
		return ErrUnVaildCheckRunOptions{Err: "request `name`"}
	}

	if len(opts.HeadSHA) == 0 {
		return ErrUnVaildCheckRunOptions{Err: "request `head_sha`"}
	}

	if len(opts.Status) > 0 && opts.Status != api.CheckRunStatusQueued && opts.StartedAt == timeutil.TimeStamp(0) {
		return ErrUnVaildCheckRunOptions{Err: "request `start_at` if staus isn't `queued`"}
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
		return nil, ErrCheckRunExist{RepoID: opts.Repo.RepoID, HeadSHA: opts.HeadSHA, Name: opts.Name}
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
	ID int64
}

func (e ErrCheckRunNotExist) Error() string {
	return fmt.Sprintf("can't find check run [id: %d]", e.ID)
}

func IsErrCheckRunNotExist(err error) bool {
	_, ok := err.(ErrCheckRunNotExist)
	return ok
}

// GetCheckRunByID get check run by id
func GetCheckRunByID(ctx context.Context, id int64) (*CheckRun, error) {
	checkRun := &CheckRun{}

	exist, err := db.GetEngine(ctx).ID(id).Get(checkRun)
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrCheckRunNotExist{ID: id}
	}

	return checkRun, nil
}
