// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"bytes"
	stdctx "context"
	"encoding/json" //nolint:depguard // DisallowUnknownFields and duplicate-key checks need the stdlib decoder
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"gitea.dev/services/context"
	repo_service "gitea.dev/services/repository"
)

type copiedContentErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Index   *int   `json:"index,omitempty"`
}

// FinalizeCopiedContent verifies a published copy commit and associates LFS content.
func FinalizeCopiedContent(ctx *context.PrivateContext) {
	reqCtx, cancel := stdctx.WithTimeout(ctx, repo_service.CopiedContentRequestTimeout)
	defer cancel()
	body, err := readCopiedContentBody(reqCtx, ctx.Resp, ctx.Req)
	if err != nil {
		if reqCtx.Err() != nil || errors.Is(err, stdctx.DeadlineExceeded) {
			writeCopiedContentServiceError(ctx, err)
			return
		}
		writeCopiedContentError(ctx, http.StatusBadRequest, "invalid_request", "request body is invalid", nil)
		return
	}
	if int64(len(body)) > repo_service.CopiedContentMaxJSONBytes {
		ctx.Resp.Header().Set("Connection", "close")
		writeCopiedContentError(ctx, http.StatusRequestEntityTooLarge, "too_large", "request body exceeds the allowed size", nil)
		return
	}

	var req repo_service.FinalizeCopiedContentRequest
	if err := decodeCopiedContentJSON(body, &req); err != nil {
		writeCopiedContentError(ctx, http.StatusBadRequest, "invalid_request", "request body is invalid", nil)
		return
	}
	if req.Destination.Parent == nil && !jsonFieldPresent(body, "destination", "parent") {
		writeCopiedContentError(ctx, http.StatusBadRequest, "invalid_request", "destination.parent is required", nil)
		return
	}

	resp, err := repo_service.FinalizeCopiedContent(reqCtx, &req)
	if err != nil {
		writeCopiedContentServiceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, resp)
}

func writeCopiedContentServiceError(ctx *context.PrivateContext, err error) {
	var copied *repo_service.CopiedContentError
	if errors.As(err, &copied) {
		writeCopiedContentError(ctx, copied.Status, copied.Code, copied.Message, copied.Index)
		return
	}
	writeCopiedContentError(ctx, http.StatusServiceUnavailable, "unavailable", "service unavailable", nil)
}

func writeCopiedContentError(ctx *context.PrivateContext, status int, code, message string, index *int) {
	ctx.JSON(status, copiedContentErrorBody{
		Code:    code,
		Message: message,
		Index:   index,
	})
}

func decodeCopiedContentJSON(data []byte, dest any) error {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return errors.New("trailing json content")
	}
	return nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return rejectDuplicateJSONValue(dec)
}

func rejectDuplicateJSONValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	switch delim := tok.(type) {
	case json.Delim:
		switch delim {
		case '{':
			seen := make(map[string]struct{})
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return err
				}
				key, ok := keyTok.(string)
				if !ok {
					return errors.New("invalid json object key")
				}
				// Wire fields are ASCII lowercase; reject Unicode and case-folding aliases.
				if strings.IndexFunc(key, func(r rune) bool { return (r < 'a' || r > 'z') && r != '_' }) != -1 {
					return errors.New("noncanonical json field")
				}
				if _, exists := seen[key]; exists {
					return errors.New("duplicate json field")
				}
				seen[key] = struct{}{}
				if err := rejectDuplicateJSONValue(dec); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		case '[':
			for dec.More() {
				if err := rejectDuplicateJSONValue(dec); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		default:
			return errors.New("invalid json delimiter")
		}
	default:
		return nil
	}
}

func jsonFieldPresent(data []byte, path ...string) bool {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return jsonFieldPresentAt(dec, path)
}

func jsonFieldPresentAt(dec *json.Decoder, path []string) bool {
	tok, err := dec.Token()
	if err != nil {
		return false
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return false
	}
	switch delim {
	case '{':
		found := false
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return false
			}
			key, _ := keyTok.(string)
			if len(path) > 0 && key == path[0] {
				if len(path) == 1 {
					found = true
					if err := skipJSONValue(dec); err != nil {
						return false
					}
					continue
				}
				found = jsonFieldPresentAt(dec, path[1:])
				continue
			}
			if err := skipJSONValue(dec); err != nil {
				return false
			}
		}
		_, _ = dec.Token()
		return found
	case '[':
		for dec.More() {
			if err := skipJSONValue(dec); err != nil {
				return false
			}
		}
		_, _ = dec.Token()
		return false
	default:
		return false
	}
}

func skipJSONValue(dec *json.Decoder) error {
	return rejectDuplicateJSONValue(dec)
}

// A context alone cannot interrupt net/http's blocked request body read.
func readCopiedContentBody(ctx stdctx.Context, writer http.ResponseWriter, req *http.Request) ([]byte, error) {
	// Gitea's wrapper predates ResponseController and does not expose Unwrap.
	for {
		response, ok := writer.(*context.Response)
		if !ok {
			break
		}
		writer = response.ResponseWriter
	}
	controller := http.NewResponseController(writer)
	deadline, _ := ctx.Deadline()
	err := controller.SetReadDeadline(deadline)
	if err != nil && !errors.Is(err, http.ErrNotSupported) {
		return nil, err
	}
	supportsDeadline := err == nil
	completed := false
	done := make(chan struct{})
	stop := stdctx.AfterFunc(ctx, func() {
		defer close(done)
		if supportsDeadline {
			_ = controller.SetReadDeadline(time.Now())
		} else {
			// In-memory handlers have no connection deadline; Close releases body readers.
			_ = req.Body.Close()
		}
	})
	defer func() {
		if !stop() {
			<-done
		}
		if supportsDeadline && completed {
			_ = controller.SetReadDeadline(time.Time{})
		}
	}()
	body, err := io.ReadAll(io.LimitReader(req.Body, repo_service.CopiedContentMaxJSONBytes+1))
	completed = err == nil
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return nil, stdctx.DeadlineExceeded
	}
	return body, err
}
