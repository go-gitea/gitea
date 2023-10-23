// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"regexp"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	offsetStringRegEx = "^(\\+|-)?\\d{2}:\\d{2}$"
)

func TestAPIListTimezones(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	require.NoError(t, unittest.LoadFixtures())

	resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/timezones"), http.StatusOK)

	var timezoneList []api.TimeZone
	DecodeJSON(t, resp, &timezoneList)

	assert.NotEmpty(t, timezoneList)

	re, err := regexp.Compile(offsetStringRegEx) //nolint:gocritic
	require.NoError(t, err)

	for _, timezone := range timezoneList {
		assert.NotEmpty(t, timezone.Name)
		assert.NotEqual(t, 0, timezone.Offset)
		assert.NotEmpty(t, timezone.CurrentTime)
		assert.True(t, re.MatchString(timezone.OffsetString))
	}
}
