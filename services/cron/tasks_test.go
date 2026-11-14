// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"context"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
)

func TestAddTaskToScheduler(t *testing.T) {
	assert.Empty(t, scheduler.Jobs())
	defer func() {
		for _, j := range scheduler.Jobs() {
			_ = scheduler.RemoveJob(j.ID())
		}
	}()

	for i, task := range []struct {
		name, schedule string
	}{
		{"task 1", "5 4 * * *"},    // no seconds
		{"task 2", "30 5 4 * * *"}, // with seconds
	} {
		assert.NoError(t, addTaskToScheduler(&Task{
			Name:   task.name,
			config: &BaseConfig{Schedule: task.schedule},
		}))
		jobs := scheduler.Jobs()
		if i > 0 {
			// the item order is not guaranteed, so we need to sort it before "assert"
			sort.Slice(jobs, func(i, j int) bool {
				return jobs[i].Tags()[0] < jobs[j].Tags()[0]
			})
		}
		assert.Len(t, jobs, i+1)
		assert.Equal(t, task.name, jobs[i].Tags()[0])
		assert.Equal(t, task.schedule, jobs[i].Tags()[1])
	}
}

func TestScheduleHasSeconds(t *testing.T) {
	tests := []struct {
		schedule  string
		hasSecond bool
	}{
		{"* * * * * *", true},
		{"* * * * *", false},
		{"5 4 * * *", false},
		{"5,8 4 * * *", false},
		{"*   *   *  * * *", true},
		{"5,8 4   *  *   *", false},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			assert.Equal(t, test.hasSecond, scheduleHasSeconds(test.schedule))
		})
	}
}

func TestTaskGroupRunsSerially(t *testing.T) {
	var running, maxRunning atomic.Int32
	newTask := func(name string, opts ...TaskOption) *Task {
		task := &Task{Name: name, config: &BaseConfig{}, fun: func(context.Context, *user_model.User, Config) error {
			n := running.Add(1)
			defer running.Add(-1)
			maxRunning.Store(max(maxRunning.Load(), n))
			runtime.Gosched()
			return nil
		}}
		for _, opt := range opts {
			opt(task)
		}
		return task
	}

	var wg sync.WaitGroup
	for i := range 20 {
		task := newTask("grouped-"+strconv.Itoa(i), WithGroup("test-group"))
		wg.Go(task.Run)
	}
	wg.Wait()
	assert.Equal(t, int32(1), maxRunning.Load())
}
