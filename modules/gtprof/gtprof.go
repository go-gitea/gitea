// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gtprof

import (
	"sync/atomic"
	"time"
)

// This is a Gitea-specific profiling package,
// the name is chosen to distinguish it from the standard pprof tool and "GNU gprof"

// LabelGracefulLifecycle is a label marking manager lifecycle phase
// Making it compliant with prometheus key regex https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
// would enable someone interested to be able to continuously gather profiles into pyroscope.
// Other labels for pprof should also follow this rule.
const LabelGracefulLifecycle = "graceful_lifecycle"

// LabelPid is a label set on goroutines that have a process attached
const LabelPid = "pid"

// LabelPpid is a label set on goroutines that have a process attached
const LabelPpid = "ppid"

// LabelProcessType is a label set on goroutines that have a process attached
const LabelProcessType = "process_type"

// LabelProcessDescription is a label set on goroutines that have a process attached
const LabelProcessDescription = "process_description"

type TracerOptions struct {
	ServiceName, AppVer string
	BuiltinThreshold    time.Duration
}

var tracerOptions atomic.Pointer[TracerOptions]

func EnableTracer(opts *TracerOptions) {
	tracerOptions.Store(opts)
}
