// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package agent

import "testing"

func TestNormalizeEnrollmentUsername(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "whoami at hostname", in: "clawdia@scpc2", want: "clawdia-scpc2"},
		{name: "trim and lower", in: "  AGENT_01@HOST-9  ", want: "agent_01-host-9"},
		{name: "replace spaces and symbols", in: "ai bot@my host!", want: "ai-bot-my-host"},
		{name: "leading separators removed", in: "..agent@host__", want: "agent-host"},
		{name: "very long", in: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", want: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{name: "empty", in: "   ", wantErr: true},
		{name: "only symbols", in: "@@@@###", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeEnrollmentUsername(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
