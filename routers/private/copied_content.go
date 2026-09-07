// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"bytes"
	stdctx "context"
	"encoding/json" //nolint:depguard // DisallowUnknownFields and duplicate-key checks need the stdlib decoder
	"errors"
	"io"
	"net/http"

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
	limited := io.LimitReader(ctx.Req.Body, repo_service.CopiedContentMaxJSONBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		writeCopiedContentError(ctx, http.StatusBadRequest, "invalid_request", "request body is invalid", nil)
		return
	}
	if int64(len(body)) > repo_service.CopiedContentMaxJSONBytes {
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

	reqCtx, cancel := stdctx.WithTimeout(ctx, repo_service.CopiedContentRequestTimeout)
	defer cancel()

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
	if dec.More() {
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
