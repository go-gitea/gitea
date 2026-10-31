// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	giturl "gitea.dev/modules/git/url"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

// UseOwnerSSHKey reports whether doer's SSH remote addr for owner's repository authenticates with owner's managed SSH key, which only owner itself or an owner of the organization may bind
func UseOwnerSSHKey(ctx context.Context, doer, owner *user_model.User, addr string) (bool, error) {
	if !giturl.IsSSH(addr) {
		return false, nil
	}
	if !user_model.DoerCanViewPrivate(doer) {
		return false, util.NewPermissionDeniedErrorf("public-only tokens can't use managed SSH keys")
	}
	canUse := doer.ID == owner.ID
	if !canUse && owner.IsOrganization() {
		var err error
		if canUse, err = organization.IsOrganizationOwner(ctx, owner.ID, doer.ID); err != nil {
			return false, err
		}
	}
	if !canUse {
		return false, util.ErrorWrapTranslatable(util.NewPermissionDeniedErrorf("%s can't use the managed SSH key of %s", doer.Name, owner.Name), "repo.migrate.ssh_key_not_allowed")
	}
	if _, err := user_model.GetSSHKeypairByOwner(ctx, owner.ID); errors.Is(err, util.ErrNotExist) {
		return false, util.ErrorWrapTranslatable(util.ErrorWrap(util.ErrUnprocessableContent, "the managed SSH key of %s does not exist", owner.Name), "repo.migrate.ssh_key_missing")
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// ManagedSSHEnv authenticates git with the managed SSH key of repo's owner when useOwnerKey is set for an SSH addr
func ManagedSSHEnv(ctx context.Context, repo *repo_model.Repository, useOwnerKey bool, addr string) (envs []string, cleanup func(), err error) {
	if !giturl.IsSSH(addr) {
		return nil, func() {}, nil
	}
	if !useOwnerKey { // never fall back to the server's own SSH identity
		return nil, nil, util.NewPermissionDeniedErrorf("the SSH remote of %s isn't bound to its owner's managed SSH key, save the mirror again", repo.FullName())
	}
	keypair, err := user_model.GetSSHKeypairByOwner(ctx, repo.OwnerID)
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
	sshCommand := setting.Migrations.SSHCommand
	if sshCommand == "" {
		sshCommand = defaultSSHCommand()
	}
	return util.ShellEscape(sshCommand) + " -o BatchMode=yes -o IdentitiesOnly=yes -o IdentityAgent=none -o ControlPath=none" +
		" -i " + util.ShellEscape(keyFile) +
		" -o StrictHostKeyChecking=" + setting.Migrations.SSHHostKeyChecking +
		" -o " + util.ShellEscape(`UserKnownHostsFile="`+knownHosts+`"`) // ssh splits the option value on spaces
}
