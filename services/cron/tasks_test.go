// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddTaskToScheduler(t *testing.T) {
	assert.Len(t, scheduler.Jobs(), 0)
	defer scheduler.Clear()

	// no seconds
	err := addTaskToScheduler(&Task{
		Name: "task 1",
		config: &BaseConfig{
			Schedule: "5 4 * * *",
		},
	})
	assert.NoError(t, err)
	assert.Len(t, scheduler.Jobs(), 1)
	assert.Equal(t, "task 1", scheduler.Jobs()[0].Tags()[0])
	assert.Equal(t, "5 4 * * *", scheduler.Jobs()[0].Tags()[1])

	// with seconds
	err = addTaskToScheduler(&Task{
		Name: "task 2",
		config: &BaseConfig{
			Schedule: "30 5 4 * * *",
		},
	})
	assert.NoError(t, err)
	assert.Len(t, scheduler.Jobs(), 2)
	assert.Equal(t, "task 2", scheduler.Jobs()[1].Tags()[0])
	assert.Equal(t, "30 5 4 * * *", scheduler.Jobs()[1].Tags()[1])
}

func TestScheduleHasSeconds(t *testing.T) {
	tests := []struct {
		schedule  string
		hasSecond bool
	}{
		{"* * * * * *", true},
		{"* * * * *", false},
		{"5 4 * * *", false},
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
