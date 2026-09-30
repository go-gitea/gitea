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
	if err := moveMirrorCredentials(ctx, x, "mirror", "'origin'"); err != nil {
		return err
	}
	return moveMirrorCredentials(ctx, x, "push_mirror", "push_mirror.remote_name")
}

func moveMirrorCredentials(ctx context.Context, x base.EngineMigration, table, remoteNameExpr string) error {
	type mirrorRemote struct {
		ID         int64
		RemoteName string
		OwnerName  string
		RepoName   string
	}
	limit := setting.Database.IterateBufferSize
	if limit <= 0 {
		limit = 50
	}
	var lastID int64
	for {
		var mirrors []mirrorRemote
		if err := x.Table(table).
			Select(table+".id, "+remoteNameExpr+" AS remote_name, repository.owner_name, repository.name AS repo_name").
			Join("INNER", "repository", "repository.id = "+table+".repo_id").
			Where(table+".id > ?", lastID).OrderBy(table + ".id").Limit(limit).
			Find(&mirrors); err != nil {
			return err
		}
		if len(mirrors) == 0 {
			return nil
		}
		lastID = mirrors[len(mirrors)-1].ID
		for _, m := range mirrors {
			// a broken repository must not block the upgrade, its credentials are still used from the git config
			if err := moveMirrorRemoteCredentials(ctx, x, table, m.ID, m.OwnerName, m.RepoName, m.RemoteName); err != nil {
				log.Warn("Unable to move the credentials of %s %d (%s/%s) to the database: %v", table, m.ID, m.OwnerName, m.RepoName, err)
			}
		}
	}
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
