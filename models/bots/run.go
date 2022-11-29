// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/nektos/act/pkg/jobparser"
	"golang.org/x/exp/slices"
	"xorm.io/builder"
)

// BotRun represents a run of a workflow file
type BotRun struct {
	ID                int64
	Title             string
	RepoID            int64                  `xorm:"index unique(repo_index)"`
	Repo              *repo_model.Repository `xorm:"-"`
	OwnerID           int64                  `xorm:"index"`
	WorkflowID        string                 `xorm:"index"`                    // the name of workflow file
	Index             int64                  `xorm:"index unique(repo_index)"` // a unique number for each run of a repository
	TriggerUserID     int64
	TriggerUser       *user_model.User `xorm:"-"`
	Ref               string
	CommitSHA         string
	IsForkPullRequest bool
	Event             webhook.HookEventType
	EventPayload      string `xorm:"LONGTEXT"`
	Status            Status `xorm:"index"`
	Started           timeutil.TimeStamp
	Stopped           timeutil.TimeStamp
	Created           timeutil.TimeStamp `xorm:"created"`
	Updated           timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(BotRun))
	db.RegisterModel(new(BotRunIndex))
}

func (run *BotRun) HTMLURL() string {
	return fmt.Sprintf("%s/bots/runs/%d", run.Repo.HTMLURL(), run.Index)
}

// LoadAttributes load Repo TriggerUser if not loaded
func (run *BotRun) LoadAttributes(ctx context.Context) error {
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
		u, err := user_model.GetPossbileUserByID(ctx, run.TriggerUserID)
		if err != nil {
			return err
		}
		run.TriggerUser = u
	}

	return nil
}

func (run *BotRun) TakeTime() time.Duration {
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

func (run *BotRun) GetPushEventPayload() (*api.PushPayload, error) {
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
func InsertRun(run *BotRun, jobs []*jobparser.SingleWorkflow) error {
	ctx, commiter, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer commiter.Close()

	index, err := db.GetNextResourceIndex(ctx, "bots_run_index", run.RepoID)
	if err != nil {
		return err
	}
	run.Index = index

	if run.Status.IsUnknown() {
		run.Status = StatusWaiting
	}

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

	runJobs := make([]*BotRunJob, 0, len(jobs))
	for _, v := range jobs {
		id, job := v.Job()
		needs := job.Needs()
		job.EraseNeeds()
		payload, _ := v.Marshal()
		status := StatusWaiting
		if len(needs) > 0 {
			status = StatusBlocked
		}
		runJobs = append(runJobs, &BotRunJob{
			RunID:             run.ID,
			RepoID:            run.RepoID,
			OwnerID:           run.OwnerID,
			CommitSHA:         run.CommitSHA,
			IsForkPullRequest: run.IsForkPullRequest,
			Name:              job.Name,
			WorkflowPayload:   payload,
			JobID:             id,
			Needs:             needs,
			RunsOn:            job.RunsOn(),
			Status:            status,
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

func GetRunByID(ctx context.Context, id int64) (*BotRun, error) {
	var run BotRun
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

func GetRunByIndex(ctx context.Context, repoID, index int64) (*BotRun, error) {
	run := &BotRun{
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

func UpdateRun(ctx context.Context, run *BotRun, cols ...string) error {
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

type BotRunIndex db.ResourceIndex
