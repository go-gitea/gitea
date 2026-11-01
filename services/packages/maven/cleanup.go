// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package maven

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"gitea.dev/models/db"
	packages_model "gitea.dev/models/packages"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	maven_module "gitea.dev/modules/packages/maven"
	"gitea.dev/modules/setting"
	packages_service "gitea.dev/services/packages"
)

// CleanupSnapshotVersions removes all but the most recent builds of Maven snapshot versions
func CleanupSnapshotVersions(ctx context.Context) error {
	retainBuilds := setting.Packages.RetainMavenSnapshotBuilds
	if retainBuilds < 1 {
		return nil
	}

	pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		Type:            packages_model.TypeMaven,
		Version:         packages_model.SearchValue{Value: "%-snapshot"},
		IsInternal:      optional.Some(false),
		HasFileWithName: maven_module.MetadataFileName,
	})
	if err != nil {
		return err
	}

	for _, pv := range pvs {
		select {
		case <-ctx.Done():
			return db.ErrCancelledf("While cleaning up Maven snapshot versions")
		default:
		}

		if err := cleanupSnapshotVersion(ctx, pv, retainBuilds); err != nil {
			log.Error("Maven snapshot cleanup of package version %d failed: %v", pv.ID, err)
		}
	}
	return nil
}

func cleanupSnapshotVersion(ctx context.Context, pv *packages_model.PackageVersion, retainBuilds int) error {
	var metadata maven_module.Metadata
	if err := json.Unmarshal([]byte(pv.MetadataJSON), &metadata); err != nil {
		return err
	}
	if metadata.ArtifactID == "" {
		return nil // no pom uploaded yet
	}

	pfs, err := packages_model.GetFilesByVersionID(ctx, pv.ID)
	if err != nil {
		return err
	}

	buildNumber := func(pf *packages_model.PackageFile) (int, bool) {
		return maven_module.ParseSnapshotBuildNumber(metadata.ArtifactID, pv.Version, pf.Name)
	}

	// the metadata build number is not validated on upload, so rely on the stored files
	var metadataFile *packages_model.PackageFile
	var builds []int
	for _, pf := range pfs {
		if pf.Name == maven_module.MetadataFileName {
			metadataFile = pf
		} else if build, ok := buildNumber(pf); ok {
			builds = append(builds, build)
		}
	}
	slices.Sort(builds)
	builds = slices.Compact(builds)
	if metadataFile == nil || len(builds) <= retainBuilds {
		return nil
	}
	threshold := builds[len(builds)-retainBuilds-1]

	referenced, err := readSnapshotVersions(ctx, metadataFile)
	if err != nil {
		return err
	}

	pfs = slices.DeleteFunc(pfs, func(pf *packages_model.PackageFile) bool {
		build, ok := buildNumber(pf)
		return !ok || build > threshold || slices.ContainsFunc(referenced, func(v maven_module.SnapshotVersion) bool {
			name := v.FileName(metadata.ArtifactID)
			return pf.Name == name || strings.HasPrefix(pf.Name, name+".") // keep signatures like .asc
		})
	})

	return db.WithTx(ctx, func(ctx context.Context) error {
		for _, pf := range pfs {
			if err := packages_service.DeletePackageFile(ctx, pf); err != nil {
				return err
			}
		}
		return nil
	})
}

// the metadata may reference files of older builds
func readSnapshotVersions(ctx context.Context, metadataFile *packages_model.PackageFile) ([]maven_module.SnapshotVersion, error) {
	pb, err := packages_model.GetBlobByID(ctx, metadataFile.BlobID)
	if err != nil {
		return nil, err
	}
	s, err := packages_service.OpenBlobStream(pb)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	versions, err := maven_module.ParseSnapshotMetadata(s)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", maven_module.MetadataFileName, err)
	}
	return versions, nil
}
