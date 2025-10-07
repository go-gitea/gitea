// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"context"
	"io"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	container_service "code.gitea.io/gitea/models/packages/container"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/packages"
	container_module "code.gitea.io/gitea/modules/packages/container"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// UpdateRepositoryNames updates the repository name property for all packages of the specific owner
func UpdateRepositoryNames(ctx context.Context, owner *user_model.User, newOwnerName string) error {
	ps, err := packages_model.GetPackagesByType(ctx, owner.ID, packages_model.TypeContainer)
	if err != nil {
		return err
	}

	newOwnerName = strings.ToLower(newOwnerName)

	for _, p := range ps {
		if err := packages_model.DeletePropertiesByName(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepository); err != nil {
			return err
		}

		if _, err := packages_model.InsertProperty(ctx, packages_model.PropertyTypePackage, p.ID, container_module.PropertyRepository, newOwnerName+"/"+p.LowerName); err != nil {
			return err
		}
	}

	return nil
}

func ParseManifestMetadata(ctx context.Context, rd io.Reader, ownerID int64, imageName string) (*v1.Manifest, *packages_model.PackageFileDescriptor, *container_module.Metadata, error) {
	var manifest v1.Manifest
	if err := json.NewDecoder(rd).Decode(&manifest); err != nil {
		return nil, nil, nil, err
	}
	configDescriptor, err := container_service.GetContainerBlob(ctx, &container_service.BlobSearchOptions{
		OwnerID: ownerID,
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
