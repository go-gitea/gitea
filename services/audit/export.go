// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"io"

	"gitea.dev/modules/json"
)

// WriteEventsAsJSON writes one JSON object per line.
func WriteEventsAsJSON(w io.Writer, events []*Event) error {
	encoder := json.NewEncoder(w)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}
	return nil
}
