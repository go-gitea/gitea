// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package pages contains the database models used by Gitea Pages.
package pages

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"

	"xorm.io/builder"
)

type PublishSource string

const (
	PublishSourceArtifact PublishSource = "artifact"
	PublishSourceBranch   PublishSource = "branch"
)

type VersionOrigin string

const (
	VersionOriginActions VersionOrigin = "actions"
	VersionOriginUpload  VersionOrigin = "upload"
	VersionOriginBranch  VersionOrigin = "branch"
)

type SourceErrorCode string

const (
	SourceErrorRepositoryMissing SourceErrorCode = "source_repository_missing"
	SourceErrorGitUnavailable    SourceErrorCode = "source_git_unavailable"
	SourceErrorBranchMissing     SourceErrorCode = "source_branch_missing"
	SourceErrorArchiveFailed     SourceErrorCode = "source_archive_failed"
	SourceErrorArchiveInvalid    SourceErrorCode = "source_archive_invalid"
	SourceErrorCommitFailed      SourceErrorCode = "source_commit_failed"
)

type DomainState string

const (
	DomainStatePending   DomainState = "pending"
	DomainStateActive    DomainState = "active"
	DomainStateReleasing DomainState = "releasing"
)

type TXTResult string

const (
	TXTResultPending        TXTResult = "pending"
	TXTResultValid          TXTResult = "valid"
	TXTResultInvalid        TXTResult = "invalid"
	TXTResultTemporaryError TXTResult = "temporary_error"
)

type InstanceErrorCode string

const (
	InstanceErrorCertificatePending    InstanceErrorCode = "certificate_pending"
	InstanceErrorContentDownloadFailed InstanceErrorCode = "content_download_failed"
	InstanceErrorContentInvalid        InstanceErrorCode = "content_invalid"
	InstanceErrorContentInstallFailed  InstanceErrorCode = "content_install_failed"
	InstanceErrorRouteApplyFailed      InstanceErrorCode = "route_apply_failed"
	InstanceErrorTXTInvalid            InstanceErrorCode = "txt_invalid"
	InstanceErrorACMEFailed            InstanceErrorCode = "acme_failed"
	InstanceErrorEntryProbeFailed      InstanceErrorCode = "entry_probe_failed"
)

// Site stores the current Pages configuration for a repository. Its row
// is retained after repository deletion so synchronization can observe it.
type Site struct {
	Deleted              bool            `xorm:"NOT NULL DEFAULT false INDEX(deleted_updated)"`
	Enabled              bool            `xorm:"NOT NULL DEFAULT false"`
	SourceType           PublishSource   `xorm:"VARCHAR(16) NOT NULL DEFAULT '' INDEX(source_type_next)"`
	SourceBranch         string          `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	SourcePath           string          `xorm:"VARCHAR(1024) NOT NULL DEFAULT ''"`
	SourceCommitID       string          `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	SourceNextCheckUnix  int64           `xorm:"NOT NULL DEFAULT 0 INDEX(source_type_next)"`
	SourceErrorCode      SourceErrorCode `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	CurrentVersionID     int64           `xorm:"NOT NULL DEFAULT 0"`
	PagesUpdatedUnixNano int64           `xorm:"NOT NULL INDEX(updated_repo) INDEX(deleted_updated)"`
	RepoID               int64           `xorm:"pk INDEX(updated_repo) INDEX(deleted_updated)"`
}

// TableName returns the database table name.
func (Site) TableName() string { return "pages_site" }

// Version identifies one validated Pages archive. A retained digest has one
// immutable record per repository.
type Version struct {
	ID                int64              `xorm:"pk autoincr"`
	RepoID            int64              `xorm:"NOT NULL UNIQUE(repo_digest) INDEX"`
	TarGzSHA256       string             `xorm:"'tar_gz_sha256' CHAR(64) NOT NULL UNIQUE(repo_digest)"`
	OriginType        VersionOrigin      `xorm:"VARCHAR(16) NOT NULL"`
	OriginActionRunID int64              `xorm:"NOT NULL DEFAULT 0"`
	CreatedUnix       timeutil.TimeStamp `xorm:"created NOT NULL"`
}

// TableName returns the database table name.
func (Version) TableName() string { return "pages_version" }

// Instance stores an administrator-registered Pages process.
type Instance struct {
	ID                    int64              `xorm:"pk autoincr"`
	RootURL               string             `xorm:"VARCHAR(512) NOT NULL DEFAULT ''"`
	RootUpdatedUnixNano   int64              `xorm:"NOT NULL DEFAULT 0"`
	LastCommunicationUnix timeutil.TimeStamp `xorm:"NOT NULL DEFAULT 0"`
	TokenHash             string             `xorm:"VARCHAR(64) NOT NULL UNIQUE"`
}

// TableName returns the database table name.
func (Instance) TableName() string { return "pages_instance" }

// Domain stores a custom host and its Gitea TXT verification state.
type Domain struct {
	ID                     int64       `xorm:"pk autoincr"`
	RepoID                 int64       `xorm:"NOT NULL INDEX(repo_state) INDEX(state_repo)"`
	Host                   string      `xorm:"VARCHAR(253) NOT NULL UNIQUE"`
	TXTToken               string      `xorm:"VARCHAR(64) NOT NULL"`
	State                  DomainState `xorm:"VARCHAR(16) NOT NULL INDEX(repo_state) INDEX(state_repo)"`
	TXTResult              TXTResult   `xorm:"VARCHAR(32) NOT NULL"`
	TXTLastSuccessUnixNano int64       `xorm:"NOT NULL DEFAULT 0"`
	TXTLastCheckUnix       int64       `xorm:"NOT NULL DEFAULT 0 INDEX(txt_last_check)"`
	CreatedUnixNano        int64       `xorm:"NOT NULL"`
}

// TableName returns the database table name.
func (Domain) TableName() string { return "pages_domain" }

// InstanceResult stores the latest accepted deployment result for one instance and repository.
type InstanceResult struct {
	InstanceID                   int64             `xorm:"pk INDEX(repo_instance)"`
	RepoID                       int64             `xorm:"pk INDEX(repo_instance)"`
	AppliedPagesUpdatedUnixNano  int64             `xorm:"NOT NULL DEFAULT 0"`
	AppliedRootUpdatedUnixNano   int64             `xorm:"NOT NULL DEFAULT 0"`
	AppliedRequireGiteaTXTRecord bool              `xorm:"NOT NULL DEFAULT false"`
	ConfigApplied                bool              `xorm:"NOT NULL DEFAULT false"`
	ActualTarGzSHA256            string            `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	DefaultPublished             bool              `xorm:"NOT NULL DEFAULT false"`
	CustomPublished              bool              `xorm:"NOT NULL DEFAULT false"`
	ErrorCode                    InstanceErrorCode `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
}

// TableName returns the database table name.
func (InstanceResult) TableName() string { return "pages_instance_site" }

func init() {
	db.RegisterModel(new(Site))
	db.RegisterModel(new(Version))
	db.RegisterModel(new(Instance))
	db.RegisterModel(new(Domain))
	db.RegisterModel(new(InstanceResult))
}

// FindVersionOptions filters and paginates a repository's Pages versions.
type FindVersionOptions struct {
	db.ListOptions
	RepoID int64
}

func (opts *FindVersionOptions) ToConds() builder.Cond {
	return builder.Eq{"repo_id": opts.RepoID}
}

func (opts *FindVersionOptions) ToOrders() string {
	return "id DESC"
}

// FindVersions lists a repository's versions with newest versions first.
func FindVersions(ctx context.Context, opts *FindVersionOptions) ([]*Version, int64, error) {
	return db.FindAndCount[Version](ctx, opts)
}

// GetVersionByID returns one Pages version.
func GetVersionByID(ctx context.Context, id int64) (*Version, bool, error) {
	return db.GetByID[Version](ctx, id)
}

// GetVersionByRepoIDAndDigest returns the immutable version for a digest.
func GetVersionByRepoIDAndDigest(ctx context.Context, repoID int64, digest string) (*Version, bool, error) {
	return db.Get[Version](ctx, builder.Eq{"repo_id": repoID, "tar_gz_sha256": digest})
}

// GetVersionsByIDs returns versions for a batch of current site pointers.
func GetVersionsByIDs(ctx context.Context, ids []int64) ([]*Version, error) {
	versions := make([]*Version, 0, len(ids))
	if len(ids) == 0 {
		return versions, nil
	}
	err := db.GetEngine(ctx).In("id", ids).Find(&versions)
	return versions, err
}

// GetCurrentVersion resolves and validates a site's current version pointer.
func GetCurrentVersion(ctx context.Context, site *Site) (*Version, bool, error) {
	if site == nil || site.CurrentVersionID == 0 {
		return nil, false, nil
	}
	version, has, err := GetVersionByID(ctx, site.CurrentVersionID)
	if err != nil || !has {
		return version, has, err
	}
	if version.RepoID != site.RepoID {
		return nil, false, errors.New("Pages current version belongs to another repository")
	}
	return version, true, nil
}

// InsertVersion creates an immutable Pages version.
func InsertVersion(ctx context.Context, version *Version) error {
	if version == nil || version.ID != 0 || version.RepoID <= 0 || len(version.TarGzSHA256) != 64 ||
		version.OriginType != VersionOriginActions && version.OriginType != VersionOriginUpload && version.OriginType != VersionOriginBranch {
		return errors.New("new Pages version is invalid")
	}
	if decoded, err := hex.DecodeString(version.TarGzSHA256); err != nil || len(decoded) != 32 || version.TarGzSHA256 != strings.ToLower(version.TarGzSHA256) {
		return errors.New("new Pages version has an invalid digest")
	}
	return db.Insert(ctx, version)
}

// DeleteVersionByID removes one Pages version record.
func DeleteVersionByID(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).ID(id).Delete(new(Version))
	return err
}

// GetSiteByRepoID returns the retained Pages row for a repository.
func GetSiteByRepoID(ctx context.Context, repoID int64) (*Site, bool, error) {
	return db.Get[Site](ctx, builder.Eq{"repo_id": repoID})
}

// InsertSite creates a repository's Pages row.
func InsertSite(ctx context.Context, site *Site) error {
	if site == nil || site.RepoID <= 0 {
		return errors.New("pages site repository ID must be positive")
	}
	return db.Insert(ctx, site)
}

// UpdateSite replaces a repository's complete Pages row.
func UpdateSite(ctx context.Context, site *Site) error {
	if site == nil || site.RepoID <= 0 {
		return errors.New("pages site repository ID must be positive")
	}
	_, err := db.GetEngine(ctx).ID(site.RepoID).NoAutoCondition().AllCols().Update(site)
	return err
}

// GetInstanceByID returns an instance by ID.
func GetInstanceByID(ctx context.Context, id int64) (*Instance, bool, error) {
	return db.GetByID[Instance](ctx, id)
}

// ListInstances returns all instances in ID order.
func ListInstances(ctx context.Context) ([]*Instance, error) {
	instances := make([]*Instance, 0)
	err := db.GetEngine(ctx).Asc("id").Find(&instances)
	return instances, err
}

// InsertInstance creates an instance. TokenHash is a digest, never a raw token.
func InsertInstance(ctx context.Context, instance *Instance) error {
	if instance == nil || instance.ID != 0 {
		return errors.New("new pages instance must not have an ID")
	}
	return db.Insert(ctx, instance)
}

// UpdateInstance replaces an existing instance.
func UpdateInstance(ctx context.Context, instance *Instance) error {
	if instance == nil || instance.ID <= 0 {
		return errors.New("pages instance ID must be positive")
	}
	_, err := db.GetEngine(ctx).ID(instance.ID).NoAutoCondition().AllCols().Update(instance)
	return err
}

// DeleteInstanceByID removes the instance and its current results.
func DeleteInstanceByID(ctx context.Context, id int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Where("instance_id = ?", id).Delete(new(InstanceResult)); err != nil {
			return err
		}
		_, err := db.DeleteByID[Instance](ctx, id)
		return err
	})
}

// GetDomainByID returns a domain by ID.
func GetDomainByID(ctx context.Context, id int64) (*Domain, bool, error) {
	return db.GetByID[Domain](ctx, id)
}

// GetDomainByHost returns a domain by normalized Host.
func GetDomainByHost(ctx context.Context, host string) (*Domain, bool, error) {
	return db.Get[Domain](ctx, builder.Eq{"host": host})
}

// ListDomainsByRepoID returns all bindings for a repository.
func ListDomainsByRepoID(ctx context.Context, repoID int64) ([]*Domain, error) {
	domains := make([]*Domain, 0)
	err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Asc("id").Find(&domains)
	return domains, err
}

// InsertDomain creates a domain binding.
func InsertDomain(ctx context.Context, domain *Domain) error {
	if domain == nil || domain.ID != 0 || domain.RepoID <= 0 || domain.Host == "" {
		return errors.New("pages domain repository ID and host are required")
	}
	return db.Insert(ctx, domain)
}

// UpdateDomain replaces an existing domain binding.
func UpdateDomain(ctx context.Context, domain *Domain) error {
	if domain == nil || domain.ID <= 0 || domain.RepoID <= 0 || domain.Host == "" {
		return errors.New("pages domain ID, repository ID, and host are required")
	}
	_, err := db.GetEngine(ctx).ID(domain.ID).NoAutoCondition().AllCols().Update(domain)
	return err
}

// DeleteDomainByID removes a binding after all instances acknowledge its removal.
func DeleteDomainByID(ctx context.Context, id int64) error {
	_, err := db.DeleteByID[Domain](ctx, id)
	return err
}

// GetInstanceResult returns the current result for an instance and repository.
func GetInstanceResult(ctx context.Context, instanceID, repoID int64) (*InstanceResult, bool, error) {
	return db.Get[InstanceResult](ctx, builder.Eq{"instance_id": instanceID, "repo_id": repoID})
}

// ListInstanceResultsByRepoID returns all instance results for a repository.
func ListInstanceResultsByRepoID(ctx context.Context, repoID int64) ([]*InstanceResult, error) {
	results := make([]*InstanceResult, 0)
	err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Asc("instance_id").Find(&results)
	return results, err
}

// InsertInstanceResult creates the current result for an instance and repository.
func InsertInstanceResult(ctx context.Context, result *InstanceResult) error {
	if result == nil || result.InstanceID <= 0 || result.RepoID <= 0 {
		return errors.New("pages instance result identifiers must be positive")
	}
	return db.Insert(ctx, result)
}

// UpdateInstanceResult replaces the current result for an instance and repository.
func UpdateInstanceResult(ctx context.Context, result *InstanceResult) error {
	if result == nil || result.InstanceID <= 0 || result.RepoID <= 0 {
		return errors.New("pages instance result identifiers must be positive")
	}
	_, err := db.GetEngine(ctx).
		Where("instance_id = ? AND repo_id = ?", result.InstanceID, result.RepoID).
		NoAutoCondition().AllCols().Update(result)
	return err
}
