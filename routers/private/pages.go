// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	pages_model "gitea.dev/models/pages"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	pages_content "gitea.dev/modules/pages"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	gitea_context "gitea.dev/services/context"
	pages_service "gitea.dev/services/pages"
)

type pagesInstanceContextKey struct{}

func pagesRoutes() *web.Router {
	r := web.NewRouter()
	r.AfterRouting(gitea_context.PrivateContexter())
	r.AfterRouting(authPagesInstance)
	r.Put("/instance", registerPagesInstance)
	r.Get("/sites", listPagesSites)
	r.Get("/sites/updated", listPagesSiteUpdates)
	r.Get("/sites/{repo-id}/content/{sha256}", downloadPagesContent)
	r.Post("/results", reportPagesResult)
	return r
}

func authPagesInstance(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		instance, err := pages_service.AuthenticateInstance(req.Context(), req.Header.Get("Authorization"))
		if err != nil {
			if !errors.Is(err, util.ErrNotExist) {
				log.Error("Pages instance authentication failed: %v", err)
			}
			ctx := gitea_context.GetPrivateContext(req)
			writePagesError(ctx, http.StatusForbidden, "invalid_instance_token")
			return
		}
		if err := pages_service.RecordInstanceCommunication(req.Context(), instance); err != nil {
			log.Error("Record Pages instance communication: %v", err)
		}
		ctx := gitea_context.GetPrivateContext(req)
		ctx.SetContextValue(pagesInstanceContextKey{}, instance)
		next.ServeHTTP(w, req)
	})
}

func pagesInstanceFromRequest(req *http.Request) *pages_model.Instance {
	instance, _ := req.Context().Value(pagesInstanceContextKey{}).(*pages_model.Instance)
	return instance
}

func writePagesJSON(ctx *gitea_context.PrivateContext, status int, value any) {
	ctx.Resp.Header().Set("Cache-Control", "no-store")
	ctx.JSON(status, value)
}

func writePagesError(ctx *gitea_context.PrivateContext, status int, code string) {
	writePagesJSON(ctx, status, struct {
		Code string `json:"code"`
	}{Code: code})
}

func decodePagesJSON(ctx *gitea_context.PrivateContext, target any) bool {
	ctx.Req.Body = http.MaxBytesReader(ctx.Resp, ctx.Req.Body, 64*1024)
	body, err := io.ReadAll(ctx.Req.Body)
	if err != nil || json.Unmarshal(body, target) != nil {
		writePagesError(ctx, http.StatusBadRequest, "invalid_json")
		return false
	}
	return true
}

func registerPagesInstance(ctx *gitea_context.PrivateContext) {
	var input struct {
		RootURL string `json:"root_url"`
	}
	if !decodePagesJSON(ctx, &input) {
		return
	}
	instance := pagesInstanceFromRequest(ctx.Req)
	registered, err := pages_service.RegisterInstanceRootURL(ctx, instance.ID, instance.TokenHash, input.RootURL)
	if err != nil {
		switch {
		case errors.Is(err, pages_service.ErrInvalidAddress):
			writePagesError(ctx, http.StatusBadRequest, "invalid_root_url")
		case errors.Is(err, pages_service.ErrDomainConflict):
			writePagesError(ctx, http.StatusConflict, "pages_domain_conflict")
		case errors.Is(err, util.ErrNotExist):
			writePagesError(ctx, http.StatusForbidden, "invalid_instance_token")
		default:
			log.Error("Register Pages instance: %v", err)
			writePagesError(ctx, http.StatusServiceUnavailable, "pages_registration_failed")
		}
		return
	}
	writePagesJSON(ctx, http.StatusOK, struct {
		RootURL             string `json:"root_url"`
		RootUpdatedUnixNano int64  `json:"root_updated_unix_nano"`
	}{RootURL: registered.RootURL, RootUpdatedUnixNano: registered.RootUpdatedUnixNano})
}

func parsePagesQueryInt(query url.Values, name string, defaultValue int64) (int64, bool) {
	values := query[name]
	if len(values) == 0 {
		return defaultValue, true
	}
	if len(values) != 1 || values[0] == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}

func listPagesSites(ctx *gitea_context.PrivateContext) {
	listPages(ctx, true)
}

func listPagesSiteUpdates(ctx *gitea_context.PrivateContext) {
	listPages(ctx, false)
}

func listPages(ctx *gitea_context.PrivateContext, full bool) {
	query := ctx.Req.URL.Query()
	if !full && query.Get("since") == "" {
		writePagesError(ctx, http.StatusBadRequest, "invalid_since")
		return
	}
	since, valid := parsePagesQueryInt(query, "since", 0)
	if !valid {
		writePagesError(ctx, http.StatusBadRequest, "invalid_since")
		return
	}
	until, valid := parsePagesQueryInt(query, "until", 0)
	if !valid {
		writePagesError(ctx, http.StatusBadRequest, "invalid_until")
		return
	}
	afterTime, valid := parsePagesQueryInt(query, "after_time", 0)
	if !valid {
		writePagesError(ctx, http.StatusBadRequest, "invalid_cursor")
		return
	}
	afterRepo, valid := parsePagesQueryInt(query, "after_repo_id", 0)
	if !valid || afterTime == 0 && afterRepo != 0 || afterTime > 0 && afterRepo == 0 {
		writePagesError(ctx, http.StatusBadRequest, "invalid_cursor")
		return
	}
	limit, valid := parsePagesQueryInt(query, "limit", 100)
	if !valid || limit == 0 || limit > pages_service.MaxSyncPageSize {
		writePagesError(ctx, http.StatusBadRequest, "invalid_limit")
		return
	}
	response, err := pages_service.ListSitesForSync(ctx, pagesInstanceFromRequest(ctx.Req), pages_service.SyncOptions{
		Full: full, Since: since, Until: until, AfterTime: afterTime, AfterRepo: afterRepo, Limit: int(limit),
	})
	if err != nil {
		switch {
		case errors.Is(err, pages_service.ErrCursorAhead):
			writePagesError(ctx, http.StatusConflict, "pages_cursor_ahead")
		default:
			log.Error("List Pages sites: %v", err)
			writePagesError(ctx, http.StatusServiceUnavailable, "pages_sync_failed")
		}
		return
	}
	writePagesJSON(ctx, http.StatusOK, response)
}

func downloadPagesContent(ctx *gitea_context.PrivateContext) {
	repoID, err := strconv.ParseInt(ctx.PathParam("repo-id"), 10, 64)
	digest := ctx.PathParam("sha256")
	if err != nil || repoID <= 0 {
		writePagesError(ctx, http.StatusBadRequest, "invalid_content_key")
		return
	}
	if _, err := pages_content.ContentRelativePath(repoID, digest); err != nil {
		writePagesError(ctx, http.StatusBadRequest, "invalid_content_key")
		return
	}
	object, size, err := pages_service.OpenCurrentContent(ctx, repoID, digest)
	if err != nil {
		switch {
		case errors.Is(err, util.ErrNotExist):
			writePagesError(ctx, http.StatusNotFound, "pages_content_not_found")
		case errors.Is(err, pages_service.ErrStateChanged):
			writePagesError(ctx, http.StatusConflict, "pages_state_changed")
		default:
			log.Error("Download Pages content: %v", err)
			writePagesError(ctx, http.StatusServiceUnavailable, "pages_content_unavailable")
		}
		return
	}
	defer object.Close()
	ctx.Resp.Header().Set("Content-Type", "application/gzip")
	ctx.Resp.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	ctx.Resp.Header().Set("Cache-Control", "no-store")
	ctx.Resp.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(ctx.Resp, object); err != nil {
		log.Error("Stream Pages content: %v", err)
	}
}

func reportPagesResult(ctx *gitea_context.PrivateContext) {
	var input pages_service.ResultRequest
	if !decodePagesJSON(ctx, &input) {
		return
	}
	instance := pagesInstanceFromRequest(ctx.Req)
	reportCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := pages_service.AcceptInstanceResult(reportCtx, instance.ID, instance.TokenHash, input)
	if err != nil {
		if errors.Is(err, pages_service.ErrStateChanged) {
			writePagesError(ctx, http.StatusConflict, "state_changed")
		} else if errors.Is(err, pages_service.ErrInvalidResult) {
			writePagesError(ctx, http.StatusBadRequest, "invalid_result")
		} else {
			log.Error("Report Pages result: %v", err)
			writePagesError(ctx, http.StatusServiceUnavailable, "pages_result_failed")
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}
