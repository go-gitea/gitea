// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"os"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func GetDisplaySigningKey(key *git.SigningKey) string {
	if key == nil || key.Format == "" {
		return ""
	}

	switch key.Format {
	case git.SigningKeyFormatOpenPGP:
		return key.KeyID
	case git.SigningKeyFormatSSH:
		content, err := os.ReadFile(key.KeyID)
		if err != nil {
			log.Error("Unable to read SSH key %s: %v", key.KeyID, err)
			return "(Unable to read SSH key)"
		}
		display, err := CalcFingerprint(string(content))
		if err != nil {
			log.Error("Unable to calculate fingerprint for SSH key %s: %v", key.KeyID, err)
			return "(Unable to calculate fingerprint for SSH key)"
		}
		return display
	}
	setting.PanicInDevOrTesting("Unknown signing key format: %s", key.Format)
	return "(Unknown key format)"
}
