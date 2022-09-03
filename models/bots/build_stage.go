// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/bots/core"
	"xorm.io/builder"
)

type BuildStage struct {
	ID        int64
	BuildID   int64 `xorm:"index"`
	Number    int64
	Name      string
	Kind      string
	Type      string
	Machine   string
	OS        string
	Arch      string
	Filename  string
	Status    core.BuildStatus
	Started   timeutil.TimeStamp
	Stopped   timeutil.TimeStamp
	LogToFile bool               // read log from database or from storage
	Created   timeutil.TimeStamp `xorm:"created"`
}

func (bj BuildStage) TableName() string {
	return "bots_build_stage"
}

func init() {
	db.RegisterModel(new(BuildStage))
}

type FindStageOptions struct {
	db.ListOptions
	BuildID  int64
	IsClosed util.OptionalBool
}

func (opts FindStageOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.BuildID > 0 {
		cond = cond.And(builder.Eq{"build_id": opts.BuildID})
	}
	if opts.IsClosed.IsTrue() {
		cond = cond.And(builder.Expr("status IN (?,?,?,?)", core.StatusError, core.StatusFailing, core.StatusPassing))
	} else if opts.IsClosed.IsFalse() {
		cond = cond.And(builder.Expr("status IN (?,?,?)", core.StatusPending, core.StatusRunning))
	}
	return cond
}

func FindStages(ctx context.Context, opts FindStageOptions) (BuildStageList, error) {
	sess := db.GetEngine(ctx).Where(opts.toConds())
	if opts.ListOptions.PageSize > 0 {
		skip, take := opts.GetSkipTake()
		sess.Limit(take, skip)
	}
	var rows []*BuildStage
	return rows, sess.Find(&rows)
}

func GetBuildWorkflows(buildID int64) (map[string]map[string]*BuildStage, error) {
	jobs := make(map[string]map[string]*BuildStage)
	err := db.GetEngine(db.DefaultContext).Iterate(new(BuildStage), func(idx int, bean interface{}) error {
		job := bean.(*BuildStage)
		_, ok := jobs[job.Filename]
		if !ok {
			jobs[job.Filename] = make(map[string]*BuildStage)
		}
		jobs[job.Filename][job.Name] = job
		return nil
	})
	return jobs, err
}
