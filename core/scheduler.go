package core

import (
	"context"
	"errors"

	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
)

var ErrDataLock = errors.New("Data Lock Error")

type Filter struct {
	Kind   string
	Type   string
	OS     string
	Arch   string
	Kernel string
}

// Scheduler schedules Build stages for execution.
type Scheduler interface {
	// Schedule schedules the stage for execution.
	Schedule(context.Context, *runnerv1.Task) error

	// Request requests the next stage scheduled for execution.
	Request(context.Context, Filter) (*runnerv1.Task, error)
}
