// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package flatpak

import (
	"fmt"
	"net/http"
	"strings"

	packages_model "gitea.dev/models/packages"
	container_model "gitea.dev/models/packages/container"
	"gitea.dev/modules/httplib"
	container_module "gitea.dev/modules/packages/container"
	"gitea.dev/routers/api/packages/helper"
	"gitea.dev/services/context"
)

// The index of an OCI remote, as read by flatpak_oci_index_make_summary in
// https://github.com/flatpak/flatpak/blob/main/common/flatpak-oci-registry.c
type indexResponse struct {
	Registry string        `json:"Registry"`
	Results  []*repository `json:"Results"`
}

type repository struct {
	Name   string   `json:"Name"`
	Images []*image `json:"Images"`
}

type image struct {
	Tags         []string          `json:"Tags"`
	Digest       string            `json:"Digest"`
	MediaType    string            `json:"MediaType"`
	OS           string            `json:"OS"`
	Architecture string            `json:"Architecture"`
	Labels       map[string]string `json:"Labels"`
}

func apiError(ctx *context.Context, status int, obj any) {
	message := helper.ProcessErrorForUser(ctx, status, obj)
	ctx.PlainText(status, message)
}

// Index lists the owner's Flatpaks for an `oci+https://` remote; the images are pulled from the container registry
func Index(ctx *context.Context) {
	tag := strings.ToLower(ctx.FormTrim("tag"))
	osName := ctx.FormTrim("os")
	arch := ctx.FormTrim("architecture")

	ps, err := packages_model.GetPackagesByType(ctx, ctx.Package.Owner.ID, packages_model.TypeFlatpak)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	resp := &indexResponse{
		Registry: httplib.GuessCurrentHostURL(ctx) + "/",
		Results:  []*repository{},
	}
	for _, p := range ps {
		pvs, _, err := container_model.SearchImageTags(ctx, &container_model.ImageTagsSearchOptions{
			PackageID: p.ID,
			IsTagged:  true,
		})
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		repo := &repository{Name: ctx.Package.Owner.LowerName + "/" + p.LowerName}
		for _, pv := range pvs {
			if tag != "" && pv.LowerVersion != tag {
				continue
			}
			images, err := versionImages(ctx, p, pv)
			if err != nil {
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}
			for _, img := range images {
				if img.Labels[container_module.LabelFlatpakRef] == "" || (osName != "" && img.OS != osName) || (arch != "" && img.Architecture != arch) {
					continue
				}
				img.Tags = []string{pv.LowerVersion}
				repo.Images = append(repo.Images, img)
			}
		}
		if len(repo.Images) > 0 {
			resp.Results = append(resp.Results, repo)
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// versionImages returns the image of a tagged manifest, or the images listed by a tagged index
func versionImages(ctx *context.Context, p *packages_model.Package, pv *packages_model.PackageVersion) ([]*image, error) {
	pd, err := packages_model.GetPackageDescriptor(ctx, pv)
	if err != nil {
		return nil, err
	}
	metadata, err := imageMetadata(pd)
	if err != nil {
		return nil, err
	}
	if len(metadata.Manifests) == 0 {
		return []*image{descriptorImage(pd, metadata)}, nil
	}

	images := make([]*image, 0, len(metadata.Manifests))
	for _, m := range metadata.Manifests {
		children, err := container_model.GetManifestVersions(ctx, &container_model.BlobSearchOptions{
			OwnerID:    p.OwnerID,
			Image:      p.LowerName,
			Digest:     m.Digest,
			IsManifest: true,
		})
		if err != nil {
			return nil, err
		}
		if len(children) == 0 {
			continue
		}
		childPd, err := packages_model.GetPackageDescriptor(ctx, children[0])
		if err != nil {
			return nil, err
		}
		childMetadata, err := imageMetadata(childPd)
		if err != nil {
			return nil, err
		}
		images = append(images, descriptorImage(childPd, childMetadata))
	}
	return images, nil
}

func imageMetadata(pd *packages_model.PackageDescriptor) (*container_module.Metadata, error) {
	metadata, ok := pd.Metadata.(*container_module.Metadata)
	if !ok {
		return nil, fmt.Errorf("imageMetadata: unexpected metadata %T of version %d", pd.Metadata, pd.Version.ID)
	}
	return metadata, nil
}

func descriptorImage(pd *packages_model.PackageDescriptor, metadata *container_module.Metadata) *image {
	img := &image{Labels: metadata.Labels}
	img.OS, img.Architecture, _ = strings.Cut(metadata.Platform, "/")
	img.Architecture, _, _ = strings.Cut(img.Architecture, "/")
	for _, pf := range pd.Files {
		if pf.File.IsLead {
			img.Digest = pf.Properties.GetByName(container_module.PropertyDigest)
			img.MediaType = pf.Properties.GetByName(container_module.PropertyMediaType)
			break
		}
	}
	return img
}
