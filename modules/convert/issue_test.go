// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestLabel_ToLabel(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	label := models.AssertExistsAndLoadBean(t, &models.Label{ID: 1}).(*models.Label)
	assert.Equal(t, &api.Label{
		ID:    label.ID,
		Name:  label.Name,
		Color: "abcdef",
	}, ToLabel(label))
}
