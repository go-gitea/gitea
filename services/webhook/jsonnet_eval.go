// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"gitea.dev/modules/util"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
)

// jsonnetRequestResult is the shape returned by RequestJsonnet programs.
type jsonnetRequestResult struct {
	Method             string            `json:"method"`
	URL                string            `json:"url"`
	Headers            map[string]string `json:"headers"`
	Body               json.RawMessage   `json:"body"`
	WithDefaultHeaders *bool             `json:"with_default_headers"`
}

func newJsonnetVM(ext map[string]any) (*jsonnet.VM, error) {
	vm := jsonnet.MakeVM()
	for k, v := range ext {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal extVar %s: %w", k, err)
		}
		vm.ExtCode(k, string(raw))
	}
	registerJsonnetNatives(vm)
	return vm, nil
}

func registerJsonnetNatives(vm *jsonnet.VM) {
	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "gitea_hmac_sha256",
		Params: ast.Identifiers{"secret", "message"},
		Func: func(params []any) (any, error) {
			secret, _ := params[0].(string)
			message, _ := params[1].(string)
			mac := hmac.New(sha256.New, []byte(secret))
			_, _ = mac.Write([]byte(message))
			return hex.EncodeToString(mac.Sum(nil)), nil
		},
	})
	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "gitea_sha256",
		Params: ast.Identifiers{"message"},
		Func: func(params []any) (any, error) {
			message, _ := params[0].(string)
			sum := sha256.Sum256([]byte(message))
			return hex.EncodeToString(sum[:]), nil
		},
	})
	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "gitea_uuid",
		Params: ast.Identifiers{},
		Func: func(params []any) (any, error) {
			return util.CryptoRandomString(32), nil
		},
	})
	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "gitea_legacy_payload",
		Params: ast.Identifiers{"hook_type", "event_type", "event", "meta", "webhook_url", "secret", "http_method", "content_type"},
		Func: func(params []any) (any, error) {
			return legacyPayloadNative(params)
		},
	})
	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "gitea_legacy_request",
		Params: ast.Identifiers{"hook_type", "event_type", "event", "meta", "webhook_url", "secret", "http_method", "content_type"},
		Func: func(params []any) (any, error) {
			return legacyRequestNative(params)
		},
	})
	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "gitea_url_query_escape",
		Params: ast.Identifiers{"s"},
		Func: func(params []any) (any, error) {
			s, _ := params[0].(string)
			return urlQueryEscape(s), nil
		},
	})
}

func evaluateJsonnet(snippet, filename string, ext map[string]any) (string, error) {
	vm, err := newJsonnetVM(ext)
	if err != nil {
		return "", err
	}
	out, err := vm.EvaluateAnonymousSnippet(filename, snippet)
	if err != nil {
		return "", fmt.Errorf("jsonnet %s: %w", filename, err)
	}
	return out, nil
}
