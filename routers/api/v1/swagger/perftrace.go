// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "gitea.dev/modules/structs"
)

// PerfTraceRouteList
// swagger:response PerfTraceRouteList
type swaggerResponsePerfTraceRouteList struct {
	// in:body
	Body []api.PerfTraceRoute `json:"body"`
}
