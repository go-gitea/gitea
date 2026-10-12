// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/secret"
	"gitea.dev/modules/setting"

	"xorm.io/xorm"
)

type mirrorRow struct {
	ID     int64
	RepoID int64
}

func (mirrorRow) TableName() string { return "mirror" }

type pushMirrorRow struct {
	ID         int64
	RepoID     int64
	RemoteName string
}

func (pushMirrorRow) TableName() string { return "push_mirror" }

// MoveMirrorCredentialsToDatabase moves the credentials of mirror remotes from the git config into the database, encrypted
func MoveMirrorCredentialsToDatabase(ctx context.Context, x base.EngineMigration) error {
	type Mirror struct {
		RemoteAddressEncrypted string `xorm:"TEXT"`
	}
	type PushMirror Mirror
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(Mirror), new(PushMirror)); err != nil {
		return err
	}
	move := func(ctx context.Context, table string, id, repoID int64, remoteName string) error {
		var repo struct{ OwnerName, Name string }
		if has, err := x.Table("repository").Where("id = ?", repoID).Get(&repo); err != nil || !has {
			return err
		}
		// a broken repository must not block the upgrade, its credentials are still used from the git config
		if err := moveMirrorRemoteCredentials(ctx, x, table, id, repo.OwnerName, repo.Name, remoteName); err != nil {
			log.Warn("Unable to move the credentials of %s %d (%s/%s) to the database: %v", table, id, repo.OwnerName, repo.Name, err)
		}
		return nil
	}
	if err := base.Iterate(ctx, nil, func(ctx context.Context, m *mirrorRow) error {
		return move(ctx, "mirror", m.ID, m.RepoID, "origin")
	}); err != nil {
		return err
	}
	return base.Iterate(ctx, nil, func(ctx context.Context, m *pushMirrorRow) error {
		return move(ctx, "push_mirror", m.ID, m.RepoID, m.RemoteName)
	})
}

func moveMirrorRemoteCredentials(ctx context.Context, x base.EngineMigration, table string, id int64, ownerName, repoName, remoteName string) error {
	codeRepo := base.LocalCodeGitRepo(ownerName, repoName)
	addr, err := remoteAddress(ctx, codeRepo, remoteName)
	if err != nil || gitcmd.RemoteAddressWithoutCredentials(addr) == addr {
		return err
	}
	encrypted, err := secret.EncryptSecret(setting.SecretKey, addr)
	if err != nil {
		return err
	}
	if _, err := x.Exec("UPDATE "+table+" SET remote_address_encrypted = ? WHERE id = ?", encrypted, id); err != nil {
		return err
	}
	if err := stripRemoteCredentials(ctx, codeRepo, remoteName, addr); err != nil {
		return err
	}
	wikiRepo := base.LocalWikiGitRepo(ownerName, repoName)
	wikiAddr, err := remoteAddress(ctx, wikiRepo, remoteName)
	if err != nil {
		return err
	}
	return stripRemoteCredentials(ctx, wikiRepo, remoteName, wikiAddr)
}

func stripRemoteCredentials(ctx context.Context, repo gitrepo.RepositoryFacade, remoteName, addr string) error {
	stripped := gitcmd.RemoteAddressWithoutCredentials(addr)
	if stripped == addr {
		return nil
	}
	_, _, err := gitcmd.NewCommand("remote", "set-url").AddDynamicArguments(remoteName, stripped).WithRepo(repo).RunStdString(ctx)
	return err
}

// remoteAddress returns an empty address if the repository or the remote does not exist
func remoteAddress(ctx context.Context, repo gitrepo.RepositoryFacade, remoteName string) (string, error) {
	if exist, _ := git.IsRepositoryExist(ctx, repo); !exist {
		return "", nil
	}
	addr, err := git.GetRemoteAddress(ctx, repo, remoteName)
	if git.IsRemoteNotExistError(err) {
		return "", nil
	}
	return addr, err
}
