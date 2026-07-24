// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// CreateUserOption create user options
// swagger:model
type CreateAuthOauth2Option struct {
	AuthenticationName       string `json:"authentication_name" binding:"Required"`
	ProviderIconURL          string `json:"provider_icon_url"`
	ProviderClientID         string `json:"provider_client_id" binding:"Required"`
	ProviderClientSecret     string `json:"provider_client_secret" binding:"Required"`
	ProviderAutoDiscoveryURL string `json:"provider_auto_discovery_url" binding:"Required"`

	SkipLocal2FA       bool   `json:"skip_local_2fa"`
	AdditionalScopes   string `json:"additional_scopes"`
	RequiredClaimName  string `json:"required_claim_name"`
	RequiredClaimValue string `json:"required_claim_value"`

	ClaimNameProvidingGroupNameForSource string `json:"claim_name_providingGroupNameForSource"`
	GroupClaimValueForAdministratorUsers string `json:"group_claim_value_for_administrator_users"`
	GroupClaimValueForRestrictedUsers    string `json:"group_claim_value_for_restricted_users"`
	MapClaimedGroupsToOrganizationTeams  string `json:"map_claimed_groups_to_organization_teams"`

	RemoveUsersFromSyncronizedTeams bool `json:"RemoveUsersFromSyncronizedTeams"`
	EnableUserSyncronization        bool `json:"EnableUserSyncronization"`
	AuthenticationSourceIsActive    bool `json:"AuthenticationSourceIsActive"`
}

// EditUserOption edit user options
// swagger:model
type EditAuthOauth2Option struct {
	AuthenticationName       string `json:"authentication_name" binding:"Required"`
	ProviderIconURL          string `json:"provider_icon_url"`
	ProviderClientID         string `json:"provider_client_id" binding:"Required"`
	ProviderClientSecret     string `json:"provider_client_secret" binding:"Required"`
	ProviderAutoDiscoveryURL string `json:"provider_auto_discovery_url" binding:"Required"`

	SkipLocal2FA       bool   `json:"skip_local_2fa"`
	AdditionalScopes   string `json:"additional_scopes"`
	RequiredClaimName  string `json:"required_claim_name"`
	RequiredClaimValue string `json:"required_claim_value"`

	ClaimNameProvidingGroupNameForSource string `json:"claim_name_providingGroupNameForSource"`
	GroupClaimValueForAdministratorUsers string `json:"group_claim_value_for_administrator_users"`
	GroupClaimValueForRestrictedUsers    string `json:"group_claim_value_for_restricted_users"`
	MapClaimedGroupsToOrganizationTeams  string `json:"map_claimed_groups_to_organization_teams"`

	RemoveUsersFromSyncronizedTeams bool `json:"RemoveUsersFromSyncronizedTeams"`
	EnableUserSyncronization        bool `json:"EnableUserSyncronization"`
	AuthenticationSourceIsActive    bool `json:"AuthenticationSourceIsActive"`
}
