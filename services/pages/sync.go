// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	pages_content "gitea.dev/modules/pages"
	"gitea.dev/modules/util"
)

const MaxSyncPageSize = 200

type Cursor struct {
	PagesUpdatedUnixNano int64 `json:"pages_updated_unix_nano"`
	RepoID               int64 `json:"repo_id"`
}

type DomainRecord struct {
	ID                     int64                   `json:"id"`
	Host                   string                  `json:"host"`
	TXTToken               string                  `json:"txt_token"`
	TXTResult              pages_model.TXTResult   `json:"txt_result"`
	TXTLastSuccessUnixNano int64                   `json:"txt_last_success_unix_nano"`
	State                  pages_model.DomainState `json:"state"`
}

type SiteRecord struct {
	RepoID               int64          `json:"repo_id"`
	Deleted              bool           `json:"deleted"`
	Enabled              bool           `json:"enabled"`
	Owner                string         `json:"owner,omitempty"`
	Repository           string         `json:"repository,omitempty"`
	TarGzSHA256          string         `json:"tar_gz_sha256,omitempty"`
	PagesUpdatedUnixNano int64          `json:"pages_updated_unix_nano"`
	CustomDomains        []DomainRecord `json:"custom_domains,omitempty"`
}

type SyncResponse struct {
	RootURL                  string       `json:"root_url"`
	RootUpdatedUnixNano      int64        `json:"root_updated_unix_nano"`
	TXTVerificationEnabled   bool         `json:"txt_verification_enabled"`
	TXTPolicyUpdatedUnixNano int64        `json:"txt_policy_updated_unix_nano"`
	Until                    int64        `json:"until"`
	NextSince                int64        `json:"next_since"`
	Sites                    []SiteRecord `json:"sites"`
	NextCursor               *Cursor      `json:"next_cursor,omitempty"`
}

type SyncOptions struct {
	Full      bool
	Since     int64
	Until     int64
	AfterTime int64
	AfterRepo int64
	Limit     int
}

// ListSitesForSync returns one stable page of current facts for an instance.
func ListSitesForSync(ctx context.Context, instance *pages_model.Instance, opts SyncOptions) (*SyncResponse, error) {
	if instance == nil || instance.RootURL == "" || opts.Since < 0 || opts.Until < 0 ||
		opts.AfterTime < 0 || opts.AfterRepo < 0 || opts.Limit < 1 || opts.Limit > MaxSyncPageSize {
		return nil, errors.New("ListSitesForSync: invalid sync options")
	}
	until := opts.Until
	if until == 0 {
		var err error
		until, err = latestSiteUpdateTime(ctx)
		if err != nil {
			return nil, err
		}
	}
	if !opts.Full && opts.Since > until {
		return nil, ErrCursorAhead
	}
	if opts.AfterTime > until || opts.AfterTime != 0 && opts.AfterTime < opts.Since {
		return nil, errors.New("ListSitesForSync: invalid page cursor")
	}

	rows := make([]*pages_model.Site, 0, (opts.Limit+1)*2)
	if opts.Full {
		// Live rows and deletion rows have different useful indexes. Querying
		// them separately avoids scanning every retained deleted repository.
		live := make([]*pages_model.Site, 0, opts.Limit+1)
		query := db.GetEngine(ctx).Where("deleted = ? AND pages_updated_unix_nano <= ?", false, until)
		if opts.AfterTime > 0 {
			query = query.And("(pages_updated_unix_nano > ? OR (pages_updated_unix_nano = ? AND repo_id > ?))", opts.AfterTime, opts.AfterTime, opts.AfterRepo)
		}
		if err := query.Asc("pages_updated_unix_nano", "repo_id").Limit(opts.Limit + 1).Find(&live); err != nil {
			return nil, fmt.Errorf("ListSitesForSync: live sites: %w", err)
		}
		rows = append(rows, live...)

		deleting := make([]*pages_model.Site, 0, opts.Limit+1)
		query = db.GetEngine(ctx).Where("deleted = ? AND pages_updated_unix_nano <= ? AND EXISTS (SELECT 1 FROM pages_domain WHERE pages_domain.repo_id = pages_site.repo_id AND pages_domain.state = ?)", true, until, pages_model.DomainStateReleasing)
		if opts.AfterTime > 0 {
			query = query.And("(pages_updated_unix_nano > ? OR (pages_updated_unix_nano = ? AND repo_id > ?))", opts.AfterTime, opts.AfterTime, opts.AfterRepo)
		}
		if err := query.Asc("pages_updated_unix_nano", "repo_id").Limit(opts.Limit + 1).Find(&deleting); err != nil {
			return nil, fmt.Errorf("ListSitesForSync: deleting sites: %w", err)
		}
		rows = append(rows, deleting...)
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].PagesUpdatedUnixNano != rows[j].PagesUpdatedUnixNano {
				return rows[i].PagesUpdatedUnixNano < rows[j].PagesUpdatedUnixNano
			}
			return rows[i].RepoID < rows[j].RepoID
		})
	} else {
		query := db.GetEngine(ctx).Where("pages_updated_unix_nano > ? AND pages_updated_unix_nano <= ?", opts.Since, until)
		if opts.AfterTime > 0 {
			query = query.And("(pages_updated_unix_nano > ? OR (pages_updated_unix_nano = ? AND repo_id > ?))", opts.AfterTime, opts.AfterTime, opts.AfterRepo)
		}
		if err := query.Asc("pages_updated_unix_nano", "repo_id").Limit(opts.Limit + 1).Find(&rows); err != nil {
			return nil, fmt.Errorf("ListSitesForSync: updated sites: %w", err)
		}
	}
	hasMore := len(rows) > opts.Limit
	if hasMore {
		rows = rows[:opts.Limit]
	}

	ids := make([]int64, 0, len(rows))
	for _, site := range rows {
		if !site.Deleted {
			ids = append(ids, site.RepoID)
		}
	}
	repos, err := repo_model.GetRepositoriesMapByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("ListSitesForSync: repositories: %w", err)
	}
	ownerIDs := make([]int64, 0, len(repos))
	for _, repo := range repos {
		ownerIDs = append(ownerIDs, repo.OwnerID)
	}
	owners, err := user_model.GetUserByIDs(ctx, ownerIDs)
	if err != nil {
		return nil, fmt.Errorf("ListSitesForSync: owners: %w", err)
	}
	ownerNames := make(map[int64]string, len(owners))
	for _, owner := range owners {
		ownerNames[owner.ID] = owner.LowerName
	}
	domains := make([]*pages_model.Domain, 0)
	if len(ids) > 0 {
		if err := db.GetEngine(ctx).In("repo_id", ids).Where("state = ?", pages_model.DomainStateActive).Find(&domains); err != nil {
			return nil, fmt.Errorf("ListSitesForSync: domains: %w", err)
		}
	}
	domainByRepo := make(map[int64]*pages_model.Domain, len(domains))
	for _, domain := range domains {
		domainByRepo[domain.RepoID] = domain
	}
	records := make([]SiteRecord, 0, len(rows))
	for _, site := range rows {
		record := SiteRecord{
			RepoID: site.RepoID, Deleted: site.Deleted, Enabled: site.Enabled,
			TarGzSHA256: site.TarGzSHA256, PagesUpdatedUnixNano: site.PagesUpdatedUnixNano,
		}
		if !site.Deleted {
			repo, ok := repos[site.RepoID]
			if !ok || ownerNames[repo.OwnerID] == "" {
				return nil, fmt.Errorf("ListSitesForSync: repository or owner %d missing", site.RepoID)
			}
			record.Owner, record.Repository = ownerNames[repo.OwnerID], repo.LowerName
			if domain := domainByRepo[site.RepoID]; domain != nil {
				record.CustomDomains = []DomainRecord{{
					ID: domain.ID, Host: domain.Host, TXTToken: domain.TXTToken,
					TXTResult: domain.TXTResult, TXTLastSuccessUnixNano: domain.TXTLastSuccessUnixNano,
					State: domain.State,
				}}
			}
		}
		records = append(records, record)
	}
	policyEnabled, policyTime, err := GetTXTVerificationPolicy(ctx)
	if err != nil {
		return nil, err
	}
	response := &SyncResponse{
		RootURL: instance.RootURL, RootUpdatedUnixNano: instance.RootUpdatedUnixNano,
		TXTVerificationEnabled: policyEnabled, TXTPolicyUpdatedUnixNano: policyTime,
		Until: until, NextSince: until, Sites: records,
	}
	if hasMore {
		last := rows[len(rows)-1]
		response.NextCursor = &Cursor{PagesUpdatedUnixNano: last.PagesUpdatedUnixNano, RepoID: last.RepoID}
	}
	return response, nil
}

// OpenCurrentContent verifies the requested digest against the current site.
func OpenCurrentContent(ctx context.Context, repoID int64, digest string) (io.ReadCloser, error) {
	site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("OpenCurrentContent: %w", err)
	}
	if !has || site.Deleted || !site.Enabled || site.TarGzSHA256 == "" {
		return nil, util.ErrNotExist
	}
	if site.TarGzSHA256 != digest {
		return nil, ErrStateChanged
	}
	object, err := pages_content.NewContentStore().Open(repoID, digest)
	if err != nil {
		return nil, fmt.Errorf("OpenCurrentContent: %w", err)
	}
	return object, nil
}
