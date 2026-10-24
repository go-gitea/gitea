// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	"gitea.dev/modules/gtprof"
)

// PerfTraceRouteList
// swagger:response PerfTraceRouteList
type swaggerResponsePerfTraceRouteList struct {
	// in:body
	Body []gtprof.TraceRouteStat `json:"body"`
}
