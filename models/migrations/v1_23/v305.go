// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

const (
	minDBVersion       = 70 // Gitea 1.5.3
	oldMigrationsCount = 230
	expectedVersion    = minDBVersion + oldMigrationsCount
)

var oldMigrationNames = []string{
	"add issue_dependencies",
	"protect each scratch token",
	"add review",
	"add must_change_password column for users table",
	"add approval whitelists to protected branches",
	"clear nonused data which not deleted when user was deleted",
	"add pull request rebase with merge commit",
	"add theme to users",
	"rename repo is_bare to repo is_empty",
	"add can close issues via commit in any branch",
	"add is locked to issues",
	"update U2F counter type",
	"hot fix for wrong release sha1 on release table",
	"add uploader id for table attachment",
	"add table to store original imported gpg keys",
	"hash application token",
	"add http method to webhook",
	"add avatar field to repository",
	"add commit status context field to commit_status",
	"add original author/url migration info to issues, comments, and repo ",
	"change length of some repository columns",
	"add index on owner_id of repository and type, review_id of comment",
	"remove orphaned repository index statuses",
	"add email notification enabled preference to user",
	"add enable_status_check, status_check_contexts to protected_branch",
	"add table columns for cross referencing issues",
	"delete orphaned attachments",
	"add repo_admin_change_team_access to user",
	"add original author name and id on migrated release",
	"add task table and status column for repository table",
	"update migration repositories' service type",
	"change length of some external login users columns",
	"update migration repositories' service type v2",
	"Add WhitelistDeployKeys to protected branch",
	"remove unnecessary columns from label",
	"add includes_all_repositories to teams",
	"add column `mode` to table watch",
	"Add template options to repository",
	"Add comment_id on table notification",
	"add can_create_org_repo to team",
	"change review content type to text",
	"update branch protection for can push and whitelist enable",
	"remove release attachments which repository deleted",
	"new feature: change target branch of pull requests",
	"Remove authentication credentials from stored URL",
	"add user_id prefix to existing user avatar name",
	"Extend TrackedTimes",
	"Add block on rejected reviews branch protection",
	"Add commit id and stale to reviews",
	"Fix migrated repositories' git service type",
	"Add owner_name on table repository",
	"add is_restricted column for users table",
	"Add Require Signed Commits to ProtectedBranch",
	"Add original information for reactions",
	"Add columns to user and repository",
	"Add some columns on review for migration",
	"Fix topic repository count",
	"add repository code language statistics",
	"fix merge base for pull requests",
	"remove dependencies from deleted repositories",
	"Expand webhooks for more granularity",
	"Add IsSystemWebhook column to webhooks table",
	"Add Branch Protection Protected Files Column",
	"Add EmailHash Table",
	"Refix merge base for merged pull requests",
	"Add OrgID column to Labels table",
	"Add CommitsAhead and CommitsBehind Column to PullRequest Table",
	"Add Branch Protection Block Outdated Branch",
	"Add ResolveDoerID to Comment table",
	"prepend refs/heads/ to issue refs",
	"Save detected language file size to database instead of percent",
	"Add KeepActivityPrivate to User table",
	"Ensure Repository.IsArchived is not null",
	"recalculate Stars number for all user",
	"update Matrix Webhook http method to 'PUT'",
	"Increase Language field to 50 in LanguageStats",
	"Add projects info to repository table",
	"create review for 0 review id code comments",
	"remove issue dependency comments who refer to non existing issues",
	"Add Created and Updated to Milestone table",
	"add primary key to repo_topic",
	"set default password algorithm to Argon2",
	"add TrustModel field to Repository",
	"add Team review request support",
	"add timestamps to Star, Label, Follow, Watch and Collaboration",
	"add changed_protected_files column for pull_request table",
	"fix publisher ID for tag releases",
	"ensure repo topics are up-to-date",
	"code comment replies should have the commitID of the review they are replying to",
	"update reactions constraint",
	"Add block on official review requests branch protection",
	"Convert task type from int to string",
	"Convert webhook task type from int to string",
	"Convert topic name from 25 to 50",
	"Add scope and nonce columns to oauth2_grant table",
	"Convert hook task type from char(16) to varchar(16) and trim the column",
	"Where Password is Valid with Empty String delete it",
	"Add user redirect",
	"Recreate user table to fix default values",
	"Update DeleteBranch comments to set the old_ref to the commit_sha",
	"Add Dismissed to Review table",
	"Add Sorting to ProjectBoard table",
	"Add sessions table for go-chi/session",
	"Add time_id column to Comment",
	"Create repo transfer table",
	"Fix Postgres ID Sequences broken by recreate-table",
	"Remove invalid labels from comments",
	"Delete orphaned IssueLabels",
	"Add LFS columns to Mirror",
	"Convert avatar url to text",
	"Delete credentials from past migrations",
	"Always save primary email on email address table",
	"Add issue resource index table",
	"Create PushMirror table",
	"Rename Task errors to message",
	"Add new table repo_archiver",
	"Create protected tag table",
	"Drop unneeded webhook related columns",
	"Add key is verified to gpg key",
	"Unwrap ldap.Sources",
	"Add agit flow pull request support",
	"Alter issue/comment table TEXT fields to LONGTEXT",
	"RecreateIssueResourceIndexTable to have a primary key instead of an unique index",
	"Add repo id column for attachment table",
	"Add Branch Protection Unprotected Files Column",
	"Add table commit_status_index",
	"Add Color to ProjectBoard table",
	"Add renamed_branch table",
	"Add issue content history table",
	"No-op (remote version is using AppState now)",
	"Add table app_state",
	"Drop table remote_version (if exists)",
	"Create key/value table for user settings",
	"Add Sorting to ProjectIssue table",
	"Add key is verified to ssh key",
	"Migrate to higher varchar on user struct",
	"Add authorize column to team_unit table",
	"Add webauthn table and migrate u2f data to webauthn - NO-OPED",
	"Use base32.HexEncoding instead of base64 encoding for cred ID as it is case insensitive - NO-OPED",
	"Increase WebAuthentication CredentialID size to 410 - NO-OPED",
	"v208 was completely broken - remigrate",
	"Create ForeignReference table",
	"Add package tables",
	"Add allow edits from maintainers to PullRequest table",
	"Add auto merge table",
	"allow to view files in PRs",
	"No-op (Improve Action table indices v1)",
	"Alter hook_task table TEXT fields to LONGTEXT",
	"Improve Action table indices v2",
	"Add sync_on_commit column to push_mirror table",
	"Add container repository property",
	"Store WebAuthentication CredentialID as bytes and increase size to at least 1024",
	"Drop old CredentialID column",
	"Rename CredentialIDBytes column to CredentialID",
	"Add badges to users",
	"Alter gpg_key/public_key content TEXT fields to MEDIUMTEXT",
	"Conan and generic packages do not need to be semantically versioned",
	"Create key/value table for system settings",
	"Add TeamInvite table",
	"Update counts of all open milestones",
	"Add ConfidentialClient column (default true) to OAuth2Application table",
	"Add index for hook_task",
	"Alter package_version.metadata_json to LONGTEXT",
	"Add header_authorization_encrypted column to webhook table",
	"Add package cleanup rule table",
	"Add index for access_token",
	"Create secrets table",
	"Drop ForeignReference table",
	"Add updated unix to LFSMetaObject",
	"Add scope for access_token",
	"Add actions tables",
	"Add card_type column to project table",
	"Alter gpg_key_import content TEXT field to MEDIUMTEXT",
	"Add exclusive label",
	"Add NeedApproval to actions tables",
	"Rename Webhook org_id to owner_id",
	"Add missed column owner_id for project table",
	"Fix incorrect project type",
	"Add version column to action_runner table",
	"Improve Action table indices v3",
	"Change Container Metadata",
	"Fix incorrect owner team unit access mode",
	"Fix incorrect admin team unit access mode",
	"Fix ExternalTracker and ExternalWiki accessMode in owner and admin team",
	"Add ActionTaskOutput table",
	"Add ArchivedUnix Column",
	"Add is_internal column to package",
	"Add Actions Artifact table",
	"Add PinOrder Column",
	"Convert scoped access tokens",
	"Drop custom_labels column of action_runner table",
	"Add variable table",
	"Add TriggerEvent to action_run table",
	"Add git_size and lfs_size columns to repository table",
	"Add branch table",
	"Alter Actions Artifact table",
	"Reduce commit status",
	"Add action_tasks_version table",
	"Update Action Ref",
	"Drop deleted branch table",
	"Fix PackageProperty typo",
	"Allow archiving labels",
	"Add Version to ActionRun table",
	"Add Action Schedule Table",
	"Add Actions artifacts expiration date",
	"Add ScheduleID for ActionRun",
	"Add RemoteAddress to mirrors",
	"Add Index to issue_user.issue_id",
	"Add Index to comment.dependent_issue_id",
	"Add Index to action.user_id",
	"Rename user themes",
	"Add auth_token table",
	"Add Index to pull_auto_merge.doer_id",
	"Add combined Index to issue_user.uid and issue_id",
	"Add ignore stale approval column on branch table",
	"Add PreviousDuration to ActionRun",
	"Add support for SHA256 git repositories",
	"Use Slug instead of ID for Badges",
	"Add user_blocking table",
	"Add default_wiki_branch to repository table",
	"Add PayloadVersion to HookTask",
	"Add Index to attachment.comment_id",
	"Ensure every project has exactly one default column - No Op",
	"Ensure every project has exactly one default column",
	"Add unique index for project issue table",
	"Add commit status summary table",
	"Add missing field of commit status summary table",
	"Add everyone_access_mode for repo_unit",
	"Drop wrongly created table o_auth2_application",
	"Add content version to issue and comment table",
	"Add force-push branch protection support",
	"Add skip_secondary_authorization option to oauth2 application table",
	"Add metadata column for comment table",
}

// Version describes the version table. Should have only one row with id==1
type Version struct {
	ID      int64 `xorm:"pk autoincr"`
	Version int64
}

func MigrateToXormigrate(x *xorm.Engine) error {
	if err := x.Sync(new(Version)); err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	currentVersion := &Version{ID: 1}
	has, err := x.Get(currentVersion)
	if err != nil {
		return fmt.Errorf("get: %w", err)
	} else if !has {
		// This should not happen
		return fmt.Errorf("could not get version")
	}

	v := currentVersion.Version
	if minDBVersion > v {
		log.Fatal(`Gitea no longer supports auto-migration from your previously installed version.
Please try upgrading to a lower version first (suggested v1.6.4), then upgrade to this version.`)
		return nil
	}

	// Downgrading Gitea's database version not supported
	if int(v-minDBVersion) > oldMigrationsCount {
		msg := fmt.Sprintf("Your database (migration version: %d) is for a newer Gitea, you can not use the newer database for this old Gitea release (%d).", v, expectedVersion)
		msg += "\nGitea will exit to keep your database safe and unchanged. Please use the correct Gitea release, do not change the migration version manually (incorrect manual operation may lose data)."
		if !setting.IsProd {
			msg += fmt.Sprintf("\nIf you are in development and really know what you're doing, you can force changing the migration version by executing: UPDATE version SET version=%d WHERE id=1;", expectedVersion)
		}
		log.Fatal("Migration Error: %s", msg)
		return nil
	}

	// add migrations that already have been run
	for _, i := range oldMigrationNames[:v-minDBVersion] {
		if _, err := x.Insert(&xormigrate.Migration{ID: i}); err != nil {
			return err
		}
	}

	// Remove old version table
	return x.DropTables(new(Version))
}
