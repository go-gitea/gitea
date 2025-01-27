package maven

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/packages/maven"
	"code.gitea.io/gitea/modules/setting"
	packages_service "code.gitea.io/gitea/services/packages"
)

// CleanupSnapshotVersion removes outdated files for SNAPHOT versions for all Maven packages.
func CleanupSnapshotVersions(ctx context.Context) error {
	retainBuilds := setting.Packages.RetainMavenSnapshotBuilds
	log.Info("Starting CleanupSnapshotVersion with retainBuilds: %d", retainBuilds)

	if retainBuilds == -1 {
		log.Info("CleanupSnapshotVersion skipped because retainBuilds is set to -1")
		return nil
	}

	if retainBuilds < 1 {
		return fmt.Errorf("forbidden value for retainBuilds: %d. Minimum 1 build should be retained", retainBuilds)
	}

	versions, err := packages.GetVersionsByPackageType(ctx, 0, packages.TypeMaven)
	if err != nil {
		return fmt.Errorf("failed to retrieve Maven package versions: %w", err)
	}

	for _, version := range versions {
		log.Info("Processing version: %s (ID: %d)", version.Version, version.ID)

		if !isSnapshotVersion(version.Version) {
			log.Info("Skipping non-SNAPSHOT version: %s (ID: %d)", version.Version, version.ID)
			continue
		}

		if err := cleanSnapshotFiles(ctx, version.ID, retainBuilds); err != nil {
			log.Error("Failed to clean up snapshot files for version '%s' (ID: %d): %v", version.Version, version.ID, err)
			return err
		}
	}

	log.Info("Completed CleanupSnapshotVersion")
	return nil
}

func isSnapshotVersion(version string) bool {
	return strings.Contains(version, "-SNAPSHOT")
}

func cleanSnapshotFiles(ctx context.Context, versionID int64, retainBuilds int) error {
	log.Info("Starting cleanSnapshotFiles for versionID: %d with retainBuilds: %d", versionID, retainBuilds)

	metadataFile, err := packages.GetFileForVersionByName(ctx, versionID, "maven-metadata.xml", packages.EmptyFileKey)
	if err != nil {
		return fmt.Errorf("failed to retrieve Maven metadata file for version ID %d: %w", versionID, err)
	}

	maxBuildNumber, err := extractMaxBuildNumberFromMetadata(ctx, metadataFile)
	if err != nil {
		return fmt.Errorf("failed to extract max build number from maven-metadata.xml for version ID %d: %w", versionID, err)
	}

	log.Info("Max build number for versionID %d: %d", versionID, maxBuildNumber)

	thresholdBuildNumber := maxBuildNumber - retainBuilds
	if thresholdBuildNumber <= 0 {
		log.Info("No files to clean up, as the threshold build number is less than or equal to zero for versionID %d", versionID)
		return nil
	}

	filesToRemove, err := packages.GetFilesByBuildNumber(ctx, versionID, thresholdBuildNumber)
	if err != nil {
		return fmt.Errorf("failed to retrieve files for version ID %d: %w", versionID, err)
	}

	for _, file := range filesToRemove {
		log.Debug("Removing file '%s' below threshold %d", file.Name, thresholdBuildNumber)
		if err := packages_service.DeletePackageFile(ctx, file); err != nil {
			return fmt.Errorf("failed to delete file '%s': %w", file.Name, err)
		}
	}

	log.Info("Completed cleanSnapshotFiles for versionID: %d", versionID)
	return nil
}

func extractMaxBuildNumberFromMetadata(ctx context.Context, metadataFile *packages.PackageFile) (int, error) {
	content, _, _, err := packages_service.GetPackageFileStream(ctx, metadataFile)
	if err != nil {
		return 0, fmt.Errorf("failed to get package file stream: %w", err)
	}
	defer content.Close()

	buildNumberStr, err := maven.ParseMavenMetaData(content)
	if err != nil {
		return 0, fmt.Errorf("failed to parse maven-metadata.xml: %w", err)
	}

	buildNumber, err := strconv.Atoi(buildNumberStr)
	if err != nil {
		return 0, fmt.Errorf("invalid build number format: %w", err)
	}

	return buildNumber, nil
}
