// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func ReservedMigrationSlot(_ context.Context, x base.EngineMigration) error {
	// no-op.
	// The "Unwrap ldap.Sources" migration was dropped, and the migration that followed it
	// was renumbered down into its place, leaving a gap at 190. Keep this slot occupied so
	// the migration IDs stay contiguous and already-migrated databases keep their version.

	return nil
}
