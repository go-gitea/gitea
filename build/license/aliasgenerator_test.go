// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package license

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLicenseNameFromAliases(t *testing.T) {
	tests := []struct {
		target string
		inputs []string
	}{
		{
			// real case which you can find in license-aliases.json
			target: "AGPL-1.0",
			inputs: []string{
				"AGPL-1.0-only",
				"AGPL-1.0-or-late",
			},
		},
		{
			target: "",
			inputs: []string{
				"APSL-1.0",
				"AGPL-1.0-only",
				"AGPL-1.0-or-late",
			},
		},
	}

	for _, tt := range tests {
		result := GetLicenseNameFromAliases(tt.inputs)
		assert.Equal(t, result, tt.target)
	}
}
