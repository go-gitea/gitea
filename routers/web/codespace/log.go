// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"

	"gitea.dev/modules/httplib"
	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
)

type logErrorResponse struct {
	Category      string `json:"category"`
	CurrentOffset int64  `json:"current_offset,omitempty"`
}

// Logs returns one byte-offset based Codespace log page for the creator.
func Logs(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	offset, ok := parseOptionalInt64Query(ctx, "offset", 0)
	if !ok {
		writeLogError(ctx, http.StatusBadRequest, "invalid_argument", 0)
		return
	}
	limit, ok := parseOptionalInt64Query(ctx, "limit", codespace_service.LogReadMaxBytes)
	if !ok {
		writeLogError(ctx, http.StatusBadRequest, "invalid_argument", 0)
		return
	}
	result, err := codespace_service.ReadLog(ctx, codespace_service.ReadLogOptions{
		UserID:        ctx.Doer.ID,
		CodespaceUUID: ctx.PathParam("uuid"),
		Offset:        offset,
		Limit:         limit,
	})
	if err != nil {
		var offsetErr *codespace_service.LogOffsetError
		if errors.As(err, &offsetErr) {
			if errors.Is(err, codespace_service.ErrReadLogOffsetConflict) {
				writeLogError(ctx, http.StatusConflict, "offset_conflict", offsetErr.CurrentOffset)
				return
			}
			if errors.Is(err, codespace_service.ErrReadLogInvalidArgument) {
				writeLogError(ctx, http.StatusBadRequest, "invalid_argument", offsetErr.CurrentOffset)
				return
			}
		}
		switch {
		case errors.Is(err, codespace_service.ErrReadLogInvalidArgument):
			writeLogError(ctx, http.StatusBadRequest, "invalid_argument", 0)
		case errors.Is(err, codespace_service.ErrReadLogPermissionDenied):
			writeLogError(ctx, http.StatusForbidden, "permission_denied", 0)
		case errors.Is(err, codespace_service.ErrReadLogNotFound):
			writeLogError(ctx, http.StatusNotFound, "codespace_not_found", 0)
		default:
			ctx.ServerError("ReadLog", err)
		}
		return
	}
	ctx.RespHeader().Set("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, result)
}

// DownloadLogs downloads the creator's Codespace log as plain text.
func DownloadLogs(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	codespaceUUID := ctx.PathParam("uuid")
	var buf bytes.Buffer
	var offset int64
	for {
		result, err := codespace_service.ReadLog(ctx, codespace_service.ReadLogOptions{
			UserID:        ctx.Doer.ID,
			CodespaceUUID: codespaceUUID,
			Offset:        offset,
			Limit:         codespace_service.LogReadMaxBytes,
		})
		if err != nil {
			handleDownloadLogError(ctx, err)
			return
		}
		for _, line := range result.Lines {
			buf.WriteString(line)
		}
		if result.EOF {
			break
		}
		offset = result.NextOffset
	}

	ctx.RespHeader().Set("Cache-Control", "no-store")
	ctx.RespHeader().Set("Content-Type", "text/plain; charset=utf-8")
	ctx.RespHeader().Set("Content-Disposition", httplib.EncodeContentDispositionAttachment(codespaceUUID+".log"))
	ctx.Resp.WriteHeader(http.StatusOK)
	_, _ = ctx.Resp.Write(buf.Bytes())
}

func handleDownloadLogError(ctx *context.Context, err error) {
	switch {
	case errors.Is(err, codespace_service.ErrReadLogPermissionDenied):
		ctx.PlainText(http.StatusForbidden, "permission_denied")
	case errors.Is(err, codespace_service.ErrReadLogNotFound):
		ctx.NotFound(nil)
	case errors.Is(err, codespace_service.ErrReadLogInvalidArgument), errors.Is(err, codespace_service.ErrReadLogOffsetConflict):
		ctx.PlainText(http.StatusConflict, "offset_conflict")
	default:
		ctx.ServerError("ReadLog", err)
	}
}

func parseOptionalInt64Query(ctx *context.Context, name string, def int64) (int64, bool) {
	raw := ctx.Req.URL.Query().Get(name)
	if raw == "" {
		return def, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	return value, err == nil
}

func writeLogError(ctx *context.Context, status int, category string, currentOffset int64) {
	ctx.RespHeader().Set("Cache-Control", "no-store")
	ctx.JSON(status, logErrorResponse{Category: category, CurrentOffset: currentOffset})
}
