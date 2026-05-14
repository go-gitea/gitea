// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArtifactDownloadSignatureSeparatesFields(t *testing.T) {
	testCases := []struct {
		name  string
		left  []byte
		right []byte
		sigA  []byte
		sigB  []byte
	}{
		{
			name:  "endpoint-length-is-part-of-payload",
			left:  buildArtifactSignaturePayload("artifact/archive", 12, 345),
			right: buildArtifactSignaturePayload("artifact/archive1", 2, 345),
			sigA:  buildSignature("artifact/archive", 12, 345),
			sigB:  buildSignature("artifact/archive1", 2, 345),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEqual(t, tc.left, tc.right)
			assert.NotEqual(t, tc.sigA, tc.sigB)
		})
	}
}
