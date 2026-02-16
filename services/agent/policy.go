// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"context"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
)

const (
	// SettingOwnerAgent marks a bot account as allowed to invite other agents.
	SettingOwnerAgent = "agent.owner_inviter"
	// SettingMachineIdentity stores the external machine identity provided during enrollment.
	SettingMachineIdentity = "agent.machine_identity"
	// SettingNetworkIdentifier stores the external network identifier provided during enrollment.
	SettingNetworkIdentifier = "agent.network_identifier"
	// SettingRequestedUsername stores raw username input before normalization.
	SettingRequestedUsername = "agent.requested_username"
)

// IsAgent returns whether the user should be treated as an agent principal.
func IsAgent(u *user_model.User) bool {
	return u != nil && u.IsTypeBot()
}

// IsOwnerAgent returns whether this agent has invite capability.
func IsOwnerAgent(ctx context.Context, u *user_model.User) (bool, error) {
	if !IsAgent(u) {
		return false, nil
	}
	v, err := user_model.GetUserSetting(ctx, u.ID, SettingOwnerAgent, "false")
	if err != nil {
		return false, err
	}
	return strings.EqualFold(v, "true") || v == "1", nil
}
