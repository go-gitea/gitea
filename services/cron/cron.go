// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cron

import (
	"context"
	"runtime/pprof"
	"time"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/sync"

	"github.com/gogs/cron"
)

var c = cron.New()

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

	lock.Lock()
	for _, task := range tasks {
		if task.IsEnabled() && task.DoRunAtStart() {
			go task.Run()
		}
	}

	c.Start()
	started = true
	lock.Unlock()
	graceful.GetManager().RunAtShutdown(context.Background(), func() {
		c.Stop()
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

func (t *TaskTableRow) FormatLastMessage(locale string) string {
	if t.Status == "finished" {
		return t.task.GetConfig().FormatMessage(locale, t.Name, t.Status, t.LastDoer)
	}

	return t.task.GetConfig().FormatMessage(locale, t.Name, t.Status, t.LastDoer, t.LastMessage)
}

// TaskTable represents a table of tasks
type TaskTable []*TaskTableRow

// ListTasks returns all running cron tasks.
func ListTasks() TaskTable {
	entries := c.Entries()
	eMap := map[string]*cron.Entry{}
	for _, e := range entries {
		eMap[e.Description] = e
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
		if e, ok := eMap[task.Name]; ok {
			spec = e.Spec
			next = e.Next
			prev = e.Prev
		}
		task.lock.Lock()
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
