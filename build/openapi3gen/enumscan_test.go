// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package openapi3gen

import "testing"

func TestEnumKey_sortsAndJoins(t *testing.T) {
	key := EnumKey([]any{"b", "a", "c"})
	if key != "a|b|c" {
		t.Fatalf("EnumKey = %q, want %q", key, "a|b|c")
	}
}

func TestEnumKey_handlesNonStringValues(t *testing.T) {
	key := EnumKey([]any{2, 1, 3})
	if key != "1|2|3" {
		t.Fatalf("EnumKey = %q, want %q", key, "1|2|3")
	}
}
