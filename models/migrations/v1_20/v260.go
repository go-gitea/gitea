// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"xorm.io/xorm"
)

func AddLabelsToActRunner(x *xorm.Engine) error {
	type ActionRunner struct {
		Labels []string `xorm:"TEXT"`
	}

	// add column of `labels` to the `action_runner` table.
	if err := x.Sync(new(ActionRunner)); err != nil {
		return err
	}

	// combine "agent labels" col and "custom labels" col to "labels" col.
	err := x.Iterate(new(actions_model.ActionRunner), func(idx int, bean interface{}) error {
		runner := bean.(*actions_model.ActionRunner)
		runner.Labels = append(runner.Labels, runner.AgentLabels...)  //nolint
		runner.Labels = append(runner.Labels, runner.CustomLabels...) //nolint
		err := actions_model.UpdateRunner(db.DefaultContext, runner, "labels")
		return err
	})

	return err
}
