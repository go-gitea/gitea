// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"io"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/modules/json"
)

// WriteEventsAsJSON writes one JSON object per line.
func WriteEventsAsJSON(w io.Writer, events []*audit_model.Event) error {
	encoder := json.NewEncoder(w)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}
	return nil
}
