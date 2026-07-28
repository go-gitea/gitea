// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import "gitea.dev/modelmigration/base"

func UseBase32HexForCredIDInWebAuthnCredential(x base.EngineMigration) error {
	// noop
	return nil
}
