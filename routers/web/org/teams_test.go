// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org_test

import (
	"testing"

	org_model "gitea.dev/models/organization"
	"gitea.dev/models/perm"
	unit_model "gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	"gitea.dev/routers/web/org"
	"gitea.dev/services/context"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTeamDefaultUnits(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "org/org3/teams/new")
	contexttest.LoadUser(t, ctx, 2)
	ctx.ContextUser = unittest.AssertExistsAndLoadBean(t, &org_model.Organization{ID: 3}).AsUser()
	ctx.Org = &context.Organization{Organization: (*org_model.Organization)(ctx.ContextUser)}

	org.NewTeam(ctx)

	team, ok := ctx.Data["Team"].(*org_model.Team)
	require.True(t, ok)
	assert.Equal(t, perm.AccessModeRead, team.UnitAccessMode(ctx, unit_model.TypeCode))
	assert.Equal(t, perm.AccessModeNone, team.UnitAccessMode(ctx, unit_model.TypeSecurityAdvisories))
}
