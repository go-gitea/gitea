// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArtifactV4SignatureSeparatesFields(t *testing.T) {
	routes := &artifactV4Routes{}
	testCases := []struct {
		name  string
		left  []byte
		right []byte
		sigA  []byte
		sigB  []byte
	}{
		{
			name:  "payload-boundaries",
			left:  buildArtifactV4SignaturePayload("download-bundle", "2026-05-14T10", "report", 34, 567),
			right: buildArtifactV4SignaturePayload("download", "-bundle2026-05-14T10", "report3", 4, 567),
			sigA:  routes.buildSignature("download-bundle", "2026-05-14T10", "report", 34, 567),
			sigB:  routes.buildSignature("download", "-bundle2026-05-14T10", "report3", 4, 567),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEqual(t, tc.left, tc.right)
			assert.NotEqual(t, tc.sigA, tc.sigB)
		})
	}
}
