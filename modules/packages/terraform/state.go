// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform

import (
	"io"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"
)

// Note: this is a subset of the Terraform state file format as the full one has two forms.
// If needed, it can be expanded in the future.

type State struct {
	Serial  uint64 `json:"serial"`
	Lineage string `json:"lineage"`
}

// ParseState parses the required parts of Terraform state file
func ParseState(r io.Reader) (*State, error) {
	var state State
	err := json.NewDecoder(r).Decode(&state)
	if err != nil {
		return nil, err
	}
	// Serial starts at 1; 0 means it wasn't set in the state file
	if state.Serial == 0 {
		return nil, util.NewInvalidArgumentErrorf("state serial is missing")
	}
	// Lineage should always be set
	if state.Lineage == "" {
		return nil, util.NewInvalidArgumentErrorf("state lineage is missing")
	}

	return &state, nil
}
