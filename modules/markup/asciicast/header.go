// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asciicast

import (
	"fmt"

	"code.gitea.io/gitea/modules/json"
)

type header struct {
	// Required header attributes:
	Version int `json:"version"`
	Width   int `json:"width"`
	Height  int `json:"height"`

	// Optional header attributes:
	Timestamp     int               `json:"timestamp"`
	Duration      float64           `json:"duration"`
	IdleTimeLimit float64           `json:"idle_time_limit"`
	Command       string            `json:"command"`
	Title         string            `json:"title"`
	Env           map[string]string `json:"env"`
	Theme         string            `json:"theme"`
}

func extractHeader(data []byte) (*header, error) {
	h := &header{}
	if err := json.Unmarshal(data, h); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	if h.Version != 2 {
		return nil, fmt.Errorf("unknown version: %v", h.Version)
	}
	if h.Width <= 0 {
		return nil, fmt.Errorf("invalid width: %v", h.Width)
	}
	if h.Height <= 0 {
		return nil, fmt.Errorf("invalid height: %v", h.Height)
	}
	return h, nil
}
