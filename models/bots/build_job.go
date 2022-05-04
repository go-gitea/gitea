// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

type BuildJob struct {
	ID        int64
	BuildID   int64 `xorm:"index"`
	Filename  string
	Jobname   string
	Status    BuildStatus
	LogToFile bool               // read log from database or from storage
	Created   timeutil.TimeStamp `xorm:"created"`
}

func (bj BuildJob) TableName() string {
	return "bots_build_job"
}

func init() {
	db.RegisterModel(new(BuildJob))
}

func GetBuildWorkflows(buildID int64) (map[string]map[string]*BuildJob, error) {
	jobs := make(map[string]map[string]*BuildJob)
	err := db.GetEngine(db.DefaultContext).Iterate(new(BuildJob), func(idx int, bean interface{}) error {
		job := bean.(*BuildJob)
		_, ok := jobs[job.Filename]
		if !ok {
			jobs[job.Filename] = make(map[string]*BuildJob)
		}
		jobs[job.Filename][job.Jobname] = job
		return nil
	})
	return jobs, err
}
