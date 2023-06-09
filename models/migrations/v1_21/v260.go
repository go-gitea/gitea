// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"code.gitea.io/gitea/models/migrations/base"

	"xorm.io/xorm"
)

func AddLabelsToActRunner(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	type ActionRunner struct {
		ID           int64
		AgentLabels  []string
		CustomLabels []string
		Labels       []string `xorm:"TEXT"` // new column
	}

	// add column of `labels` to the `action_runner` table.
	if err := sess.Sync(new(ActionRunner)); err != nil {
		return err
	}

	// combine "agent_labels" col and "custom_labels" col to "labels" col.
	var runners []*ActionRunner
	if err := sess.Table("action_runner").Select("id, agent_labels, custom_labels").Find(&runners); err != nil {
		return err
	}

	for _, r := range runners {
		r.Labels = append(r.Labels, r.AgentLabels...)
		r.Labels = append(r.Labels, r.CustomLabels...)

		if _, err := sess.ID(r.ID).Cols("labels").Update(r); err != nil {
			return err
		}
	}

	// drop "agent_labels" and "custom_labels" cols
	if err := base.DropTableColumns(sess, "action_runner", "agent_labels", "custom_labels"); err != nil {
		return err
	}

	return sess.Commit()
}
