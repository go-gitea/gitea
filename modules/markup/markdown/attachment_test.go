// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"testing"

	"code.gitea.io/gitea/modules/container"

	"github.com/stretchr/testify/assert"
)

func Test_ParseAttachments(t *testing.T) {
	attachments, err := ParseAttachments(`
# This is a test
![test](/attachments/08058e33-d916-432a-88b9-5f616cfe9f01)

## free

<img width="318" alt="image" src="/attachments/08058e33-d916-432a-88b9-5f616cfe9f00">
`)
	if err != nil {
		t.Fatal(err)
	}
	assert.EqualValues(t, container.Set[string]{
		"08058e33-d916-432a-88b9-5f616cfe9f01": struct{}{},
		"08058e33-d916-432a-88b9-5f616cfe9f00": struct{}{},
	}, attachments)
}
