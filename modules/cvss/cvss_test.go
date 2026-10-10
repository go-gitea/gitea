// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cvss

import (
	"testing"

	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	r, err := Parse("CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H")
	require.NoError(t, err)
	assert.Equal(t, Result{Version: Version31, Vector: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", Score: 9.8, Rating: "critical"}, *r)

	r, err = Parse(" CVSS:4.0/AV:L/AC:L/AT:N/PR:L/UI:P/VC:N/VI:H/VA:H/SC:N/SI:L/SA:L ")
	require.NoError(t, err)
	assert.Equal(t, Result{Version: Version40, Vector: "CVSS:4.0/AV:L/AC:L/AT:N/PR:L/UI:P/VC:N/VI:H/VA:H/SC:N/SI:L/SA:L", Score: 5.2, Rating: "medium"}, *r)

	for _, vector := range []string{
		"",
		"CVSS:3.0/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H",
		"CVSS:4.0/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
	} {
		_, err := Parse(vector)
		assert.ErrorIs(t, err, util.ErrInvalidArgument, vector)
	}
}
