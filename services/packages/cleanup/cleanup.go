// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	packages_module "code.gitea.io/gitea/modules/packages"
	packages_service "code.gitea.io/gitea/services/packages"
	alpine_service "code.gitea.io/gitea/services/packages/alpine"
	arch_service "code.gitea.io/gitea/services/packages/arch"
	cargo_service "code.gitea.io/gitea/services/packages/cargo"
	container_service "code.gitea.io/gitea/services/packages/container"
	debian_service "code.gitea.io/gitea/services/packages/debian"
	rpm_service "code.gitea.io/gitea/services/packages/rpm"
)

// CleanupTask executes cleanup rules and cleanup expired package data
func CleanupTask(ctx context.Context, olderThan time.Duration) error {
	if err := ExecuteCleanupRules(ctx); err != nil {
		return err
	}

	return CleanupExpiredData(ctx, olderThan)
}

func executeCleanupOneRulePackage(ctx context.Context, pcr *packages_model.PackageCleanupRule, p *packages_model.Package) (versionDeleted bool, err error) {
	olderThan := time.Now().AddDate(0, 0, -pcr.RemoveDays)
	pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
		PackageID:  p.ID,
		IsInternal: optional.Some(false),
		Sort:       packages_model.SortCreatedDesc,
	})
	if err != nil {
		return false, fmt.Errorf("CleanupRule [%d]: SearchVersions failed: %w", pcr.ID, err)
	}
	if pcr.KeepCount > 0 {
		if pcr.KeepCount < len(pvs) {
			pvs = pvs[pcr.KeepCount:]
		} else {
			pvs = nil
		}
	}
	for _, pv := range pvs {
		if pcr.Type == packages_model.TypeContainer {
			if skip, err := container_service.ShouldBeSkipped(ctx, pcr, p, pv); err != nil {
				return false, fmt.Errorf("CleanupRule [%d]: container.ShouldBeSkipped failed: %w", pcr.ID, err)
			} else if skip {
				log.Debug("Rule[%d]: keep '%s/%s' (container)", pcr.ID, p.Name, pv.Version)
				continue
			}
		}
		toMatch := pv.LowerVersion
		if pcr.MatchFullName {
			toMatch = p.LowerName + "/" + pv.LowerVersion
		}
		if pcr.KeepPatternMatcher != nil && pcr.KeepPatternMatcher.MatchString(toMatch) {
			log.Debug("Rule[%d]: keep '%s/%s' (keep pattern)", pcr.ID, p.Name, pv.Version)
			continue
		}
		if pv.CreatedUnix.AsLocalTime().After(olderThan) {
			log.Debug("Rule[%d]: keep '%s/%s' (remove days) %v", pcr.ID, p.Name, pv.Version, pv.CreatedUnix.FormatDate())
			continue
		}
		if pcr.RemovePatternMatcher != nil && !pcr.RemovePatternMatcher.MatchString(toMatch) {
			log.Debug("Rule[%d]: keep '%s/%s' (remove pattern)", pcr.ID, p.Name, pv.Version)
			continue
		}
		log.Debug("Rule[%d]: remove '%s/%s'", pcr.ID, p.Name, pv.Version)
		if err := packages_service.DeletePackageVersionAndReferences(ctx, pv); err != nil {
			log.Error("CleanupRule [%d]: DeletePackageVersionAndReferences failed: %v", pcr.ID, err)
			continue
		}
		versionDeleted = true
	}
	return versionDeleted, nil
}

func executeCleanupOneRule(ctx context.Context, pcr *packages_model.PackageCleanupRule) error {
	if err := pcr.CompiledPattern(); err != nil {
		return fmt.Errorf("CleanupRule [%d]: CompilePattern failed: %w", pcr.ID, err)
	}

	packages, err := packages_model.GetPackagesByType(ctx, pcr.OwnerID, pcr.Type)
	if err != nil {
		return fmt.Errorf("CleanupRule [%d]: GetPackagesByType failed: %w", pcr.ID, err)
	}

	anyVersionDeleted := false
	for _, p := range packages {
		versionDeleted := false
		err = db.WithTx(ctx, func(ctx context.Context) (err error) {
			versionDeleted, err = executeCleanupOneRulePackage(ctx, pcr, p)
			return err
		})
		if err != nil {
			log.Error("CleanupRule [%d]: executeCleanupOneRulePackage(%d) failed: %v", pcr.ID, p.ID, err)
			continue
		}
		anyVersionDeleted = anyVersionDeleted || versionDeleted
		if versionDeleted {
			if pcr.Type == packages_model.TypeCargo {
				owner, err := user_model.GetUserByID(ctx, pcr.OwnerID)
				if err != nil {
					return fmt.Errorf("GetUserByID failed: %w", err)
				}
				if err := cargo_service.UpdatePackageIndexIfExists(ctx, owner, owner, p.ID); err != nil {
					return fmt.Errorf("CleanupRule [%d]: cargo.UpdatePackageIndexIfExists failed: %w", pcr.ID, err)
				}
			}
		}
	}

	if anyVersionDeleted {
		switch pcr.Type {
		case packages_model.TypeDebian:
			if err := debian_service.BuildAllRepositoryFiles(ctx, pcr.OwnerID); err != nil {
				return fmt.Errorf("CleanupRule [%d]: debian.BuildAllRepositoryFiles failed: %w", pcr.ID, err)
			}
		case packages_model.TypeAlpine:
			if err := alpine_service.BuildAllRepositoryFiles(ctx, pcr.OwnerID); err != nil {
				return fmt.Errorf("CleanupRule [%d]: alpine.BuildAllRepositoryFiles failed: %w", pcr.ID, err)
			}
		case packages_model.TypeRpm:
			if err := rpm_service.BuildAllRepositoryFiles(ctx, pcr.OwnerID); err != nil {
				return fmt.Errorf("CleanupRule [%d]: rpm.BuildAllRepositoryFiles failed: %w", pcr.ID, err)
			}
		case packages_model.TypeArch:
			release, err := arch_service.AquireRegistryLock(ctx, pcr.OwnerID)
			if err != nil {
				return err
			}
			defer release()

			if err := arch_service.BuildAllRepositoryFiles(ctx, pcr.OwnerID); err != nil {
				return fmt.Errorf("CleanupRule [%d]: arch.BuildAllRepositoryFiles failed: %w", pcr.ID, err)
			}
		}
	}
	return nil
}

func ExecuteCleanupRules(ctx context.Context) error {
	return packages_model.IterateEnabledCleanupRules(ctx, func(ctx context.Context, pcr *packages_model.PackageCleanupRule) error {
		select {
		case <-ctx.Done():
			return db.ErrCancelledf("While processing package cleanup rules")
		default:
		}

		err := executeCleanupOneRule(ctx, pcr)
		if err != nil {
			log.Error("CleanupRule [%d]: executeCleanupOneRule failed: %v", pcr.ID, err)
		}
		return nil
	})
}

func CleanupExpiredData(outerCtx context.Context, olderThan time.Duration) error {
	ctx, committer, err := db.TxContext(outerCtx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := container_service.Cleanup(ctx, olderThan); err != nil {
		return err
	}

	ps, err := packages_model.FindUnreferencedPackages(ctx)
	if err != nil {
		return err
	}
	for _, p := range ps {
		if err := packages_model.DeleteAllProperties(ctx, packages_model.PropertyTypePackage, p.ID); err != nil {
			return err
		}
		if err := packages_model.DeletePackageByID(ctx, p.ID); err != nil {
			return err
		}
	}

	pbs, err := packages_model.FindExpiredUnreferencedBlobs(ctx, olderThan)
	if err != nil {
		return err
	}

	for _, pb := range pbs {
		if err := packages_model.DeleteBlobByID(ctx, pb.ID); err != nil {
			return err
		}
	}

	if err := committer.Commit(); err != nil {
		return err
	}

	contentStore := packages_module.NewContentStore()
	for _, pb := range pbs {
		if err := contentStore.Delete(packages_module.BlobHash256Key(pb.HashSHA256)); err != nil {
			log.Error("Error deleting package blob [%v]: %v", pb.ID, err)
		}
	}

	return nil
}
