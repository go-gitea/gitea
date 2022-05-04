// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"github.com/google/uuid"
	"xorm.io/builder"
)

func init() {
	db.RegisterModel(new(Build))
	db.RegisterModel(new(BuildIndex))
}

// BuildStatus represents a build status
type BuildStatus int

// enumerate all the statuses of bot build
const (
	BuildPending  BuildStatus = iota // wait for assign
	BuildAssigned                    // assigned to a runner
	BuildRunning                     // running
	BuildFailed
	BuildFinished
	BuildCanceled
	BuildTimeout
)

func (status BuildStatus) IsPending() bool {
	return status == BuildPending || status == BuildAssigned
}

func (status BuildStatus) IsRunning() bool {
	return status == BuildRunning
}

func (status BuildStatus) IsFailed() bool {
	return status == BuildFailed || status == BuildCanceled || status == BuildTimeout
}

func (status BuildStatus) IsSuccess() bool {
	return status == BuildFinished
}

// Build represnets bot build task
type Build struct {
	ID            int64
	Title         string
	UUID          string `xorm:"CHAR(36)"`
	Index         int64  `xorm:"index unique(repo_index)"`
	RepoID        int64  `xorm:"index unique(repo_index)"`
	TriggerUserID int64
	TriggerUser   *user_model.User `xorm:"-"`
	Ref           string
	CommitSHA     string
	Event         webhook.HookEventType
	Token         string             // token for this task
	Grant         string             // permissions for this task
	EventPayload  string             `xorm:"LONGTEXT"`
	RunnerID      int64              `xorm:"index"`
	Status        BuildStatus        `xorm:"index"`
	Created       timeutil.TimeStamp `xorm:"created"`
	StartTime     timeutil.TimeStamp
	EndTime       timeutil.TimeStamp
	Updated       timeutil.TimeStamp `xorm:"updated"`
}

// TableName represents a bot build
func (Build) TableName() string {
	return "bots_build"
}

func (t *Build) HTMLURL() string {
	return fmt.Sprintf("")
}

func updateRepoBuildsNumbers(ctx context.Context, repo *repo_model.Repository) error {
	_, err := db.GetEngine(ctx).ID(repo.ID).
		SetExpr("num_builds",
			builder.Select("count(*)").From("bots_build").
				Where(builder.Eq{"repo_id": repo.ID}),
		).
		SetExpr("num_closed_builds",
			builder.Select("count(*)").From("bots_build").
				Where(builder.Eq{
					"repo_id": repo.ID,
				}.And(
					builder.In("status", BuildFailed, BuildCanceled, BuildTimeout, BuildFinished),
				),
				),
		).
		Update(repo)
	return err
}

// InsertBuild inserts a bot build task
func InsertBuild(t *Build, workflowsStatuses map[string]map[string]BuildStatus) error {
	if t.UUID == "" {
		t.UUID = uuid.New().String()
	}
	index, err := db.GetNextResourceIndex("bots_build_index", t.RepoID)
	if err != nil {
		return err
	}
	t.Index = index

	ctx, commiter, err := db.TxContext()
	if err != nil {
		return err
	}
	defer commiter.Close()

	if err := db.Insert(ctx, t); err != nil {
		return err
	}

	if err := updateRepoBuildsNumbers(ctx, &repo_model.Repository{ID: t.RepoID}); err != nil {
		return err
	}

	var buildJobs []BuildJob
	for filename, workflow := range workflowsStatuses {
		for job, status := range workflow {
			buildJobs = append(buildJobs, BuildJob{
				BuildID:  t.ID,
				Filename: filename,
				Jobname:  job,
				Status:   status,
			})
		}
	}
	if err := db.Insert(ctx, buildJobs); err != nil {
		return err
	}

	if err := commiter.Commit(); err != nil {
		return err
	}

	if err := CreateBuildLog(t.ID); err != nil {
		log.Error("create build log for %d table failed, will try it again when received logs", t.ID)
	}
	return nil
}

// UpdateBuild updates bot build
func UpdateBuild(t *Build, cols ...string) error {
	_, err := db.GetEngine(db.DefaultContext).ID(t.ID).Cols(cols...).Update(t)
	return err
}

// ErrBuildNotExist represents an error for bot build not exist
type ErrBuildNotExist struct {
	RepoID int64
	Index  int64
	UUID   string
}

func (err ErrBuildNotExist) Error() string {
	return fmt.Sprintf("Bot build [%s] is not exist", err.UUID)
}

// GetBuildByUUID gets bot build by uuid
func GetBuildByUUID(buildUUID string) (*Build, error) {
	var build Build
	has, err := db.GetEngine(db.DefaultContext).Where("uuid=?", buildUUID).Get(&build)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrBuildNotExist{
			UUID: buildUUID,
		}
	}
	return &build, nil
}

// GetCurBuildByID return the build for the bot
func GetCurBuildByID(runnerID int64) (*Build, error) {
	var builds []Build
	err := db.GetEngine(db.DefaultContext).
		Where("runner_id=?", runnerID).
		And("status=?", BuildPending).
		Asc("created").
		Find(&builds)
	if err != nil {
		return nil, err
	}
	if len(builds) == 0 {
		return nil, nil
	}
	return &builds[0], err
}

// GetCurBuildByUUID return the task for the bot
func GetCurBuildByUUID(runnerUUID string) (*Build, error) {
	runner, err := GetRunnerByUUID(runnerUUID)
	if err != nil {
		return nil, err
	}
	return GetCurBuildByID(runner.ID)
}

func GetBuildByRepoAndIndex(repoID, index int64) (*Build, error) {
	var build Build
	has, err := db.GetEngine(db.DefaultContext).Where("repo_id=?", repoID).
		And("`index` = ?", index).
		Get(&build)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrBuildNotExist{
			RepoID: repoID,
			Index:  index,
		}
	}
	return &build, nil
}

// AssignBuildToRunner assign a build to a runner
func AssignBuildToRunner(buildID int64, runnerID int64) error {
	cnt, err := db.GetEngine(db.DefaultContext).
		Where("runner_id=0").
		And("id=?", buildID).
		Cols("runner_id").
		Update(&Build{
			RunnerID: runnerID,
		})
	if err != nil {
		return err
	}
	if cnt != 1 {
		return errors.New("assign faild")
	}
	return nil
}

type FindBuildOptions struct {
	db.ListOptions
	RepoID   int64
	IsClosed util.OptionalBool
}

func (opts FindBuildOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.IsClosed.IsTrue() {
		cond = cond.And(builder.Expr("status IN (?,?,?,?)", BuildCanceled, BuildFailed, BuildTimeout, BuildFinished))
	} else if opts.IsClosed.IsFalse() {
		cond = cond.And(builder.Expr("status IN (?,?,?)", BuildPending, BuildAssigned, BuildRunning))
	}
	return cond
}

func FindBuilds(opts FindBuildOptions) (BuildList, error) {
	sess := db.GetEngine(db.DefaultContext).Where(opts.toConds())
	if opts.ListOptions.PageSize > 0 {
		skip, take := opts.GetSkipTake()
		sess.Limit(take, skip)
	}
	var builds []*Build
	return builds, sess.Find(&builds)
}

func CountBuilds(opts FindBuildOptions) (int64, error) {
	return db.GetEngine(db.DefaultContext).Table("bots_build").Where(opts.toConds()).Count()
}

type BuildIndex db.ResourceIndex

// TableName represents a bot build index
func (BuildIndex) TableName() string {
	return "bots_build_index"
}
