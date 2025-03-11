package maven

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/packages/maven"
	"code.gitea.io/gitea/modules/setting"
	packages_service "code.gitea.io/gitea/services/packages"
)

// CleanupSnapshotVersions removes outdated files for SNAPHOT versions for all Maven packages.
func CleanupSnapshotVersions(ctx context.Context) error {
	retainBuilds := setting.Packages.RetainMavenSnapshotBuilds
	debugSession := setting.Packages.DebugMavenCleanup
	log.Debug("Starting Maven CleanupSnapshotVersions with retainBuilds: %d, debugSession: %t", retainBuilds, debugSession)

	if retainBuilds == -1 {
		log.Info("Maven CleanupSnapshotVersions skipped because retainBuilds is set to -1")
		return nil
	}

	if retainBuilds < 1 {
		return fmt.Errorf("Maven CleanupSnapshotVersions: forbidden value for retainBuilds: %d. Minimum 1 build should be retained", retainBuilds)
	}

	versions, err := packages.GetVersionsByPackageType(ctx, 0, packages.TypeMaven)
	if err != nil {
		return fmt.Errorf("Maven CleanupSnapshotVersions: failed to retrieve Maven package versions: %w", err)
	}

	var errors []error

	for _, version := range versions {
		if !isSnapshotVersion(version.Version) {
			continue
		}

		if err := cleanSnapshotFiles(ctx, version.ID, retainBuilds, debugSession); err != nil {
			errors = append(errors, fmt.Errorf("Maven CleanupSnapshotVersions: version '%s' (ID: %d): %w", version.Version, version.ID, err))
		}
	}

	if len(errors) > 0 {
		for _, err := range errors {
			log.Warn("Maven CleanupSnapshotVersions: Error during cleanup: %v", err)
		}
		return fmt.Errorf("Maven CleanupSnapshotVersions: cleanup completed with errors: %v", errors)
	}

	log.Debug("Completed Maven CleanupSnapshotVersions")
	return nil
}

func isSnapshotVersion(version string) bool {
	return strings.HasSuffix(version, "-SNAPSHOT")
}

func cleanSnapshotFiles(ctx context.Context, versionID int64, retainBuilds int, debugSession bool) error {
	log.Debug("Starting Maven cleanSnapshotFiles for versionID: %d with retainBuilds: %d, debugSession: %t", versionID, retainBuilds, debugSession)

	metadataFile, err := packages.GetFileForVersionByName(ctx, versionID, "maven-metadata.xml", packages.EmptyFileKey)
	if err != nil {
		return fmt.Errorf("cleanSnapshotFiles: failed to retrieve Maven metadata file for version ID %d: %w", versionID, err)
	}

	maxBuildNumber, classifiers, err := extractMaxBuildNumber(ctx, metadataFile)
	if err != nil {
		return fmt.Errorf("cleanSnapshotFiles: failed to extract max build number from maven-metadata.xml for version ID %d: %w", versionID, err)
	}

	thresholdBuildNumber := maxBuildNumber - retainBuilds
	if thresholdBuildNumber <= 0 {
		log.Debug("cleanSnapshotFiles: No files to clean up, as the threshold build number is less than or equal to zero for versionID %d", versionID)
		return nil
	}

	filesToRemove, skippedFiles, err := packages.GetFilesBelowBuildNumber(ctx, versionID, thresholdBuildNumber, classifiers...)
	if err != nil {
		return fmt.Errorf("cleanSnapshotFiles: failed to retrieve files for version ID %d: %w", versionID, err)
	}

	if debugSession {
		var fileNamesToRemove, skippedFileNames []string

		for _, file := range filesToRemove {
			fileNamesToRemove = append(fileNamesToRemove, file.Name)
		}

		for _, file := range skippedFiles {
			skippedFileNames = append(skippedFileNames, file.Name)
		}

		log.Info("cleanSnapshotFiles: Debug session active. Files to remove: %v, Skipped files: %v", fileNamesToRemove, skippedFileNames)
		return nil
	}

	for _, file := range filesToRemove {
		log.Debug("Removing file '%s' below threshold %d", file.Name, thresholdBuildNumber)
		if err := packages_service.DeletePackageFile(ctx, file); err != nil {
			return fmt.Errorf("Maven cleanSnapshotFiles: failed to delete file '%s': %w", file.Name, err)
		}
	}

	log.Debug("Completed Maven cleanSnapshotFiles for versionID: %d", versionID)
	return nil
}

func extractMaxBuildNumber(ctx context.Context, metadataFile *packages.PackageFile) (int, []string, error) {
	pb, err := packages.GetBlobByID(ctx, metadataFile.BlobID)
	if err != nil {
		return 0, nil, fmt.Errorf("extractMaxBuildNumber: failed to get package blob: %w", err)
	}

	content, _, _, err := packages_service.GetPackageBlobStream(ctx, metadataFile, pb, nil, true)
	if err != nil {
		return 0, nil, fmt.Errorf("extractMaxBuildNumber: failed to get package file stream: %w", err)
	}
	defer content.Close()

	snapshotMetadata, err := maven.ParseSnapshotVersionMetaData(content)
	if err != nil {
		return 0, nil, fmt.Errorf("extractMaxBuildNumber: failed to parse maven-metadata.xml: %w", err)
	}

	buildNumber := snapshotMetadata.BuildNumber
	classifiers := snapshotMetadata.Classifiers

	return buildNumber, classifiers, nil
}
