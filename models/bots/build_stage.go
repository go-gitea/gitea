// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

type BuildStage struct {
	ID        int64
	BuildID   int64 `xorm:"index"`
	Number    int64
	Name      string
	Kind      string
	Type      string
	Filename  string
	Status    BuildStatus
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
