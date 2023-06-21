// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"errors"
	"fmt"

	"code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	org "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type IdentidyOwnerParameters struct {
	*context.Context
	Owner string
	Email string
}

// This function will find user related to provided email adress and check if
// he is able to push packages to provided namespace (user/organization/or
// empty namespace allowed for admin users). Function will return user making
// operation, organization or user owning the package.
func IdentifyOwner(ctx *context.Context, owner, email string) (*user.User, *org.Organization, error) {
	u, err := user.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to find user with email %s, %v", email, err)
	}

	if owner == "" && u.IsAdmin {
		return u, (*org.Organization)(u), nil
	}

	if owner == u.Name {
		return u, (*org.Organization)(u), nil
	}

	if u.Name != owner {
		org, err := org.GetOrgByName(ctx, owner)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to get organization: %s, %v", owner, err)
		}
		ismember, err := org.IsOrgMember(u.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to check if user %s belongs to organization %s: %v", u.Name, org.Name, err)
		}
		if !ismember {
			return nil, nil, fmt.Errorf("user %s is not member of organization %s", u.Name, org.Name)
		}
		return u, org, nil
	}
	return nil, nil, fmt.Errorf("unknown package owner")
}

// Validate package signature with owner's GnuPG keys stored in gitea's database.
func ValidatePackageSignature(ctx *context.Context, pkg, sign []byte, u *user.User) error {
	keys, err := asymkey.ListGPGKeys(ctx, u.ID, db.ListOptions{
		ListAll: true,
	})
	if err != nil {
		return errors.New("unable to get public keys")
	}
	if len(keys) == 0 {
		return errors.New("no keys for user with email: " + u.Email)
	}

	var keyarmors []string
	for _, key := range keys {
		k, err := asymkey.GetGPGImportByKeyID(key.KeyID)
		if err != nil {
			return errors.New("unable to import GPG key armor")
		}
		keyarmors = append(keyarmors, k.Content)
	}

	var matchedKeyring *crypto.KeyRing
	for _, armor := range keyarmors {
		pgpkey, err := crypto.NewKeyFromArmored(armor)
		if err != nil {
			return fmt.Errorf("unable to get keys for %s: %v", u.Name, err)
		}
		keyring, err := crypto.NewKeyRing(pgpkey)
		if err != nil {
			return fmt.Errorf("unable to form keyring %s: %v", u.Name, err)
		}
		for _, idnt := range keyring.GetIdentities() {
			if idnt.Email == u.Email {
				matchedKeyring = keyring
				break
			}
		}
		if matchedKeyring != nil {
			break
		}
	}
	if matchedKeyring == nil {
		return fmt.Errorf("GPG key related to %s not found", u.Email)
	}

	var (
		pgpmes = crypto.NewPlainMessage(pkg)
		pgpsig = crypto.NewPGPSignature(sign)
	)

	return matchedKeyring.VerifyDetached(pgpmes, pgpsig, crypto.GetUnixTime())
}
