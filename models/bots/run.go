// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/timeutil"
	api "code.gitea.io/gitea/modules/structs"

	"xorm.io/builder"
	"github.com/nektos/act/pkg/jobparser"
	"golang.org/x/exp/slices"
)

// Run represents a run of a workflow file
type Run struct {
	ID            int64
	Title         string
	RepoID        int64                  `xorm:"index unique(repo_index)"`
	Repo          *repo_model.Repository `xorm:"-"`
	WorkflowID    string                 `xorm:"index"`                    // the name of workflow file
	Index         int64                  `xorm:"index unique(repo_index)"` // a unique number for each run of a repository
	TriggerUserID int64
	TriggerUser   *user_model.User `xorm:"-"`
	Ref           string
	CommitSHA     string
	Event         webhook.HookEventType
	Token         string // token for this task
	Grant         string // permissions for this task
	EventPayload  string `xorm:"LONGTEXT"`
	Status        Status `xorm:"index"`
	Started       timeutil.TimeStamp
	Stopped       timeutil.TimeStamp
	Created       timeutil.TimeStamp `xorm:"created"`
	Updated       timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(Run))
	db.RegisterModel(new(RunIndex))
}

func (Run) TableName() string {
	return "bots_run"
}

func (run *Run) HTMLURL() string {
	return fmt.Sprintf("%s/builds/runs/%d", run.Repo.HTMLURL(), run.Index)
}

// LoadAttributes load Repo TriggerUser if not loaded
func (run *Run) LoadAttributes(ctx context.Context) error {
	if run == nil {
		return nil
	}

	if run.Repo == nil {
		repo, err := repo_model.GetRepositoryByIDCtx(ctx, run.RepoID)
		if err != nil {
			return err
		}
		run.Repo = repo
	}
	if err := run.Repo.LoadAttributes(ctx); err != nil {
		return err
	}

	if run.TriggerUser == nil {
		u, err := user_model.GetUserByIDCtx(ctx, run.TriggerUserID)
		if err != nil {
			return err
		}
		run.TriggerUser = u
	}

	return nil
}

func (run *Run) TakeTime() time.Duration {
	if run.Started == 0 {
		return 0
	}
	started := run.Started.AsTime()
	if run.Status.IsDone() {
		return run.Stopped.AsTime().Sub(started)
	}
	run.Stopped.AsTime().Sub(started)
	return time.Since(started).Truncate(time.Second)
}

func (run *Run) GetPushEventPayload() (*api.PushPayload, error) {
	if run.Event == webhook.HookEventPush {
		var payload api.PushPayload
		if err := json.Unmarshal([]byte(run.EventPayload), &payload); err != nil {
			return nil, err
		}
		return &payload, nil
	}
	return nil, fmt.Errorf("event %s is not a push event", run.Event)
}

func updateRepoRunsNumbers(ctx context.Context, repo *repo_model.Repository) error {
	_, err := db.GetEngine(ctx).ID(repo.ID).
		SetExpr("num_runs",
			builder.Select("count(*)").From("bots_run").
				Where(builder.Eq{"repo_id": repo.ID}),
		).
		SetExpr("num_closed_runs",
			builder.Select("count(*)").From("bots_run").
				Where(builder.Eq{
					"repo_id": repo.ID,
				}.And(
					builder.In("status",
						StatusSuccess,
						StatusFailure,
						StatusCancelled,
						StatusSkipped,
					),
				),
				),
		).
		Update(repo)
	return err
}

// InsertRun inserts a bot run
func InsertRun(run *Run, jobs []*jobparser.SingleWorkflow) error {
	index, err := db.GetNextResourceIndex("bots_run_index", run.RepoID)
	if err != nil {
		return err
	}
	run.Index = index

	ctx, commiter, err := db.TxContext()
	if err != nil {
		return err
	}
	defer commiter.Close()

	if err := db.Insert(ctx, run); err != nil {
		return err
	}

	if run.Repo == nil {
		repo, err := repo_model.GetRepositoryByIDCtx(ctx, run.RepoID)
		if err != nil {
			return err
		}
		run.Repo = repo
	}

	if err := updateRepoRunsNumbers(ctx, run.Repo); err != nil {
		return err
	}

	runJobs := make([]*RunJob, 0, len(jobs))
	for _, v := range jobs {
		id, job := v.Job()
		payload, _ := v.Marshal()
		runJobs = append(runJobs, &RunJob{
			RunID:           run.ID,
			Name:            job.Name,
			Ready:           true, // TODO: should be false if there are needs to satisfy
			WorkflowPayload: payload,
			JobID:           id,
			Needs:           nil, // TODO: analyse needs
			RunsOn:          job.RunsOn(),
			Status:          StatusWaiting,
		})
	}
	if err := db.Insert(ctx, runJobs); err != nil {
		return err
	}

	return commiter.Commit()
}

// ErrRunNotExist represents an error for bot run not exist
type ErrRunNotExist struct {
	ID     int64
	RepoID int64
	Index  int64
}

func (err ErrRunNotExist) Error() string {
	if err.RepoID > 0 {
		return fmt.Sprintf("run repe_id [%d] index [%d] is not exist", err.RepoID, err.Index)
	}
	return fmt.Sprintf("run [%d] is not exist", err.ID)
}

func GetRunByID(ctx context.Context, id int64) (*Run, error) {
	var run Run
	has, err := db.GetEngine(ctx).Where("id=?", id).Get(&run)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRunNotExist{
			ID: id,
		}
	}

	return &run, nil
}

func GetRunByIndex(ctx context.Context, repoID, index int64) (*Run, error) {
	run := &Run{
		RepoID: repoID,
		Index:  index,
	}
	has, err := db.GetEngine(ctx).Get(run)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRunNotExist{
			RepoID: repoID,
			Index:  index,
		}
	}

	return run, nil
}

func UpdateRun(ctx context.Context, run *Run, cols ...string) error {
	sess := db.GetEngine(ctx).ID(run.ID)
	if len(cols) > 0 {
		sess.Cols(cols...)
	}
	_, err := sess.Update(run)

	if run.Status != 0 || slices.Contains(cols, "status") {
		if run.RepoID == 0 {
			run, err = GetRunByID(ctx, run.ID)
			if err != nil {
				return err
			}
		}
		if run.Repo == nil {
			repo, err := repo_model.GetRepositoryByIDCtx(ctx, run.RepoID)
			if err != nil {
				return err
			}
			run.Repo = repo
		}
		if err := updateRepoRunsNumbers(ctx, run.Repo); err != nil {
			return err
		}
	}

	return err
}

type RunIndex db.ResourceIndex

func (RunIndex) TableName() string {
	return "bots_run_index"
}
