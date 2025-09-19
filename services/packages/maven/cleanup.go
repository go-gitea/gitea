package maven

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/packages/maven"
	"code.gitea.io/gitea/modules/setting"
	packages_service "code.gitea.io/gitea/services/packages"
)

// CleanupSnapshotVersions removes outdated files for SNAPHOT versions for all Maven packages.
func CleanupSnapshotVersions(ctx context.Context) error {
	retainBuilds := setting.Packages.RetainMavenSnapshotBuilds
	debugSession := setting.Packages.DebugMavenCleanup
	log.Debug("Maven Cleanup: starting with retainBuilds: %d, debugSession: %t", retainBuilds, debugSession)

	if retainBuilds < 1 {
		log.Warn("Maven Cleanup: skipped as value for retainBuilds less than 1: %d. Minimum 1 build should be retained", retainBuilds)
		return nil
	}

	versions, err := packages.GetVersionsByPackageType(ctx, 0, packages.TypeMaven)
	if err != nil {
		return fmt.Errorf("maven Cleanup: failed to retrieve Maven package versions: %w", err)
	}

	var errs []error
	var metadataErrors []error

	for _, version := range versions {
		if !isSnapshotVersion(version.Version) {
			continue
		}

		var artifactId, groupId string
		if version.MetadataJSON != "" {
			var metadata map[string]interface{}
			if err := json.Unmarshal([]byte(version.MetadataJSON), &metadata); err != nil {
				log.Warn("Maven Cleanup: error during cleanup: failed to unmarshal metadataJSON for package version ID: %d: %w", version.ID, err)
			} else {
				artifactId, _ = metadata["artifact_id"].(string)
				groupId, _ = metadata["group_id"].(string)
				log.Debug("Maven Cleanup: processing package version with ID: %s, Group ID: %s, Artifact ID: %s, Version: %s", version.ID, groupId, artifactId, version.Version)
			}
		}

		if err := cleanSnapshotFiles(ctx, version.ID, retainBuilds, debugSession); err != nil {
			formattedErr := fmt.Errorf("version '%s' (ID: %d, Group ID: %s, Artifact ID: %s): %w",
				version.Version, version.ID, groupId, artifactId, err)

			if errors.Is(err, packages.ErrMetadataFile) {
				metadataErrors = append(metadataErrors, formattedErr)
			} else {
				errs = append(errs, formattedErr)
			}
		}
	}

	for _, err := range metadataErrors {
		log.Warn("Maven Cleanup: error during cleanup: %v", err)
	}

	if len(errs) > 0 {
		for _, err := range errs {
			log.Error("Maven Cleanup: error during cleanup: %v", err)
		}
		return fmt.Errorf("maven Cleanup: completed with errors: %v", errs)
	}

	log.Debug("Completed Maven Cleanup")
	return nil
}

func isSnapshotVersion(version string) bool {
	return strings.HasSuffix(version, "-SNAPSHOT")
}

func cleanSnapshotFiles(ctx context.Context, versionID int64, retainBuilds int, debugSession bool) error {
	log.Debug("Maven Cleanup: starting cleanSnapshotFiles for versionID: %d with retainBuilds: %d, debugSession: %t", versionID, retainBuilds, debugSession)

	metadataFile, err := packages.GetFileForVersionByName(ctx, versionID, "maven-metadata.xml", packages.EmptyFileKey)
	if err != nil {
		return fmt.Errorf("%w: failed to retrieve maven-metadata.xml: %w", packages.ErrMetadataFile, err)
	}

	maxBuildNumber, classifiers, err := extractMaxBuildNumber(ctx, metadataFile)
	if err != nil {
		return fmt.Errorf("%w: failed to extract max build number from maven-metadata.xml: %w", packages.ErrMetadataFile, err)
	}

	thresholdBuildNumber := maxBuildNumber - retainBuilds
	if thresholdBuildNumber <= 0 {
		log.Debug("Maven Cleanup: no files to clean up, as the threshold build number is less than or equal to zero for versionID %d", versionID)
		return nil
	}

	filesToRemove, skippedFiles, err := packages.GetFilesBelowBuildNumber(ctx, versionID, thresholdBuildNumber, classifiers...)
	if err != nil {
		return fmt.Errorf("cleanSnapshotFiles: failed to retrieve files for version: %w", err)
	}

	if debugSession {
		var fileNamesToRemove, skippedFileNames []string

		for _, file := range filesToRemove {
			fileNamesToRemove = append(fileNamesToRemove, file.Name)
		}

		for _, file := range skippedFiles {
			skippedFileNames = append(skippedFileNames, file.Name)
		}

		log.Debug("Maven Cleanup: debug session active. Files to remove: %v, Skipped files: %v", fileNamesToRemove, skippedFileNames)
		return nil
	}

	for _, file := range filesToRemove {
		log.Debug("Maven Cleanup: removing file '%s' below threshold %d", file.Name, thresholdBuildNumber)
		if err := packages_service.DeletePackageFile(ctx, file); err != nil {
			return fmt.Errorf("cleanSnapshotFiles: failed to delete file '%s': %w", file.Name, err)
		}
	}

	return nil
}

func extractMaxBuildNumber(ctx context.Context, metadataFile *packages.PackageFile) (int, []string, error) {
	pb, err := packages.GetBlobByID(ctx, metadataFile.BlobID)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get package blob: %w", err)
	}

	content, _, _, err := packages_service.OpenBlobForDownload(ctx, metadataFile, pb, "", nil, true)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get package file stream: %w", err)
	}
	defer content.Close()

	snapshotMetadata, err := maven.ParseSnapshotVersionMetaData(content)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to parse maven-metadata.xml: %w", err)
	}

	buildNumber := snapshotMetadata.BuildNumber
	classifiers := snapshotMetadata.Classifiers

	return buildNumber, classifiers, nil
}
