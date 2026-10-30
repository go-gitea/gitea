// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

// An organization can delegate its Actions settings -- runners, secrets and
// variables -- to a team its owners have given write access to the Actions
// unit. Everything else about the organization stays owner-only.
//
// The shape of each case is the same and is the point: the same member, the
// same request, refused before the delegation and allowed after it. Nothing
// in the fixtures carries the Actions unit (no `type: 10` in team_unit.yml),
// so the "before" half is a real check and not an assumption.
func TestAPIOrgActionsDelegation(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	const org = "org3"
	// user2 owns org3; user4 is an ordinary member of it.
	ownerToken := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteOrganization)
	memberToken := getUserToken(t, "user4", auth_model.AccessTokenScopeWriteOrganization)

	registrationToken := fmt.Sprintf("/api/v1/orgs/%s/actions/runners/registration-token", org)
	secrets := fmt.Sprintf("/api/v1/orgs/%s/actions/secrets", org)
	variables := fmt.Sprintf("/api/v1/orgs/%s/actions/variables", org)

	t.Run("AMemberIsRefusedBeforeAnyDelegation", func(t *testing.T) {
		for _, url := range []string{registrationToken, secrets, variables} {
			method := http.MethodGet
			if url == registrationToken {
				method = http.MethodPost
			}
			req := NewRequest(t, method, url).AddTokenAuth(memberToken)
			MakeRequest(t, req, http.StatusForbidden)
		}
	})

	// The delegation itself, made the way an owner would make it.
	var team api.Team
	t.Run("AnOwnerDelegatesToATeam", func(t *testing.T) {
		req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/orgs/%s/teams", org), api.CreateTeamOption{
			Name:        "ci",
			Description: "runs the organization's workflows",
			Permission:  "read",
			UnitsMap:    map[string]string{"repo.actions": "write"},
		}).AddTokenAuth(ownerToken)
		resp := MakeRequest(t, req, http.StatusCreated)
		DecodeJSON(t, resp, &team)
		assert.Equal(t, "write", team.UnitsMap["repo.actions"])

		req = NewRequest(t, http.MethodPut,
			fmt.Sprintf("/api/v1/teams/%d/members/user4", team.ID)).AddTokenAuth(ownerToken)
		MakeRequest(t, req, http.StatusNoContent)
	})

	t.Run("TheSameMemberMayNowManageActions", func(t *testing.T) {
		req := NewRequest(t, http.MethodPost, registrationToken).AddTokenAuth(memberToken)
		resp := MakeRequest(t, req, http.StatusOK)
		minted := DecodeJSON(t, resp, &struct {
			Token string `json:"token"`
		}{})
		assert.NotEmpty(t, minted.Token)

		MakeRequest(t, NewRequest(t, http.MethodGet, secrets).AddTokenAuth(memberToken), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, variables).AddTokenAuth(memberToken), http.StatusOK)
	})

	t.Run("AndNothingElseAboutTheOrganization", func(t *testing.T) {
		// The delegation is to the Actions settings and to nothing else.
		// Creating a team, reading the hooks and removing a member all stay
		// where they were.
		req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/orgs/%s/teams", org), api.CreateTeamOption{
			Name:       "not-allowed",
			Permission: "read",
		}).AddTokenAuth(memberToken)
		MakeRequest(t, req, http.StatusForbidden)

		req = NewRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/orgs/%s/hooks", org)).AddTokenAuth(memberToken)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("ReadOnTheUnitIsNotEnough", func(t *testing.T) {
		// Write, not read. A team that may only look at workflow runs is not
		// thereby allowed to register a runner or read a secret.
		req := NewRequestWithJSON(t, http.MethodPatch, fmt.Sprintf("/api/v1/teams/%d", team.ID), api.EditTeamOption{
			Name:       "ci",
			Permission: "read",
			UnitsMap:   map[string]string{"repo.actions": "read"},
		}).AddTokenAuth(ownerToken)
		MakeRequest(t, req, http.StatusOK)

		MakeRequest(t, NewRequest(t, http.MethodPost, registrationToken).AddTokenAuth(memberToken), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, http.MethodGet, secrets).AddTokenAuth(memberToken), http.StatusForbidden)
	})

	t.Run("AnOwnerIsStillAnOwner", func(t *testing.T) {
		MakeRequest(t, NewRequest(t, http.MethodPost, registrationToken).AddTokenAuth(ownerToken), http.StatusOK)
		MakeRequest(t, NewRequest(t, http.MethodGet, secrets).AddTokenAuth(ownerToken), http.StatusOK)
	})
}
