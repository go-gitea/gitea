package asymkey

import (
	"os"

	"code.gitea.io/gitea/modules/git"
)

func GetDisplaySigningKey(key *git.SigningKey) (string, error) {
	if key != nil {
		switch key.Format {
		case git.SigningKeyFormatOpenPGP:
			return key.KeyID, nil
		case git.SigningKeyFormatSSH:
			content, readErr := os.ReadFile(key.KeyID)
			if readErr != nil {
				return "", readErr
			}
			return CalcFingerprint(string(content))
		}
	}
	return "", nil
}
