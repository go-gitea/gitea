// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"bytes"

	"code.gitea.io/gitea/modules/json"
)

type JsonUtils struct{} //nolint:revive // variable naming triggers on Json, wants JSON

var jsonUtils = JsonUtils{}

func NewJsonUtils() *JsonUtils { //nolint:revive // variable naming triggers on Json, wants JSON
	return &jsonUtils
}

func (su *JsonUtils) EncodeToString(v any) string {
	out, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(out)
}

func (su *JsonUtils) PrettyIndent(s string) string {
	var out bytes.Buffer
	err := json.Indent(&out, []byte(s), "", "  ")
	if err != nil {
		return ""
	}
	return out.String()
}
