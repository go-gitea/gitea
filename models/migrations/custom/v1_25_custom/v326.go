// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25_custom //nolint

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"xorm.io/xorm"
)

// generateSlugFromName creates a URL-safe slug from a subject display name
// This is a copy of the function from models/repo/subject.go for use in migration
func generateSlugFromName(name string) string {
	// Normalize Unicode (NFD = decompose accents)
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	normalized, _, _ := transform.String(t, name)

	// Convert to lowercase
	slug := strings.ToLower(normalized)

	// Replace underscores with hyphens
	slug = strings.ReplaceAll(slug, "_", "-")

	// Replace spaces with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")

	// Remove all characters except alphanumeric and hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	slug = reg.ReplaceAllString(slug, "")

	// Replace multiple consecutive hyphens with a single hyphen
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	// If slug is empty after processing, use a default
	if slug == "" {
		slug = "subject"
	}

	// Limit length to 100 characters
	if len(slug) > 100 {
		slug = slug[:100]
		// Trim any trailing hyphen that might have been created by truncation
		slug = strings.TrimRight(slug, "-")
	}

	return slug
}

func AddSubjectSlugColumn(x *xorm.Engine) error {
	// Define a temporary struct for the subject table
	type Subject struct {
		ID   int64  `xorm:"pk autoincr"`
		Name string `xorm:"VARCHAR(255) NOT NULL"`
		Slug string `xorm:"VARCHAR(255)"`
	}

	// Add slug column (without UNIQUE constraint initially)
	if err := x.Sync(new(Subject)); err != nil {
		return fmt.Errorf("failed to add slug column: %w", err)
	}

	// Generate slugs for all existing subjects
	// Note: For large datasets (>10k subjects), consider batch processing
	var subjects []Subject
	if err := x.Table("subject").Find(&subjects); err != nil {
		return fmt.Errorf("failed to fetch existing subjects: %w", err)
	}

	fmt.Printf("Migration v326: Processing %d existing subjects...\n", len(subjects))

	// Pre-migration validation - check for potential slug collisions
	slugPreview := make(map[string][]string) // slug -> list of names
	for _, subject := range subjects {
		slug := generateSlugFromName(subject.Name)
		slugPreview[slug] = append(slugPreview[slug], subject.Name)
	}

	// Report potential collisions before making changes
	potentialCollisions := 0
	for slug, names := range slugPreview {
		if len(names) > 1 {
			potentialCollisions++
			fmt.Printf("  WARNING: Multiple subjects will share slug '%s':\n", slug)
			for _, name := range names {
				fmt.Printf("    - '%s'\n", name)
			}
		}
	}

	if potentialCollisions > 0 {
		fmt.Printf("Migration v326: Found %d slug collisions. These will be resolved by appending numeric suffixes.\n", potentialCollisions)
	}

	// Generate and assign slugs
	usedSlugs := make(map[string]int)
	collisionCount := 0

	for i := range subjects {
		baseSlug := generateSlugFromName(subjects[i].Name)
		slug := baseSlug

		// Handle slug collisions by appending a number
		if count, exists := usedSlugs[slug]; exists {
			usedSlugs[slug] = count + 1
			slug = fmt.Sprintf("%s-%d", baseSlug, count+1)
			collisionCount++
			fmt.Printf("  Assigning slug '%s' to subject '%s' (ID: %d)\n",
				slug, subjects[i].Name, subjects[i].ID)
		} else {
			usedSlugs[slug] = 1
		}

		subjects[i].Slug = slug

		// Update the subject with the generated slug
		if _, err := x.ID(subjects[i].ID).Cols("slug").Update(&subjects[i]); err != nil {
			return fmt.Errorf("failed to update subject %d ('%s') with slug '%s': %w",
				subjects[i].ID, subjects[i].Name, slug, err)
		}
	}

	if collisionCount > 0 {
		fmt.Printf("Migration v326: Resolved %d slug collisions by appending numeric suffixes\n", collisionCount)
	}
	fmt.Printf("Migration v326: Successfully generated slugs for all %d subjects\n", len(subjects))

	// Verify all subjects have slugs before adding constraints
	var subjectsWithoutSlug int64
	subjectsWithoutSlug, err := x.Table("subject").Where("slug IS NULL OR slug = ''").Count()
	if err != nil {
		return fmt.Errorf("failed to verify slug generation: %w", err)
	}
	if subjectsWithoutSlug > 0 {
		return fmt.Errorf("migration safety check failed: %d subjects still have empty slugs", subjectsWithoutSlug)
	}

	// Add UNIQUE and NOT NULL constraints to slug column
	// This is database-specific
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Get database type
	dialect := x.Dialect().URI().DBType

	switch dialect {
	case "mysql":
		// MySQL: Modify column to add UNIQUE and NOT NULL
		if _, err := sess.Exec("ALTER TABLE `subject` MODIFY COLUMN `slug` VARCHAR(255) NOT NULL"); err != nil {
			_ = sess.Rollback()
			return fmt.Errorf("failed to add NOT NULL constraint to slug: %w", err)
		}
		if _, err := sess.Exec("ALTER TABLE `subject` ADD UNIQUE INDEX `UQE_subject_slug` (`slug`)"); err != nil {
			_ = sess.Rollback()
			return fmt.Errorf("failed to add UNIQUE constraint to slug: %w", err)
		}

	case "postgres":
		// PostgreSQL: Alter column and add constraint
		if _, err := sess.Exec(`ALTER TABLE "subject" ALTER COLUMN "slug" SET NOT NULL`); err != nil {
			_ = sess.Rollback()
			return fmt.Errorf("failed to add NOT NULL constraint to slug: %w", err)
		}
		if _, err := sess.Exec(`ALTER TABLE "subject" ADD CONSTRAINT "UQE_subject_slug" UNIQUE ("slug")`); err != nil {
			_ = sess.Rollback()
			return fmt.Errorf("failed to add UNIQUE constraint to slug: %w", err)
		}

	case "sqlite3":
		// SQLite: Need to recreate the table with the constraint
		// This is more complex, but we can use a workaround by creating a unique index
		if _, err := sess.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS "UQE_subject_slug" ON "subject" ("slug")`); err != nil {
			_ = sess.Rollback()
			return fmt.Errorf("failed to add UNIQUE constraint to slug: %w", err)
		}

	case "mssql":
		// MSSQL: Alter column and add constraint
		if _, err := sess.Exec(`ALTER TABLE [subject] ALTER COLUMN [slug] VARCHAR(255) NOT NULL`); err != nil {
			_ = sess.Rollback()
			return fmt.Errorf("failed to add NOT NULL constraint to slug: %w", err)
		}
		if _, err := sess.Exec(`ALTER TABLE [subject] ADD CONSTRAINT [UQE_subject_slug] UNIQUE ([slug])`); err != nil {
			_ = sess.Rollback()
			return fmt.Errorf("failed to add UNIQUE constraint to slug: %w", err)
		}

	default:
		_ = sess.Rollback()
		return fmt.Errorf("unsupported database type: %s", dialect)
	}

	// Remove UNIQUE constraint from name column (if it exists)
	switch dialect {
	case "mysql":
		// Try to drop the unique index on name (ignore error if it doesn't exist)
		_, _ = sess.Exec("ALTER TABLE `subject` DROP INDEX `UQE_subject_name`")

	case "postgres":
		// Try to drop the unique constraint on name (ignore error if it doesn't exist)
		_, _ = sess.Exec(`ALTER TABLE "subject" DROP CONSTRAINT IF EXISTS "UQE_subject_name"`)

	case "sqlite3":
		// Try to drop the unique index on name (ignore error if it doesn't exist)
		_, _ = sess.Exec(`DROP INDEX IF EXISTS "UQE_subject_name"`)

	case "mssql":
		// Try to drop the unique constraint on name (ignore error if it doesn't exist)
		_, _ = sess.Exec(`ALTER TABLE [subject] DROP CONSTRAINT IF EXISTS [UQE_subject_name]`)
	}

	if err := sess.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("Migration v326: Successfully added UNIQUE constraint to slug column\n")
	fmt.Printf("Migration v326: Migration completed successfully!\n")

	return nil
}
