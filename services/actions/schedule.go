// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/log"

	"github.com/robfig/cron/v3"
)

var schedule *Schedule

type Schedule struct {
	c *cron.Cron
}

func (s *Schedule) AddFunc(spec string, f func()) (int, error) {
	id, err := s.c.AddFunc(spec, f)
	return int(id), err
}

func (s *Schedule) Remove(ids []int) {
	for _, id := range ids {
		s.c.Remove(cron.EntryID(id))
	}
}

func newSchedule() {
	c := cron.New()
	c.Start()
	schedule = &Schedule{
		c: c,
	}
}

func resetSchedule() {
	schedules, _, err := actions_model.FindSchedules(
		context.Background(),
		actions_model.FindScheduleOptions{GetAll: true},
	)
	if err != nil {
		log.Error("FindSchedules: %v", err)
	}

	for _, schedule := range schedules {
		entryIDs := []int{}
		for _, spec := range schedule.Specs {
			id, err := CreateScheduleTask(context.Background(), schedule, spec)
			if err != nil {
				continue
			}
			entryIDs = append(entryIDs, id)
		}
		schedule.EntryIDs = entryIDs
		if err := actions_model.UpdateSchedule(context.Background(), schedule, "entry_ids"); err != nil {
			log.Error("UpdateSchedule: %v", err)
		}
	}
}
