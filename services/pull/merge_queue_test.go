// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeQueueRequiredContexts(t *testing.T) {
	assert.Equal(t,
		[]string{"CI / test (merge_group)", "CI / lint (merge_group)", "CI / *", "external"},
		MergeQueueRequiredContexts([]string{"CI / test (pull_request)", "CI / lint (pull_request_target)", "CI / *", "external"}),
	)
}
