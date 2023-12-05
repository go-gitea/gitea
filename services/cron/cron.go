// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"context"
	"runtime/pprof"
	"time"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/sync"
	"code.gitea.io/gitea/modules/translation"

	"github.com/go-co-op/gocron"
)

var scheduler = gocron.NewScheduler(time.Local)

// Prevent duplicate running tasks.
var taskStatusTable = sync.NewStatusTable()

// NewContext begins cron tasks
// Each cron task is run within the shutdown context as a running server
// AtShutdown the cron server is stopped
func NewContext(original context.Context) {
	defer pprof.SetGoroutineLabels(original)
	_, _, finished := process.GetManager().AddTypedContext(graceful.GetManager().ShutdownContext(), "Service: Cron", process.SystemProcessType, true)
	initBasicTasks()
	initExtendedTasks()
	initActionsTasks()

	lock.Lock()
	for _, task := range tasks {
		if task.IsEnabled() && task.DoRunAtStart() {
			go task.Run()
		}
	}

	scheduler.StartAsync()
	started = true
	lock.Unlock()
	graceful.GetManager().RunAtShutdown(context.Background(), func() {
		scheduler.Stop()
		lock.Lock()
		started = false
		lock.Unlock()
		finished()
	})
}

// TaskTableRow represents a task row in the tasks table
type TaskTableRow struct {
	Name        string
	Spec        string
	Next        time.Time
	Prev        time.Time
	Status      string
	LastMessage string
	LastDoer    string
	ExecTimes   int64
	task        *Task
}

func (t *TaskTableRow) FormatLastMessage(locale translation.Locale) string {
	if t.Status == "finished" {
		return t.task.GetConfig().FormatMessage(locale, t.Name, t.Status, t.LastDoer)
	}

	return t.task.GetConfig().FormatMessage(locale, t.Name, t.Status, t.LastDoer, t.LastMessage)
}

// TaskTable represents a table of tasks
type TaskTable []*TaskTableRow

// ListTasks returns all running cron tasks.
func ListTasks() TaskTable {
	jobs := scheduler.Jobs()
	jobMap := map[string]*gocron.Job{}
	for _, job := range jobs {
		// the first tag is the task name
		tags := job.Tags()
		if len(tags) == 0 { // should never happen
			continue
		}
		jobMap[job.Tags()[0]] = job
	}

	lock.Lock()
	defer lock.Unlock()

	tTable := make([]*TaskTableRow, 0, len(tasks))
	for _, task := range tasks {
		spec := "-"
		var (
			next time.Time
			prev time.Time
		)
		if e, ok := jobMap[task.Name]; ok {
			tags := e.Tags()
			if len(tags) > 1 {
				spec = tags[1] // the second tag is the task spec
			}
			next = e.NextRun()
			prev = e.PreviousRun()
		}

		task.lock.Lock()
		// If the manual run is after the cron run, use that instead.
		if prev.Before(task.LastRun) {
			prev = task.LastRun
		}
		tTable = append(tTable, &TaskTableRow{
			Name:        task.Name,
			Spec:        spec,
			Next:        next,
			Prev:        prev,
			ExecTimes:   task.ExecTimes,
			LastMessage: task.LastMessage,
			Status:      task.Status,
			LastDoer:    task.LastDoer,
			task:        task,
		})
		task.lock.Unlock()
	}

	return tTable
}
