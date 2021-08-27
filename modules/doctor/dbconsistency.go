// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package doctor

import (
	"context"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/migrations"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func checkDBConsistency(logger log.Logger, autofix bool) error {
	// make sure DB version is uptodate
	if err := models.NewEngine(context.Background(), migrations.EnsureUpToDate); err != nil {
		logger.Critical("Model version on the database does not match the current Gitea version. Model consistency will not be checked until the database is upgraded")
		return err
	}

	// find labels without existing repo or org
	count, err := models.CountOrphanedLabels()
	if err != nil {
		logger.Critical("Error: %v whilst counting orphaned labels", err)
		return err
	}
	if count > 0 {
		if autofix {
			if err = models.DeleteOrphanedLabels(); err != nil {
				logger.Critical("Error: %v whilst deleting orphaned labels", err)
				return err
			}
			logger.Info("%d labels without existing repository/organisation deleted", count)
		} else {
			logger.Warn("%d labels without existing repository/organisation", count)
		}
	}

	// find IssueLabels without existing label
	count, err = models.CountOrphanedIssueLabels()
	if err != nil {
		logger.Critical("Error: %v whilst counting orphaned issue_labels", err)
		return err
	}
	if count > 0 {
		if autofix {
			if err = models.DeleteOrphanedIssueLabels(); err != nil {
				logger.Critical("Error: %v whilst deleting orphaned issue_labels", err)
				return err
			}
			logger.Info("%d issue_labels without existing label deleted", count)
		} else {
			logger.Warn("%d issue_labels without existing label", count)
		}
	}

	// find issues without existing repository
	count, err = models.CountOrphanedIssues()
	if err != nil {
		logger.Critical("Error: %v whilst counting orphaned issues", err)
		return err
	}
	if count > 0 {
		if autofix {
			if err = models.DeleteOrphanedIssues(); err != nil {
				logger.Critical("Error: %v whilst deleting orphaned issues", err)
				return err
			}
			logger.Info("%d issues without existing repository deleted", count)
		} else {
			logger.Warn("%d issues without existing repository", count)
		}
	}

	// find pulls without existing issues
	count, err = models.CountOrphanedObjects("pull_request", "issue", "pull_request.issue_id=issue.id")
	if err != nil {
		logger.Critical("Error: %v whilst counting orphaned objects", err)
		return err
	}
	if count > 0 {
		if autofix {
			if err = models.DeleteOrphanedObjects("pull_request", "issue", "pull_request.issue_id=issue.id"); err != nil {
				logger.Critical("Error: %v whilst deleting orphaned objects", err)
				return err
			}
			logger.Info("%d pull requests without existing issue deleted", count)
		} else {
			logger.Warn("%d pull requests without existing issue", count)
		}
	}

	// find tracked times without existing issues/pulls
	count, err = models.CountOrphanedObjects("tracked_time", "issue", "tracked_time.issue_id=issue.id")
	if err != nil {
		logger.Critical("Error: %v whilst counting orphaned objects", err)
		return err
	}
	if count > 0 {
		if autofix {
			if err = models.DeleteOrphanedObjects("tracked_time", "issue", "tracked_time.issue_id=issue.id"); err != nil {
				logger.Critical("Error: %v whilst deleting orphaned objects", err)
				return err
			}
			logger.Info("%d tracked times without existing issue deleted", count)
		} else {
			logger.Warn("%d tracked times without existing issue", count)
		}
	}

	// find null archived repositories
	count, err = models.CountNullArchivedRepository()
	if err != nil {
		logger.Critical("Error: %v whilst counting null archived repositories", err)
		return err
	}
	if count > 0 {
		if autofix {
			updatedCount, err := models.FixNullArchivedRepository()
			if err != nil {
				logger.Critical("Error: %v whilst fixing null archived repositories", err)
				return err
			}
			logger.Info("%d repositories with null is_archived updated", updatedCount)
		} else {
			logger.Warn("%d repositories with null is_archived", count)
		}
	}

	// find label comments with empty labels
	count, err = models.CountCommentTypeLabelWithEmptyLabel()
	if err != nil {
		logger.Critical("Error: %v whilst counting label comments with empty labels", err)
		return err
	}
	if count > 0 {
		if autofix {
			updatedCount, err := models.FixCommentTypeLabelWithEmptyLabel()
			if err != nil {
				logger.Critical("Error: %v whilst removing label comments with empty labels", err)
				return err
			}
			logger.Info("%d label comments with empty labels removed", updatedCount)
		} else {
			logger.Warn("%d label comments with empty labels", count)
		}
	}

	// find label comments with labels from outside the repository
	count, err = models.CountCommentTypeLabelWithOutsideLabels()
	if err != nil {
		logger.Critical("Error: %v whilst counting label comments with outside labels", err)
		return err
	}
	if count > 0 {
		if autofix {
			updatedCount, err := models.FixCommentTypeLabelWithOutsideLabels()
			if err != nil {
				logger.Critical("Error: %v whilst removing label comments with outside labels", err)
				return err
			}
			log.Info("%d label comments with outside labels removed", updatedCount)
		} else {
			log.Warn("%d label comments with outside labels", count)
		}
	}

	// find issue_label with labels from outside the repository
	count, err = models.CountIssueLabelWithOutsideLabels()
	if err != nil {
		logger.Critical("Error: %v whilst counting issue_labels from outside the repository or organisation", err)
		return err
	}
	if count > 0 {
		if autofix {
			updatedCount, err := models.FixIssueLabelWithOutsideLabels()
			if err != nil {
				logger.Critical("Error: %v whilst removing issue_labels from outside the repository or organisation", err)
				return err
			}
			logger.Info("%d issue_labels from outside the repository or organisation removed", updatedCount)
		} else {
			logger.Warn("%d issue_labels from outside the repository or organisation", count)
		}
	}

	// TODO: function to recalc all counters

	if setting.Database.UsePostgreSQL {
		count, err = models.CountBadSequences()
		if err != nil {
			logger.Critical("Error: %v whilst checking sequence values", err)
			return err
		}
		if count > 0 {
			if autofix {
				err := models.FixBadSequences()
				if err != nil {
					logger.Critical("Error: %v whilst attempting to fix sequences", err)
					return err
				}
				logger.Info("%d sequences updated", count)
			} else {
				logger.Warn("%d sequences with incorrect values", count)
			}
		}
	}

	// find protected branches without existing repository
	count, err = models.CountOrphanedObjects("protected_branch", "repository", "protected_branch.repo_id=repository.id")
	if err != nil {
		logger.Critical("Error: %v whilst counting orphaned objects", err)
		return err
	}
	if count > 0 {
		if autofix {
			if err = models.DeleteOrphanedObjects("protected_branch", "repository", "protected_branch.repo_id=repository.id"); err != nil {
				logger.Critical("Error: %v whilst deleting orphaned objects", err)
				return err
			}
			logger.Info("%d protected branches without existing repository deleted", count)
		} else {
			logger.Warn("%d protected branches without existing repository", count)
		}
	}

	// find deleted branches without existing repository
	count, err = models.CountOrphanedObjects("deleted_branch", "repository", "deleted_branch.repo_id=repository.id")
	if err != nil {
		logger.Critical("Error: %v whilst counting orphaned objects", err)
		return err
	}
	if count > 0 {
		if autofix {
			if err = models.DeleteOrphanedObjects("deleted_branch", "repository", "deleted_branch.repo_id=repository.id"); err != nil {
				logger.Critical("Error: %v whilst deleting orphaned objects", err)
				return err
			}
			logger.Info("%d deleted branches without existing repository deleted", count)
		} else {
			logger.Warn("%d deleted branches without existing repository", count)
		}
	}

	// find LFS locks without existing repository
	count, err = models.CountOrphanedObjects("lfs_lock", "repository", "lfs_lock.repo_id=repository.id")
	if err != nil {
		logger.Critical("Error: %v whilst counting orphaned objects", err)
		return err
	}
	if count > 0 {
		if autofix {
			if err = models.DeleteOrphanedObjects("lfs_lock", "repository", "lfs_lock.repo_id=repository.id"); err != nil {
				logger.Critical("Error: %v whilst deleting orphaned objects", err)
				return err
			}
			logger.Info("%d LFS locks without existing repository deleted", count)
		} else {
			logger.Warn("%d LFS locks without existing repository", count)
		}
	}

	return nil
}

func init() {
	Register(&Check{
		Title:     "Check consistency of database",
		Name:      "check-db-consistency",
		IsDefault: false,
		Run:       checkDBConsistency,
		Priority:  3,
	})
}
