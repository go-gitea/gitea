// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"context"
	"fmt"
	"net/url"

	pages_model "gitea.dev/models/pages"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

// DeploymentStatus describes publication of the current Pages version.
type DeploymentStatus string

const (
	DeploymentStatusWaiting    DeploymentStatus = "waiting"
	DeploymentStatusPublishing DeploymentStatus = "running"
	DeploymentStatusPublished  DeploymentStatus = "success"
	DeploymentStatusFailed     DeploymentStatus = "failure"
	DeploymentStatusDisabled   DeploymentStatus = "cancelled"
)

// InstanceDeploymentStatus describes the current result from one Pages instance.
type InstanceDeploymentStatus struct {
	RootURL       string
	DefaultURL    string
	DefaultStatus DeploymentStatus
	CustomURL     string
	CustomStatus  DeploymentStatus
	ErrorCode     pages_model.InstanceErrorCode
}

// CurrentDeploymentStatus contains the derived status of the current version.
type CurrentDeploymentStatus struct {
	TarGzSHA256 string
	Status      DeploymentStatus
	Instances   []*InstanceDeploymentStatus
}

func deploymentTargetStatus(result *pages_model.InstanceResult, digest string, published bool) DeploymentStatus {
	if result.ConfigApplied && result.ActualTarGzSHA256 == digest && published {
		return DeploymentStatusPublished
	}
	if result.ErrorCode != "" {
		return DeploymentStatusFailed
	}
	return DeploymentStatusPublishing
}

// GetCurrentDeploymentStatus derives publication state from the current Gitea facts
// and the latest accepted result from each registered instance.
func GetCurrentDeploymentStatus(ctx context.Context, repo *repo_model.Repository) (*CurrentDeploymentStatus, error) {
	if repo == nil || repo.ID <= 0 {
		return nil, util.ErrNotExist
	}
	site, has, err := pages_model.GetSiteByRepoID(ctx, repo.ID)
	if err != nil {
		return nil, err
	}
	if !has || site.Deleted {
		return nil, util.ErrNotExist
	}
	version, has, err := pages_model.GetCurrentVersion(ctx, site)
	if err != nil {
		return nil, err
	}
	if !has {
		return &CurrentDeploymentStatus{Status: DeploymentStatusDisabled}, nil
	}
	if err := repo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	instances, err := pages_model.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	results, err := pages_model.ListInstanceResultsByRepoID(ctx, repo.ID)
	if err != nil {
		return nil, err
	}
	resultByInstance := make(map[int64]*pages_model.InstanceResult, len(results))
	for _, result := range results {
		resultByInstance[result.InstanceID] = result
	}
	domains, err := pages_model.ListDomainsByRepoID(ctx, repo.ID)
	if err != nil {
		return nil, err
	}
	customHost := ""
	for _, domain := range domains {
		if domain.State == pages_model.DomainStateActive {
			customHost = domain.Host
			break
		}
	}

	deployment := &CurrentDeploymentStatus{
		TarGzSHA256: version.TarGzSHA256,
		Status:      DeploymentStatusWaiting,
		Instances:   make([]*InstanceDeploymentStatus, 0, len(instances)),
	}
	allPublished := true
	for _, instance := range instances {
		if instance.RootURL == "" {
			continue
		}
		status, err := buildInstanceDeploymentStatus(repo, site, instance, resultByInstance[instance.ID], version.TarGzSHA256, customHost)
		if err != nil {
			return nil, err
		}
		if status.DefaultStatus == DeploymentStatusFailed {
			deployment.Status = DeploymentStatusFailed
		}
		allPublished = allPublished && status.DefaultStatus == DeploymentStatusPublished
		deployment.Instances = append(deployment.Instances, status)
	}

	if !site.Enabled {
		deployment.Status = DeploymentStatusDisabled
	} else if deployment.Status != DeploymentStatusFailed {
		switch {
		case len(deployment.Instances) == 0:
			deployment.Status = DeploymentStatusWaiting
		case allPublished:
			deployment.Status = DeploymentStatusPublished
		default:
			deployment.Status = DeploymentStatusPublishing
		}
	}
	return deployment, nil
}

func buildInstanceDeploymentStatus(repo *repo_model.Repository, site *pages_model.Site, instance *pages_model.Instance, result *pages_model.InstanceResult, digest, customHost string) (*InstanceDeploymentStatus, error) {
	defaultURL, err := DefaultSiteURL(instance.RootURL, repo.OwnerName, repo.LowerName)
	if err != nil {
		return nil, fmt.Errorf("buildInstanceDeploymentStatus: default URL: %w", err)
	}
	status := &InstanceDeploymentStatus{
		RootURL: instance.RootURL, DefaultURL: defaultURL,
		DefaultStatus: DeploymentStatusPublishing,
	}
	if customHost != "" {
		rootURL, err := url.Parse(instance.RootURL)
		if err != nil {
			return nil, fmt.Errorf("buildInstanceDeploymentStatus: root URL: %w", err)
		}
		status.CustomURL = rootURL.Scheme + "://" + customHost + "/"
		status.CustomStatus = DeploymentStatusPublishing
	}
	if !site.Enabled {
		status.DefaultStatus = DeploymentStatusDisabled
		if status.CustomURL != "" {
			status.CustomStatus = DeploymentStatusDisabled
		}
		return status, nil
	}
	if result == nil || result.AppliedPagesUpdatedUnixNano != site.PagesUpdatedUnixNano || result.AppliedRootUpdatedUnixNano != instance.RootUpdatedUnixNano {
		return status, nil
	}
	status.DefaultStatus = deploymentTargetStatus(result, digest, result.DefaultPublished)
	if status.CustomURL == "" {
		status.ErrorCode = result.ErrorCode
		return status, nil
	}
	if result.AppliedRequireGiteaTXTRecord == setting.Pages.RequireGiteaTXTRecord {
		status.CustomStatus = deploymentTargetStatus(result, digest, result.CustomPublished)
		status.ErrorCode = result.ErrorCode
	} else if status.DefaultStatus == DeploymentStatusFailed {
		status.ErrorCode = result.ErrorCode
	}
	return status, nil
}
