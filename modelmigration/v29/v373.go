// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"
	"math/big"
	"strings"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/log"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
)

// recertifyPublicKey re-signs the user ID self-signature if keybase/go-crypto (before 1.23.2) zero-padded its MPI, which Sequoia rejects
func recertifyPublicKey(privateKey string) (string, error) {
	entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(privateKey))
	if err != nil {
		return "", err
	}
	entity := entities[0]
	identity := entity.PrimaryIdentity()
	signature := identity.SelfSignature.RSASignature
	if int(signature.BitLength()) == new(big.Int).SetBytes(signature.Bytes()).BitLen() {
		return "", nil
	}
	if err := identity.SelfSignature.SignUserId(identity.Name, entity.PrimaryKey, entity.PrivateKey, nil); err != nil {
		return "", err
	}

	var publicKey strings.Builder
	armorWriter, _ := armor.Encode(&publicKey, openpgp.PublicKeyType, nil)
	if err := entity.Serialize(armorWriter); err != nil {
		return "", err
	}
	armorWriter.Close()
	return publicKey.String(), nil
}

func RepairPackageSigningKeys(_ctx context.Context, x base.EngineMigration) error {
	type UserSetting struct {
		UserID       int64
		SettingKey   string
		SettingValue string
	}

	var privateKeys []*UserSetting
	if err := x.In("setting_key", "debian.key.private", "rpm.key.private").Find(&privateKeys); err != nil {
		return err
	}
	for _, privateKey := range privateKeys {
		publicKey, err := recertifyPublicKey(privateKey.SettingValue)
		if err != nil {
			log.Warn("Skipping unreadable %s of user %d: %v", privateKey.SettingKey, privateKey.UserID, err)
			continue
		}
		if publicKey == "" {
			continue
		}
		if _, err := x.Where("user_id = ? AND setting_key = ?", privateKey.UserID, strings.TrimSuffix(privateKey.SettingKey, "private")+"public").Update(&UserSetting{SettingValue: publicKey}); err != nil {
			return err
		}
	}
	return nil
}
