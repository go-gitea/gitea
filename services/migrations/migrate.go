// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/models"
	admin_model "code.gitea.io/gitea/models/admin"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/hostmatcher"
	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"lab.forgefriends.org/friendlyforgeformat/gofff"
	gofff_domain "lab.forgefriends.org/friendlyforgeformat/gofff/domain"
	gofff_forges "lab.forgefriends.org/friendlyforgeformat/gofff/forges"
	gofff_gitea "lab.forgefriends.org/friendlyforgeformat/gofff/forges/gitea"
)

var (
	allowList *hostmatcher.HostMatchList
	blockList *hostmatcher.HostMatchList
)

// IsMigrateURLAllowed checks if an URL is allowed to be migrated from
func IsMigrateURLAllowed(remoteURL string, doer *user_model.User) error {
	// Remote address can be HTTP/HTTPS/Git URL or local path.
	u, err := url.Parse(remoteURL)
	if err != nil {
		return &models.ErrInvalidCloneAddr{IsURLError: true, Host: remoteURL}
	}

	if u.Scheme == "file" || u.Scheme == "" {
		if !doer.CanImportLocal() {
			return &models.ErrInvalidCloneAddr{Host: "<LOCAL_FILESYSTEM>", IsPermissionDenied: true, LocalPath: true}
		}
		isAbs := filepath.IsAbs(u.Host + u.Path)
		if !isAbs {
			return &models.ErrInvalidCloneAddr{Host: "<LOCAL_FILESYSTEM>", IsInvalidPath: true, LocalPath: true}
		}
		isDir, err := util.IsDir(u.Host + u.Path)
		if err != nil {
			log.Error("Unable to check if %s is a directory: %v", u.Host+u.Path, err)
			return err
		}
		if !isDir {
			return &models.ErrInvalidCloneAddr{Host: "<LOCAL_FILESYSTEM>", IsInvalidPath: true, LocalPath: true}
		}

		return nil
	}

	if u.Scheme == "git" && u.Port() != "" && (strings.Contains(remoteURL, "%0d") || strings.Contains(remoteURL, "%0a")) {
		return &models.ErrInvalidCloneAddr{Host: u.Host, IsURLError: true}
	}

	if u.Opaque != "" || u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "git" {
		return &models.ErrInvalidCloneAddr{Host: u.Host, IsProtocolInvalid: true, IsPermissionDenied: true, IsURLError: true}
	}

	hostName, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		// u.Host can be "host" or "host:port"
		err = nil //nolint
		hostName = u.Host
	}

	// some users only use proxy, there is no DNS resolver. it's safe to ignore the LookupIP error
	addrList, _ := net.LookupIP(hostName)
	return checkByAllowBlockList(hostName, addrList)
}

func checkByAllowBlockList(hostName string, addrList []net.IP) error {
	var ipAllowed bool
	var ipBlocked bool
	for _, addr := range addrList {
		ipAllowed = ipAllowed || allowList.MatchIPAddr(addr)
		ipBlocked = ipBlocked || blockList.MatchIPAddr(addr)
	}
	var blockedError error
	if blockList.MatchHostName(hostName) || ipBlocked {
		blockedError = &models.ErrInvalidCloneAddr{Host: hostName, IsPermissionDenied: true}
	}
	// if we have an allow-list, check the allow-list before return to get the more accurate error
	if !allowList.IsEmpty() {
		if !allowList.MatchHostName(hostName) && !ipAllowed {
			return &models.ErrInvalidCloneAddr{Host: hostName, IsPermissionDenied: true}
		}
	}
	// otherwise, we always follow the blocked list
	return blockedError
}

func ToGofffLogger(messenger base.Messenger) gofff.Logger {
	if messenger == nil {
		messenger = func(string, ...interface{}) {}
	}
	return gofff.Logger{
		Message:  messenger,
		Trace:    log.Trace,
		Debug:    log.Debug,
		Info:     log.Info,
		Warn:     log.Warn,
		Error:    log.Error,
		Critical: log.Critical,
		Fatal:    log.Fatal,
	}
}

// MigrateRepository migrate repository according MigrateOptions
func MigrateRepository(ctx context.Context, doer *user_model.User, ownerName string, opts base.MigrateOptions, messenger base.Messenger) (*repo_model.Repository, error) {
	err := IsMigrateURLAllowed(opts.CloneAddr, doer)
	if err != nil {
		return nil, err
	}
	if opts.LFS && len(opts.LFSEndpoint) > 0 {
		err := IsMigrateURLAllowed(opts.LFSEndpoint, doer)
		if err != nil {
			return nil, err
		}
	}

	tmpDir, err := os.MkdirTemp(os.TempDir(), "migrate")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	downloader, err := newDownloader(ctx, ownerName, tmpDir, opts, messenger)
	if err != nil {
		return nil, err
	}

	uploader := NewGiteaLocalUploader(ctx, doer, ownerName, opts)
	uploader.gitServiceType = opts.GitServiceType

	if err := gofff_domain.Migrate(ctx, downloader, uploader, ToGofffLogger(messenger), opts.ToGofffFeatures()); err != nil {
		uploader.Rollback()
		if err2 := admin_model.CreateRepositoryNotice(fmt.Sprintf("Migrate repository from %s failed: %v", opts.OriginalURL, err)); err2 != nil {
			log.Error("create respotiry notice failed: ", err2)
		}
		return nil, err
	}
	return uploader.repo, nil
}

func newDownloader(ctx context.Context, ownerName, tmpDir string, opts base.MigrateOptions, messenger base.Messenger) (gofff.ForgeInterface, error) {
	features := opts.ToGofffFeatures()

	switch opts.GitServiceType {
	case structs.GiteaService:
		options := gofff_gitea.Options{
			Options: gofff.Options{
				Configuration: gofff.Configuration{
					Directory:              tmpDir,
					NewMigrationHTTPClient: NewMigrationHTTPClient,
				},
				Features: features,
				Logger:   ToGofffLogger(messenger),
			},
			CloneAddr:    opts.CloneAddr,
			AuthUsername: opts.AuthUsername,
			AuthToken:    opts.AuthToken,
		}
		return gofff_forges.NewForge(&options)
	default:
		log.Error("Unrecognized %v", opts.GitServiceType)
		return nil, fmt.Errorf("Unrecognized %v", opts.GitServiceType)
	}
}

// Init migrations service
func Init() error {
	// TODO: maybe we can deprecate these legacy ALLOWED_DOMAINS/ALLOW_LOCALNETWORKS/BLOCKED_DOMAINS, use ALLOWED_HOST_LIST/BLOCKED_HOST_LIST instead

	blockList = hostmatcher.ParseSimpleMatchList("migrations.BLOCKED_DOMAINS", setting.Migrations.BlockedDomains)

	allowList = hostmatcher.ParseSimpleMatchList("migrations.ALLOWED_DOMAINS/ALLOW_LOCALNETWORKS", setting.Migrations.AllowedDomains)
	if allowList.IsEmpty() {
		// the default policy is that migration module can access external hosts
		allowList.AppendBuiltin(hostmatcher.MatchBuiltinExternal)
	}
	if setting.Migrations.AllowLocalNetworks {
		allowList.AppendBuiltin(hostmatcher.MatchBuiltinPrivate)
		allowList.AppendBuiltin(hostmatcher.MatchBuiltinLoopback)
	}
	// TODO: at the moment, if ALLOW_LOCALNETWORKS=false, ALLOWED_DOMAINS=domain.com, and domain.com has IP 127.0.0.1, then it's still allowed.
	// if we want to block such case, the private&loopback should be added to the blockList when ALLOW_LOCALNETWORKS=false
	return nil
}
