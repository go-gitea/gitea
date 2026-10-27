// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"time"

	"gitea.dev/modules/gtprof"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
)

// ListPerfTraces lists the slow requests recorded by the built-in tracer, aggregated by route
func ListPerfTraces(ctx *context.APIContext) {
	// swagger:operation GET /admin/perf/traces admin adminListPerfTraces
	// ---
	// summary: List slow request traces aggregated by route
	// produces:
	// - application/json
	// parameters:
	// - name: min_duration
	//   in: query
	//   description: only include traces whose duration is at least this many milliseconds
	//   type: integer
	// - name: route
	//   in: query
	//   description: only include routes containing this text
	//   type: string
	// - name: exclude_long_running
	//   in: query
	//   description: exclude streams, git transfers and downloads which are slow by design
	//   type: boolean
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// - name: since
	//   in: query
	//   description: only include traces after this time (RFC 3339)
	//   type: string
	//   format: date-time
	// responses:
	//   "200":
	//     "$ref": "#/responses/PerfTraceRouteList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	q := gtprof.TraceQuery{
		MinDuration:        time.Duration(ctx.FormInt64("min_duration")) * time.Millisecond,
		Route:              ctx.FormString("route"),
		ExcludeLongRunning: ctx.FormBool("exclude_long_running"),
	}
	if since := ctx.FormString("since"); since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err != nil {
			ctx.APIError(http.StatusUnprocessableEntity, "invalid since, expect RFC 3339")
			return
		}
		q.Since = t
	}
	stats := gtprof.GetTraceRouteStats(q)
	listOpts := utils.GetListOptions(ctx)
	ctx.SetTotalCountHeader(int64(len(stats)))
	stats = util.PaginateSlice(stats, listOpts.Page, listOpts.PageSize)
	res := make([]*api.PerfTraceRoute, 0, len(stats))
	for _, s := range stats {
		res = append(res, convertPerfTraceRoute(s))
	}
	ctx.JSON(http.StatusOK, res)
}

func convertPerfTraceSpan(s *gtprof.TraceSpanRecord) *api.PerfTraceSpan {
	ret := &api.PerfTraceSpan{Name: s.Name, DurationMs: s.DurationMs, Attributes: s.Attributes}
	for _, c := range s.Children {
		ret.Children = append(ret.Children, convertPerfTraceSpan(c))
	}
	return ret
}

func convertPerfTraceRoute(r *gtprof.TraceRouteStat) *api.PerfTraceRoute {
	ret := &api.PerfTraceRoute{
		Route: r.Route, LongRunning: r.LongRunning, Count: r.Count, MaxDurationMs: r.MaxDurationMs,
		P95DurationMs: r.P95DurationMs, LastTime: r.LastTime, Records: make([]*api.PerfTraceRecord, 0, len(r.Records)),
	}
	for _, rec := range r.Records {
		ret.Records = append(ret.Records, &api.PerfTraceRecord{
			Time: rec.Time, Route: rec.Route, DurationMs: rec.DurationMs, LongRunning: rec.LongRunning,
			SQLCount: rec.SQLCount, SQLDurationMs: rec.SQLDurationMs, GitCount: rec.GitCount, GitDurationMs: rec.GitDurationMs,
			Root: convertPerfTraceSpan(rec.Root),
		})
	}
	return ret
}
