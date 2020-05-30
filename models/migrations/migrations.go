// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

const minDBVersion = 70 // Gitea 1.5.3

// Migration describes on migration from lower version to high version
type Migration interface {
	Description() string
	Migrate(*xorm.Engine) error
}

type migration struct {
	description string
	migrate     func(*xorm.Engine) error
}

// NewMigration creates a new migration
func NewMigration(desc string, fn func(*xorm.Engine) error) Migration {
	return &migration{desc, fn}
}

// Description returns the migration's description
func (m *migration) Description() string {
	return m.description
}

// Migrate executes the migration
func (m *migration) Migrate(x *xorm.Engine) error {
	return m.migrate(x)
}

// Version describes the version table. Should have only one row with id==1
type Version struct {
	ID      int64 `xorm:"pk autoincr"`
	Version int64
}

// This is a sequence of migrations. Add new migrations to the bottom of the list.
// If you want to "retire" a migration, remove it from the top of the list and
// update minDBVersion accordingly
var migrations = []Migration{

	// Gitea 1.5.3 ends at v70

	// v70 -> v71
	NewMigration("add issue_dependencies", addIssueDependencies),
	// v71 -> v72
	NewMigration("protect each scratch token", addScratchHash),
	// v72 -> v73
	NewMigration("add review", addReview),

	// Gitea 1.6.4 ends at v73

	// v73 -> v74
	NewMigration("add must_change_password column for users table", addMustChangePassword),
	// v74 -> v75
	NewMigration("add approval whitelists to protected branches", addApprovalWhitelistsToProtectedBranches),
	// v75 -> v76
	NewMigration("clear nonused data which not deleted when user was deleted", clearNonusedData),

	// Gitea 1.7.6 ends at v76

	// v76 -> v77
	NewMigration("add pull request rebase with merge commit", addPullRequestRebaseWithMerge),
	// v77 -> v78
	NewMigration("add theme to users", addUserDefaultTheme),
	// v78 -> v79
	NewMigration("rename repo is_bare to repo is_empty", renameRepoIsBareToIsEmpty),
	// v79 -> v80
	NewMigration("add can close issues via commit in any branch", addCanCloseIssuesViaCommitInAnyBranch),
	// v80 -> v81
	NewMigration("add is locked to issues", addIsLockedToIssues),
	// v81 -> v82
	NewMigration("update U2F counter type", changeU2FCounterType),

	// Gitea 1.8.3 ends at v82

	// v82 -> v83
	NewMigration("hot fix for wrong release sha1 on release table", fixReleaseSha1OnReleaseTable),
	// v83 -> v84
	NewMigration("add uploader id for table attachment", addUploaderIDForAttachment),
	// v84 -> v85
	NewMigration("add table to store original imported gpg keys", addGPGKeyImport),
	// v85 -> v86
	NewMigration("hash application token", hashAppToken),
	// v86 -> v87
	NewMigration("add http method to webhook", addHTTPMethodToWebhook),
	// v87 -> v88
	NewMigration("add avatar field to repository", addAvatarFieldToRepository),

	// Gitea 1.9.6 ends at v88

	// v88 -> v89
	NewMigration("add commit status context field to commit_status", addCommitStatusContext),
	// v89 -> v90
	NewMigration("add original author/url migration info to issues, comments, and repo ", addOriginalMigrationInfo),
	// v90 -> v91
	NewMigration("change length of some repository columns", changeSomeColumnsLengthOfRepo),
	// v91 -> v92
	NewMigration("add index on owner_id of repository and type, review_id of comment", addIndexOnRepositoryAndComment),
	// v92 -> v93
	NewMigration("remove orphaned repository index statuses", removeLingeringIndexStatus),
	// v93 -> v94
	NewMigration("add email notification enabled preference to user", addEmailNotificationEnabledToUser),
	// v94 -> v95
	NewMigration("add enable_status_check, status_check_contexts to protected_branch", addStatusCheckColumnsForProtectedBranches),
	// v95 -> v96
	NewMigration("add table columns for cross referencing issues", addCrossReferenceColumns),
	// v96 -> v97
	NewMigration("delete orphaned attachments", deleteOrphanedAttachments),
	// v97 -> v98
	NewMigration("add repo_admin_change_team_access to user", addRepoAdminChangeTeamAccessColumnForUser),
	// v98 -> v99
	NewMigration("add original author name and id on migrated release", addOriginalAuthorOnMigratedReleases),

	// Gitea 1.10.3 ends at v99

	// v99 -> v100
	NewMigration("add task table and status column for repository table", addTaskTable),
	// v100 -> v101
	NewMigration("update migration repositories' service type", updateMigrationServiceTypes),
	// v101 -> v102
	NewMigration("change length of some external login users columns", changeSomeColumnsLengthOfExternalLoginUser),
	// v102 -> v103
	NewMigration("update migration repositories' service type", dropColumnHeadUserNameOnPullRequest),
	// v103 -> v104
	NewMigration("Add WhitelistDeployKeys to protected branch", addWhitelistDeployKeysToBranches),
	// v104 -> v105
	NewMigration("remove unnecessary columns from label", removeLabelUneededCols),
	// v105 -> v106
	NewMigration("add includes_all_repositories to teams", addTeamIncludesAllRepositories),
	// v106 -> v107
	NewMigration("add column `mode` to table watch", addModeColumnToWatch),
	// v107 -> v108
	NewMigration("Add template options to repository", addTemplateToRepo),
	// v108 -> v109
	NewMigration("Add comment_id on table notification", addCommentIDOnNotification),
	// v109 -> v110
	NewMigration("add can_create_org_repo to team", addCanCreateOrgRepoColumnForTeam),
	// v110 -> v111
	NewMigration("change review content type to text", changeReviewContentToText),
	// v111 -> v112
	NewMigration("update branch protection for can push and whitelist enable", addBranchProtectionCanPushAndEnableWhitelist),
	// v112 -> v113
	NewMigration("remove release attachments which repository deleted", removeAttachmentMissedRepo),
	// v113 -> v114
	NewMigration("new feature: change target branch of pull requests", featureChangeTargetBranch),
	// v114 -> v115
	NewMigration("Remove authentication credentials from stored URL", sanitizeOriginalURL),
	// v115 -> v116
	NewMigration("add user_id prefix to existing user avatar name", renameExistingUserAvatarName),
	// v116 -> v117
	NewMigration("Extend TrackedTimes", extendTrackedTimes),
	// v117 -> v118
	NewMigration("Add block on rejected reviews branch protection", addBlockOnRejectedReviews),
	// v118 -> v119
	NewMigration("Add commit id and stale to reviews", addReviewCommitAndStale),
	// v119 -> v120
	NewMigration("Fix migrated repositories' git service type", fixMigratedRepositoryServiceType),
	// v120 -> v121
	NewMigration("Add owner_name on table repository", addOwnerNameOnRepository),
	// v121 -> v122
	NewMigration("add is_restricted column for users table", addIsRestricted),
	// v122 -> v123
	NewMigration("Add Require Signed Commits to ProtectedBranch", addRequireSignedCommits),
	// v123 -> v124
	NewMigration("Add original informations for reactions", addReactionOriginals),
	// v124 -> v125
	NewMigration("Add columns to user and repository", addUserRepoMissingColumns),
	// v125 -> v126
	NewMigration("Add some columns on review for migration", addReviewMigrateInfo),
	// v126 -> v127
	NewMigration("Fix topic repository count", fixTopicRepositoryCount),
	// v127 -> v128
	NewMigration("add repository code language statistics", addLanguageStats),
	// v128 -> v129
	NewMigration("fix merge base for pull requests", fixMergeBase),
	// v129 -> v130
	NewMigration("remove dependencies from deleted repositories", purgeUnusedDependencies),
	// v130 -> v131
	NewMigration("Expand webhooks for more granularity", expandWebhooks),
	// v131 -> v132
	NewMigration("Add IsSystemWebhook column to webhooks table", addSystemWebhookColumn),
	// v132 -> v133
	NewMigration("Add Branch Protection Protected Files Column", addBranchProtectionProtectedFilesColumn),
	// v133 -> v134
	NewMigration("Add EmailHash Table", addEmailHashTable),
	// v134 -> v135
	NewMigration("Refix merge base for merged pull requests", refixMergeBase),
	// v135 -> v136
	NewMigration("Add OrgID column to Labels table", addOrgIDLabelColumn),
	// v136 -> v137
	NewMigration("Add CommitsAhead and CommitsBehind Column to PullRequest Table", addCommitDivergenceToPulls),
	// v137 -> v138
	NewMigration("Add Branch Protection Block Outdated Branch", addBlockOnOutdatedBranch),
	// v138 -> v139
	NewMigration("Add ResolveDoerID to Comment table", addResolveDoerIDCommentColumn),
	// v139 -> v140
	NewMigration("prepend refs/heads/ to issue refs", prependRefsHeadsToIssueRefs),
	// v140 -> v141
	NewMigration("Save detected language file size to database instead of percent", fixLanguageStatsToSaveSize),
}

// GetCurrentDBVersion returns the current db version
func GetCurrentDBVersion(x *xorm.Engine) (int64, error) {
	if err := x.Sync(new(Version)); err != nil {
		return -1, fmt.Errorf("sync: %v", err)
	}

	currentVersion := &Version{ID: 1}
	has, err := x.Get(currentVersion)
	if err != nil {
		return -1, fmt.Errorf("get: %v", err)
	}
	if !has {
		return -1, nil
	}
	return currentVersion.Version, nil
}

// ExpectedVersion returns the expected db version
func ExpectedVersion() int64 {
	return int64(minDBVersion + len(migrations))
}

// EnsureUpToDate will check if the db is at the correct version
func EnsureUpToDate(x *xorm.Engine) error {
	currentDB, err := GetCurrentDBVersion(x)
	if err != nil {
		return err
	}

	if currentDB < 0 {
		return fmt.Errorf("Database has not been initialised")
	}

	if minDBVersion > currentDB {
		return fmt.Errorf("DB version %d (<= %d) is too old for auto-migration. Upgrade to Gitea 1.6.4 first then upgrade to this version", currentDB, minDBVersion)
	}

	expected := ExpectedVersion()

	if currentDB != expected {
		return fmt.Errorf(`Current database version %d is not equal to the expected version %d. Please run "gitea [--config /path/to/app.ini] migrate" to update the database version`, currentDB, expected)
	}

	return nil
}

// Migrate database to current version
func Migrate(x *xorm.Engine) error {
	if err := x.Sync(new(Version)); err != nil {
		return fmt.Errorf("sync: %v", err)
	}

	currentVersion := &Version{ID: 1}
	has, err := x.Get(currentVersion)
	if err != nil {
		return fmt.Errorf("get: %v", err)
	} else if !has {
		// If the version record does not exist we think
		// it is a fresh installation and we can skip all migrations.
		currentVersion.ID = 0
		currentVersion.Version = int64(minDBVersion + len(migrations))

		if _, err = x.InsertOne(currentVersion); err != nil {
			return fmt.Errorf("insert: %v", err)
		}
	}

	v := currentVersion.Version
	if minDBVersion > v {
		log.Fatal(`Gitea no longer supports auto-migration from your previously installed version.
Please try upgrading to a lower version first (suggested v1.6.4), then upgrade to this version.`)
		return nil
	}

	if int(v-minDBVersion) > len(migrations) {
		// User downgraded Gitea.
		currentVersion.Version = int64(len(migrations) + minDBVersion)
		_, err = x.ID(1).Update(currentVersion)
		return err
	}
	for i, m := range migrations[v-minDBVersion:] {
		log.Info("Migration[%d]: %s", v+int64(i), m.Description())
		if err = m.Migrate(x); err != nil {
			return fmt.Errorf("do migrate: %v", err)
		}
		currentVersion.Version = v + int64(i) + 1
		if _, err = x.ID(1).Update(currentVersion); err != nil {
			return err
		}
	}
	return nil
}

func dropTableColumns(sess *xorm.Session, tableName string, columnNames ...string) (err error) {
	if tableName == "" || len(columnNames) == 0 {
		return nil
	}
	// TODO: This will not work if there are foreign keys

	switch {
	case setting.Database.UseSQLite3:
		// First drop the indexes on the columns
		res, errIndex := sess.Query(fmt.Sprintf("PRAGMA index_list(`%s`)", tableName))
		if errIndex != nil {
			return errIndex
		}
		for _, row := range res {
			indexName := row["name"]
			indexRes, err := sess.Query(fmt.Sprintf("PRAGMA index_info(`%s`)", indexName))
			if err != nil {
				return err
			}
			if len(indexRes) != 1 {
				continue
			}
			indexColumn := string(indexRes[0]["name"])
			for _, name := range columnNames {
				if name == indexColumn {
					_, err := sess.Exec(fmt.Sprintf("DROP INDEX `%s`", indexName))
					if err != nil {
						return err
					}
				}
			}
		}

		// Here we need to get the columns from the original table
		sql := fmt.Sprintf("SELECT sql FROM sqlite_master WHERE tbl_name='%s' and type='table'", tableName)
		res, err := sess.Query(sql)
		if err != nil {
			return err
		}
		tableSQL := string(res[0]["sql"])

		// Separate out the column definitions
		tableSQL = tableSQL[strings.Index(tableSQL, "("):]

		// Remove the required columnNames
		for _, name := range columnNames {
			tableSQL = regexp.MustCompile(regexp.QuoteMeta("`"+name+"`")+"[^`,)]*?[,)]").ReplaceAllString(tableSQL, "")
		}

		// Ensure the query is ended properly
		tableSQL = strings.TrimSpace(tableSQL)
		if tableSQL[len(tableSQL)-1] != ')' {
			if tableSQL[len(tableSQL)-1] == ',' {
				tableSQL = tableSQL[:len(tableSQL)-1]
			}
			tableSQL += ")"
		}

		// Find all the columns in the table
		columns := regexp.MustCompile("`([^`]*)`").FindAllString(tableSQL, -1)

		tableSQL = fmt.Sprintf("CREATE TABLE `new_%s_new` ", tableName) + tableSQL
		if _, err := sess.Exec(tableSQL); err != nil {
			return err
		}

		// Now restore the data
		columnsSeparated := strings.Join(columns, ",")
		insertSQL := fmt.Sprintf("INSERT INTO `new_%s_new` (%s) SELECT %s FROM %s", tableName, columnsSeparated, columnsSeparated, tableName)
		if _, err := sess.Exec(insertSQL); err != nil {
			return err
		}

		// Now drop the old table
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s`", tableName)); err != nil {
			return err
		}

		// Rename the table
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `new_%s_new` RENAME TO `%s`", tableName, tableName)); err != nil {
			return err
		}

	case setting.Database.UsePostgreSQL:
		cols := ""
		for _, col := range columnNames {
			if cols != "" {
				cols += ", "
			}
			cols += "DROP COLUMN `" + col + "` CASCADE"
		}
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` %s", tableName, cols)); err != nil {
			return fmt.Errorf("Drop table `%s` columns %v: %v", tableName, columnNames, err)
		}
	case setting.Database.UseMySQL:
		// Drop indexes on columns first
		sql := fmt.Sprintf("SHOW INDEX FROM %s WHERE column_name IN ('%s')", tableName, strings.Join(columnNames, "','"))
		res, err := sess.Query(sql)
		if err != nil {
			return err
		}
		for _, index := range res {
			indexName := index["column_name"]
			if len(indexName) > 0 {
				_, err := sess.Exec(fmt.Sprintf("DROP INDEX `%s` ON `%s`", indexName, tableName))
				if err != nil {
					return err
				}
			}
		}

		// Now drop the columns
		cols := ""
		for _, col := range columnNames {
			if cols != "" {
				cols += ", "
			}
			cols += "DROP COLUMN `" + col + "`"
		}
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` %s", tableName, cols)); err != nil {
			return fmt.Errorf("Drop table `%s` columns %v: %v", tableName, columnNames, err)
		}
	case setting.Database.UseMSSQL:
		cols := ""
		for _, col := range columnNames {
			if cols != "" {
				cols += ", "
			}
			cols += "`" + strings.ToLower(col) + "`"
		}
		sql := fmt.Sprintf("SELECT Name FROM SYS.DEFAULT_CONSTRAINTS WHERE PARENT_OBJECT_ID = OBJECT_ID('%[1]s') AND PARENT_COLUMN_ID IN (SELECT column_id FROM sys.columns WHERE lower(NAME) IN (%[2]s) AND object_id = OBJECT_ID('%[1]s'))",
			tableName, strings.Replace(cols, "`", "'", -1))
		constraints := make([]string, 0)
		if err := sess.SQL(sql).Find(&constraints); err != nil {
			sess.Rollback()
			return fmt.Errorf("Find constraints: %v", err)
		}
		for _, constraint := range constraints {
			if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP CONSTRAINT `%s`", tableName, constraint)); err != nil {
				sess.Rollback()
				return fmt.Errorf("Drop table `%s` constraint `%s`: %v", tableName, constraint, err)
			}
		}
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN %s", tableName, cols)); err != nil {
			sess.Rollback()
			return fmt.Errorf("Drop table `%s` columns %v: %v", tableName, columnNames, err)
		}

		return sess.Commit()
	default:
		log.Fatal("Unrecognized DB")
	}

	return nil
}
