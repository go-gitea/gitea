// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform

import (
	"io"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"
)

type State struct {
	Version          int        `json:"version"`
	TerraformVersion string     `json:"terraform_version"`
	Serial           uint64     `json:"serial"`
	Lineage          string     `json:"lineage"`
	Resources        json.Value `json:"resources"`
	Outputs          json.Value `json:"outputs"`
}

// ParseState parses the Terraform state file
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
	if state.Version != 4 {
		return nil, util.NewInvalidArgumentErrorf("state version %d is not supported", state.Version)
	}
	if state.TerraformVersion == "" {
		return nil, util.NewInvalidArgumentErrorf("state terraform version is missing")
	}
	// Lineage should always be set
	if state.Lineage == "" {
		return nil, util.NewInvalidArgumentErrorf("state lineage is missing")
	}
	if state.Resources == nil {
		return nil, util.NewInvalidArgumentErrorf("state resources are missing")
	}
	if state.Outputs == nil {
		return nil, util.NewInvalidArgumentErrorf("state outputs are missing")
	}

	return &state, nil
}
