// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"
	"net/url"
	"strings"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/secret"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"xorm.io/xorm"
)

type mirrorWithEncryptedAddress struct {
	RemoteAddressEncrypted string `xorm:"TEXT"`
}

func (mirrorWithEncryptedAddress) TableName() string {
	return "mirror"
}

type pushMirrorWithEncryptedAddress struct {
	RemoteAddressEncrypted string `xorm:"TEXT"`
}

func (pushMirrorWithEncryptedAddress) TableName() string {
	return "push_mirror"
}

// MoveMirrorCredentialsToDatabase moves the credentials of mirror remotes from the git config into the database, encrypted
func MoveMirrorCredentialsToDatabase(ctx context.Context, x base.EngineMigration) error {
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(mirrorWithEncryptedAddress), new(pushMirrorWithEncryptedAddress)); err != nil {
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
	if err != nil || addr == "" || stripCredentials(addr) == addr {
		return err
	}
	encrypted, err := secret.EncryptSecret(setting.SecretKey, addr)
	if err != nil {
		return err
	}
	if _, err := x.Exec("UPDATE "+table+" SET remote_address_encrypted = ? WHERE id = ?", encrypted, id); err != nil {
		return err
	}
	for _, repo := range []gitrepo.RepositoryFacade{codeRepo, base.LocalWikiGitRepo(ownerName, repoName)} {
		if err := stripRemoteCredentials(ctx, repo, remoteName); err != nil {
			return err
		}
	}
	return nil
}

func remoteAddress(ctx context.Context, repo gitrepo.RepositoryFacade, remoteName string) (string, error) {
	if !isGitRepo(repo) {
		return "", nil
	}
	stdout, _, err := gitcmd.NewCommand("config", "--get").AddDynamicArguments("remote." + remoteName + ".url").WithRepo(repo).RunStdString(ctx)
	if gitcmd.IsErrorExitCode(err, 1) {
		return "", nil // the remote does not exist
	}
	return strings.TrimSpace(stdout), err
}

func stripRemoteCredentials(ctx context.Context, repo gitrepo.RepositoryFacade, remoteName string) error {
	addr, err := remoteAddress(ctx, repo, remoteName)
	if err != nil || addr == "" || stripCredentials(addr) == addr {
		return err
	}
	_, _, err = gitcmd.NewCommand("config").AddDynamicArguments("remote."+remoteName+".url", stripCredentials(addr)).WithRepo(repo).RunStdString(ctx)
	return err
}

func stripCredentials(addr string) string {
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		return addr
	}
	u, err := url.Parse(addr)
	if err != nil || u.User == nil {
		return addr
	}
	u.User = nil
	return u.String()
}

func isGitRepo(repo gitrepo.RepositoryFacade) bool {
	exist, _ := util.IsExist(gitrepo.RepoLocalPath(repo))
	return exist
}
