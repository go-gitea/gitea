// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform

import (
	"io"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"
	gojson "encoding/json" //nolint:depguard
)

var (
	ErrNoSerial           = util.NewInvalidArgumentErrorf("state serial is missing")
	ErrNoLineage          = util.NewInvalidArgumentErrorf("state lineage is missing")
	ErrNoTerraformVersion = util.NewInvalidArgumentErrorf("state terraform version is missing")
	ErrNoResources        = util.NewInvalidArgumentErrorf("state resources are missing")
	ErrNoOutputs          = util.NewInvalidArgumentErrorf("state outputs are missing")
)

type State struct {
	Version          int               `json:"version"`
	TerraformVersion string            `json:"terraform_version"`
	Serial           uint64            `json:"serial"`
	Lineage          string            `json:"lineage"`
	Resources        gojson.RawMessage `json:"resources"`
	Outputs          gojson.RawMessage `json:"outputs"`
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
		return nil, ErrNoSerial
	}
	if state.Version != 4 {
		return nil, util.NewInvalidArgumentErrorf("state version %d is not supported", state.Version)
	}
	if state.TerraformVersion == "" {
		return nil, ErrNoTerraformVersion
	}
	// Lineage should always be set
	if state.Lineage == "" {
		return nil, ErrNoLineage
	}
	if state.Resources == nil {
		return nil, ErrNoResources
	}
	if state.Outputs == nil {
		return nil, ErrNoOutputs
	}

	return &state, nil
}
