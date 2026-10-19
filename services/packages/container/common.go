// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"context"
	"io"
	"strings"

	"gitea.dev/models/db"
	org_model "gitea.dev/models/organization"
	packages_model "gitea.dev/models/packages"
	container_service "gitea.dev/models/packages/container"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/packages"
	container_module "gitea.dev/modules/packages/container"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// UpdateRepositoryNames updates the repository name property for all packages of the specific owner
func UpdateRepositoryNames(ctx context.Context, owner *user_model.User, newOwnerName string) error {
	var ps []*packages_model.Package
	if err := db.GetEngine(ctx).Where("owner_id = ? AND type = ?", owner.ID, packages_model.TypeContainer).Find(&ps); err != nil {
		return err
	}

	newOwnerName = strings.ToLower(newOwnerName)

	for _, p := range ps {
		repoName := newOwnerName + "/" + p.LowerName
		if p.TeamID > 0 {
			team, err := org_model.GetTeamByID(ctx, p.TeamID)
			if err != nil {
				return err
			}
			repoName = newOwnerName + "/teams/" + team.LowerName + "/" + p.LowerName
		}

		if err := packages_model.DeletePropertiesByName(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepository); err != nil {
			return err
		}

		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepository, repoName); err != nil {
			return err
		}
	}

	return nil
}

// UpdateTeamRepositoryNames updates the repository name property for all packages of the specific owner and team
func UpdateTeamRepositoryNames(ctx context.Context, owner *user_model.User, team *org_model.Team, newTeamName string) error {
	ps, err := packages_model.GetTeamPackagesByType(ctx, owner.ID, team.ID, packages_model.TypeContainer)
	if err != nil {
		return err
	}

	newTeamName = strings.ToLower(newTeamName)

	for _, p := range ps {
		if err := packages_model.DeletePropertiesByName(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepository); err != nil {
			return err
		}

		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepository, owner.LowerName+"/teams/"+newTeamName+"/"+p.LowerName); err != nil {
			return err
		}
	}

	return nil
}

func ParseManifestMetadata(ctx context.Context, rd io.Reader, ownerID, teamID int64, imageName string) (*v1.Manifest, *packages_model.PackageFileDescriptor, *container_module.Metadata, error) {
	var manifest v1.Manifest
	if err := json.NewDecoder(rd).Decode(&manifest); err != nil {
		return nil, nil, nil, err
	}
	configDescriptor, err := container_service.GetContainerBlob(ctx, &container_service.BlobSearchOptions{
		OwnerID: ownerID,
		TeamID:  teamID,
		Image:   imageName,
		Digest:  manifest.Config.Digest.String(),
	})
	if err != nil {
		return nil, nil, nil, err
	}

	configReader, err := packages.NewContentStore().OpenBlob(packages.BlobHash256Key(configDescriptor.Blob.HashSHA256))
	if err != nil {
		return nil, nil, nil, err
	}
	defer configReader.Close()
	metadata, err := container_module.ParseImageConfig(manifest.Config.MediaType, configReader)
	return &manifest, configDescriptor, metadata, err
}
