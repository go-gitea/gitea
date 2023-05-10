// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseRemoteUpdateOutput(t *testing.T) {
	output := `
* [new tag]         v0.1.8     -> v0.1.8
* [new branch]      master     -> origin/master
- [deleted]         (none)     -> origin/test
+ f895a1e...957a993 test       -> origin/test  (forced update)
`
	results := parseRemoteUpdateOutput(output, "origin")
	assert.Len(t, results, 4)
	assert.EqualValues(t, "refs/tag/v0.1.8", results[0].refName)
}
