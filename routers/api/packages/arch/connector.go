// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"errors"
	"io"

	"code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	organization_model "code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
)

// Connector helps to retrieve GPG keys related to package validation and
// manage blobs related to specific user spaces:
// 1 - Check if user is allowed to push package to specific namespace.
// 2 - Retrieving GPG keys related to provided email.
// 3 - Get/put arch arch package/signature/database files to connected file
// storage.
type Connector struct {
	ctx  *context.Context
	user *user_model.User
	org  *organization_model.Organization
}

// This function will find user related to provided email adress and check if
// he is able to push packages to provided namespace (user/organization/or
// empty namespace allowed for admin users).
func (c *Connector) ValidateNamespace(namespace, email string) error {
	var err error
	c.user, err = user_model.GetUserByEmail(c.ctx, email)
	if err != nil {
		log.Error("unable to get user with email: %s  %v", email, err)
		return err
	}

	if namespace == "" && c.user.IsAdmin {
		c.org = (*organization_model.Organization)(c.user)
		return nil
	}

	if c.user.Name != namespace && c.org == nil {
		c.org, err = organization_model.GetOrgByName(c.ctx, namespace)
		if err != nil {
			log.Error("unable to organization: %s %v", namespace, err)
			return err
		}
		ismember, err := c.org.IsOrgMember(c.user.ID)
		if err != nil {
			log.Error(
				"unable to check if user belongs to organization: %s %s %v",
				c.user.Name, email, err,
			)
			return err
		}
		if !ismember {
			log.Error("user %s is not member of organization: %s", c.user.Name, email)
			return errors.New("user is not member of organization: " + namespace)
		}
	} else {
		c.org = (*organization_model.Organization)(c.user)
	}
	return nil
}

// This function will try to find user related to specific email. And check
// that user is allowed to push to 'owner' namespace (package owner, could
// be empty, user or organization).
// After namespace check, this function
func (c *Connector) GetValidKeys(email string) ([]string, error) {
	keys, err := asymkey.ListGPGKeys(c.ctx, c.user.ID, db.ListOptions{
		ListAll: true,
	})
	if err != nil {
		log.Error("unable to get keys related to user: %v", err)
		return nil, errors.New("unable to get public keys")
	}
	if len(keys) == 0 {
		log.Error("no keys related to user")
		return nil, errors.New("no keys for user with email: " + email)
	}

	var keyarmors []string
	for _, key := range keys {
		k, err := asymkey.GetGPGImportByKeyID(key.KeyID)
		if err != nil {
			log.Error("unable to import GPG key by ID: %v", err)
			return nil, errors.New("internal error")
		}
		keyarmors = append(keyarmors, k.Content)
	}

	return keyarmors, nil
}

// Get specific file content from content storage.
func (c *Connector) Get(key string) ([]byte, error) {
	cs := packages_module.NewContentStore()
	obj, err := cs.Get(packages_module.BlobHash256Key(key))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(obj)
}

// Save contents related to specific arch package.
func (c *Connector) Save(key string, content io.Reader, size int64) error {
	cs := packages_module.NewContentStore()
	return cs.Save(packages_module.BlobHash256Key(key), content, size)
}

// Join database or package names to prevent collisions with same packages in
// different user spaces. Skips empty strings and returns name joined with
// dots.
func Join(s ...string) string {
	rez := ""
	for i, v := range s {
		if v == "" {
			continue
		}
		if i+1 == len(s) {
			rez += v
			continue
		}
		rez += v + "."
	}
	return rez
}
