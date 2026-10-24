// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"gitea.dev/models/organization"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	giturl "gitea.dev/modules/git/url"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

// ManagedSSHKeyOwner picks the managed SSH key for doer's SSH remote addr of owner's repository: doer's own, or as its owner the organization's, which requestedID 0 prefers
func ManagedSSHKeyOwner(ctx context.Context, doer, owner *user_model.User, addr string, requestedID int64) (int64, error) {
	if !giturl.IsSSH(addr) {
		return 0, nil
	}
	if !user_model.DoerCanViewPrivate(doer) {
		return 0, util.NewPermissionDeniedErrorf("public-only tokens can't use managed SSH keys")
	}
	keyOwnerID := doer.ID
	if requestedID != doer.ID && owner.IsOrganization() && (requestedID == 0 || requestedID == owner.ID) {
		isOrgOwner, err := organization.IsOrganizationOwner(ctx, owner.ID, doer.ID)
		if err != nil {
			return 0, err
		}
		if isOrgOwner {
			keyOwnerID = owner.ID
		}
	}
	if requestedID != 0 && requestedID != keyOwnerID {
		return 0, util.ErrorWrapTranslatable(util.NewPermissionDeniedErrorf("the managed SSH key of %d is not available", requestedID), "repo.migrate.invalid_ssh_key_owner")
	}
	if _, err := user_model.GetSSHKeypairByOwner(ctx, keyOwnerID); errors.Is(err, util.ErrNotExist) {
		return 0, util.ErrorWrapTranslatable(util.ErrorWrap(util.ErrUnprocessableContent, "the managed SSH key of %d does not exist", keyOwnerID), "repo.migrate.ssh_key_missing")
	} else if err != nil {
		return 0, err
	}
	return keyOwnerID, nil
}

// ManagedSSHEnv authenticates git with keyOwnerID's managed SSH key while it may serve repo, as its organization's owner or a user administering it
func ManagedSSHEnv(ctx context.Context, repo *repo_model.Repository, keyOwnerID int64, addr string) (envs []string, cleanup func(), err error) {
	if keyOwnerID == 0 || !giturl.IsSSH(addr) {
		return nil, func() {}, nil
	}
	if keyOwnerID != repo.OwnerID {
		keyOwner, err := user_model.GetValidDoerUserByID(ctx, keyOwnerID)
		if err != nil {
			return nil, nil, err
		}
		perm, err := access_model.GetIndividualUserRepoPermission(ctx, repo, keyOwner)
		if err != nil {
			return nil, nil, err
		}
		if !perm.IsAdmin() {
			return nil, nil, util.NewPermissionDeniedErrorf("%s can no longer administer %s", keyOwner.Name, repo.FullName())
		}
	}
	keypair, err := user_model.GetSSHKeypairByOwner(ctx, keyOwnerID)
	if err != nil {
		return nil, nil, err
	}
	privateKey, err := keypair.PrivateKey()
	if err != nil {
		return nil, nil, err
	}
	knownHosts := filepath.Join(setting.Git.HomePath, ".ssh", "known_hosts")
	if err := os.MkdirAll(filepath.Dir(knownHosts), 0o700); err != nil {
		return nil, nil, err
	}
	dir, cleanup, err := setting.AppDataTempDir("ssh").MkdirTempRandom("key-")
	if err != nil {
		return nil, nil, err
	}
	keyFile := filepath.Join(dir, "id")
	if err = restrictToCurrentUser(dir); err == nil {
		err = os.WriteFile(keyFile, privateKey, 0o600)
	}
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return append(os.Environ(), "GIT_SSH_COMMAND="+managedSSHCommand(keyFile, knownHosts)), cleanup, nil
}

// managedSSHCommand offers nothing but keyFile and never prompts, ControlPath=none keeps a shared connection from authenticating as another key
func managedSSHCommand(keyFile, knownHosts string) string {
	if setting.Migrations.SSHHostKeyChecking == "no" {
		knownHosts = os.DevNull
	}
	return util.ShellEscape(setting.Migrations.SSHCommand) + " -o BatchMode=yes -o IdentitiesOnly=yes -o IdentityAgent=none -o ControlPath=none" +
		" -i " + util.ShellEscape(keyFile) +
		" -o StrictHostKeyChecking=" + setting.Migrations.SSHHostKeyChecking +
		" -o " + util.ShellEscape(`UserKnownHostsFile="`+knownHosts+`"`) // ssh splits the option value on spaces
}
