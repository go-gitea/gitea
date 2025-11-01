// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25_custom

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func Test_CreateSubjectsTable(t *testing.T) {
	// Define the Repository table structure for testing
	type Repository struct {
		ID      int64  `xorm:"pk autoincr"`
		Subject string `xorm:"VARCHAR(255)"`
	}

	// Define the Subject table structure for verification
	type Subject struct {
		ID          int64              `xorm:"pk autoincr"`
		Name        string             `xorm:"VARCHAR(255) UNIQUE NOT NULL"`
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(Repository))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	// Test Case 1: Empty repository table (no subjects)
	t.Run("EmptyRepositoryTable", func(t *testing.T) {
		// Run the migration
		err := CreateSubjectsTable(x)
		assert.NoError(t, err)

		// Verify subject table exists and is empty
		var count int64
		count, err = x.Table("subject").Count()
		assert.NoError(t, err)
		assert.Equal(t, int64(0), count, "Subject table should be empty when no repositories exist")

		// Clean up for next test
		_, err = x.Exec("DROP TABLE IF EXISTS subject")
		assert.NoError(t, err)
	})

	// Test Case 2: Repositories with valid subjects
	t.Run("ValidSubjects", func(t *testing.T) {
		// Insert test repositories with subjects
		repos := []Repository{
			{Subject: "Mathematics"},
			{Subject: "Physics"},
			{Subject: "Chemistry"},
			{Subject: "Biology"},
		}
		for _, repo := range repos {
			_, err := x.Insert(&repo)
			assert.NoError(t, err)
		}

		// Run the migration
		err := CreateSubjectsTable(x)
		assert.NoError(t, err)

		// Verify all subjects were inserted
		var subjects []Subject
		err = x.Table("subject").Asc("name").Find(&subjects)
		assert.NoError(t, err)
		assert.Equal(t, 4, len(subjects), "Should have 4 distinct subjects")

		// Verify subject names are correct (alphabetically sorted)
		expectedNames := []string{"Biology", "Chemistry", "Mathematics", "Physics"}
		for i, subject := range subjects {
			assert.Equal(t, expectedNames[i], subject.Name)
		}

		// Clean up for next test
		_, err = x.Exec("DELETE FROM repository")
		assert.NoError(t, err)
		_, err = x.Exec("DROP TABLE IF EXISTS subject")
		assert.NoError(t, err)
	})

	// Test Case 3: Repositories with empty strings and NULL subjects (should be filtered out)
	t.Run("FilterEmptyAndNullSubjects", func(t *testing.T) {
		// Insert test repositories with various subject values
		repos := []Repository{
			{Subject: "Valid Subject"},
			{Subject: ""},      // Empty string - should be filtered
			{Subject: ""},      // Another empty string
			{Subject: "Valid"}, // Valid subject
		}
		for _, repo := range repos {
			_, err := x.Insert(&repo)
			assert.NoError(t, err)
		}

		// Insert a repository with NULL subject using raw SQL
		_, err := x.Exec("INSERT INTO repository (subject) VALUES (NULL)")
		assert.NoError(t, err)

		// Run the migration
		err = CreateSubjectsTable(x)
		assert.NoError(t, err)

		// Verify only non-empty subjects were inserted
		var subjects []Subject
		err = x.Table("subject").Asc("name").Find(&subjects)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(subjects), "Should have 2 distinct non-empty subjects")

		// Verify subject names
		assert.Equal(t, "Valid", subjects[0].Name)
		assert.Equal(t, "Valid Subject", subjects[1].Name)

		// Clean up for next test
		_, err = x.Exec("DELETE FROM repository")
		assert.NoError(t, err)
		_, err = x.Exec("DROP TABLE IF EXISTS subject")
		assert.NoError(t, err)
	})

	// Test Case 4: Duplicate subjects across repositories (should result in distinct entries)
	t.Run("DuplicateSubjects", func(t *testing.T) {
		// Insert test repositories with duplicate subjects
		repos := []Repository{
			{Subject: "Computer Science"},
			{Subject: "Computer Science"}, // Duplicate
			{Subject: "Mathematics"},
			{Subject: "Computer Science"}, // Another duplicate
			{Subject: "Mathematics"},      // Duplicate
			{Subject: "Physics"},
		}
		for _, repo := range repos {
			_, err := x.Insert(&repo)
			assert.NoError(t, err)
		}

		// Run the migration
		err := CreateSubjectsTable(x)
		assert.NoError(t, err)

		// Verify only distinct subjects were inserted
		var subjects []Subject
		err = x.Table("subject").Asc("name").Find(&subjects)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(subjects), "Should have 3 distinct subjects despite duplicates")

		// Verify subject names
		expectedNames := []string{"Computer Science", "Mathematics", "Physics"}
		for i, subject := range subjects {
			assert.Equal(t, expectedNames[i], subject.Name)
		}

		// Clean up for next test
		_, err = x.Exec("DELETE FROM repository")
		assert.NoError(t, err)
		_, err = x.Exec("DROP TABLE IF EXISTS subject")
		assert.NoError(t, err)
	})

	// Test Case 5: Idempotency (running migration twice should not cause errors or duplicate data)
	t.Run("Idempotency", func(t *testing.T) {
		// Insert test repositories
		repos := []Repository{
			{Subject: "Test Subject 1"},
			{Subject: "Test Subject 2"},
		}
		for _, repo := range repos {
			_, err := x.Insert(&repo)
			assert.NoError(t, err)
		}

		// Run the migration first time
		err := CreateSubjectsTable(x)
		assert.NoError(t, err)

		// Verify subjects were inserted
		var count int64
		count, err = x.Table("subject").Count()
		assert.NoError(t, err)
		assert.Equal(t, int64(2), count)

		// Run the migration again (should handle existing data gracefully)
		// Note: In a real scenario, the migration framework prevents re-running,
		// but we test that the SQL itself is safe
		// The INSERT will fail due to UNIQUE constraint, which is expected behavior
		_, err = x.Exec("INSERT INTO subject (name) SELECT DISTINCT subject FROM repository WHERE subject != '' AND subject IS NOT NULL")
		// We expect an error here due to UNIQUE constraint violation
		assert.Error(t, err, "Second insert should fail due to UNIQUE constraint")

		// Verify no duplicates were created
		count, err = x.Table("subject").Count()
		assert.NoError(t, err)
		assert.Equal(t, int64(2), count, "Should still have only 2 subjects")

		// Clean up for next test
		_, err = x.Exec("DELETE FROM repository")
		assert.NoError(t, err)
		_, err = x.Exec("DROP TABLE IF EXISTS subject")
		assert.NoError(t, err)
	})

	// Test Case 6: Mixed valid, empty, null, and duplicate subjects
	t.Run("MixedScenario", func(t *testing.T) {
		// Insert a comprehensive mix of test data
		repos := []Repository{
			{Subject: "Astronomy"},
			{Subject: ""},
			{Subject: "Astronomy"}, // Duplicate
			{Subject: "Geology"},
			{Subject: ""},
			{Subject: "Astronomy"}, // Another duplicate
			{Subject: "Geology"},   // Duplicate
		}
		for _, repo := range repos {
			_, err := x.Insert(&repo)
			assert.NoError(t, err)
		}

		// Insert NULL subjects
		_, err := x.Exec("INSERT INTO repository (subject) VALUES (NULL)")
		assert.NoError(t, err)
		_, err = x.Exec("INSERT INTO repository (subject) VALUES (NULL)")
		assert.NoError(t, err)

		// Run the migration
		err = CreateSubjectsTable(x)
		assert.NoError(t, err)

		// Verify only distinct non-empty subjects were inserted
		var subjects []Subject
		err = x.Table("subject").Asc("name").Find(&subjects)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(subjects), "Should have 2 distinct non-empty subjects")

		// Verify subject names
		assert.Equal(t, "Astronomy", subjects[0].Name)
		assert.Equal(t, "Geology", subjects[1].Name)

		// Clean up
		_, err = x.Exec("DELETE FROM repository")
		assert.NoError(t, err)
		_, err = x.Exec("DROP TABLE IF EXISTS subject")
		assert.NoError(t, err)
	})

	// Test Case 7: Verify table structure
	t.Run("TableStructure", func(t *testing.T) {
		// Run the migration
		err := CreateSubjectsTable(x)
		assert.NoError(t, err)

		// Verify table exists and has correct structure
		tables, err := x.DBMetas()
		assert.NoError(t, err)

		var subjectTableFound bool
		for _, table := range tables {
			if table.Name == "subject" {
				subjectTableFound = true
				// Verify columns exist
				assert.NotNil(t, table.GetColumn("id"), "ID column should exist")
				assert.NotNil(t, table.GetColumn("name"), "Name column should exist")
				assert.NotNil(t, table.GetColumn("created_unix"), "CreatedUnix column should exist")
				assert.NotNil(t, table.GetColumn("updated_unix"), "UpdatedUnix column should exist")
				break
			}
		}

		assert.True(t, subjectTableFound, "Subject table should exist")

		// Clean up
		_, err = x.Exec("DROP TABLE IF EXISTS subject")
		assert.NoError(t, err)
	})
}

