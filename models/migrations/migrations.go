// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/migrations/v1_10"
	"code.gitea.io/gitea/models/migrations/v1_11"
	"code.gitea.io/gitea/models/migrations/v1_12"
	"code.gitea.io/gitea/models/migrations/v1_13"
	"code.gitea.io/gitea/models/migrations/v1_14"
	"code.gitea.io/gitea/models/migrations/v1_15"
	"code.gitea.io/gitea/models/migrations/v1_16"
	"code.gitea.io/gitea/models/migrations/v1_17"
	"code.gitea.io/gitea/models/migrations/v1_18"
	"code.gitea.io/gitea/models/migrations/v1_19"
	"code.gitea.io/gitea/models/migrations/v1_20"
	"code.gitea.io/gitea/models/migrations/v1_21"
	"code.gitea.io/gitea/models/migrations/v1_22"
	"code.gitea.io/gitea/models/migrations/v1_23"
	"code.gitea.io/gitea/models/migrations/v1_24"
	"code.gitea.io/gitea/models/migrations/v1_6"
	"code.gitea.io/gitea/models/migrations/v1_7"
	"code.gitea.io/gitea/models/migrations/v1_8"
	"code.gitea.io/gitea/models/migrations/v1_9"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

const minDBVersion = 70 // Gitea 1.5.3

type migration struct {
	idNumber    int64 // DB version is "the last migration's idNumber" + 1
	description string
	migrate     func(*xorm.Engine) error
}

// newMigration creates a new migration
func newMigration(idNumber int64, desc string, fn func(*xorm.Engine) error) *migration {
	return &migration{idNumber, desc, fn}
}

// Migrate executes the migration
func (m *migration) Migrate(x *xorm.Engine) error {
	return m.migrate(x)
}

// Version describes the version table. Should have only one row with id==1
type Version struct {
	ID      int64 `xorm:"pk autoincr"`
	Version int64 // DB version is "the last migration's idNumber" + 1
}

// Use noopMigration when there is a migration that has been no-oped
var noopMigration = func(_ *xorm.Engine) error { return nil }

var preparedMigrations []*migration

// This is a sequence of migrations. Add new migrations to the bottom of the list.
// If you want to "retire" a migration, remove it from the top of the list and
// update minDBVersion accordingly
func prepareMigrationTasks() []*migration {
	if preparedMigrations != nil {
		return preparedMigrations
	}
	preparedMigrations = []*migration{
		// Gitea 1.5.0 ends at database version 69

		newMigration(70, "add issue_dependencies", v1_6.AddIssueDependencies),
		newMigration(71, "protect each scratch token", v1_6.AddScratchHash),
		newMigration(72, "add review", v1_6.AddReview),

		// Gitea 1.6.0 ends at database version 73

		newMigration(73, "add must_change_password column for users table", v1_7.AddMustChangePassword),
		newMigration(74, "add approval whitelists to protected branches", v1_7.AddApprovalWhitelistsToProtectedBranches),
		newMigration(75, "clear nonused data which not deleted when user was deleted", v1_7.ClearNonusedData),

		// Gitea 1.7.0 ends at database version 76

		newMigration(76, "add pull request rebase with merge commit", v1_8.AddPullRequestRebaseWithMerge),
		newMigration(77, "add theme to users", v1_8.AddUserDefaultTheme),
		newMigration(78, "rename repo is_bare to repo is_empty", v1_8.RenameRepoIsBareToIsEmpty),
		newMigration(79, "add can close issues via commit in any branch", v1_8.AddCanCloseIssuesViaCommitInAnyBranch),
		newMigration(80, "add is locked to issues", v1_8.AddIsLockedToIssues),
		newMigration(81, "update U2F counter type", v1_8.ChangeU2FCounterType),

		// Gitea 1.8.0 ends at database version 82

		newMigration(82, "hot fix for wrong release sha1 on release table", v1_9.FixReleaseSha1OnReleaseTable),
		newMigration(83, "add uploader id for table attachment", v1_9.AddUploaderIDForAttachment),
		newMigration(84, "add table to store original imported gpg keys", v1_9.AddGPGKeyImport),
		newMigration(85, "hash application token", v1_9.HashAppToken),
		newMigration(86, "add http method to webhook", v1_9.AddHTTPMethodToWebhook),
		newMigration(87, "add avatar field to repository", v1_9.AddAvatarFieldToRepository),

		// Gitea 1.9.0 ends at database version 88

		newMigration(88, "add commit status context field to commit_status", v1_10.AddCommitStatusContext),
		newMigration(89, "add original author/url migration info to issues, comments, and repo ", v1_10.AddOriginalMigrationInfo),
		newMigration(90, "change length of some repository columns", v1_10.ChangeSomeColumnsLengthOfRepo),
		newMigration(91, "add index on owner_id of repository and type, review_id of comment", v1_10.AddIndexOnRepositoryAndComment),
		newMigration(92, "remove orphaned repository index statuses", v1_10.RemoveLingeringIndexStatus),
		newMigration(93, "add email notification enabled preference to user", v1_10.AddEmailNotificationEnabledToUser),
		newMigration(94, "add enable_status_check, status_check_contexts to protected_branch", v1_10.AddStatusCheckColumnsForProtectedBranches),
		newMigration(95, "add table columns for cross referencing issues", v1_10.AddCrossReferenceColumns),
		newMigration(96, "delete orphaned attachments", v1_10.DeleteOrphanedAttachments),
		newMigration(97, "add repo_admin_change_team_access to user", v1_10.AddRepoAdminChangeTeamAccessColumnForUser),
		newMigration(98, "add original author name and id on migrated release", v1_10.AddOriginalAuthorOnMigratedReleases),
		newMigration(99, "add task table and status column for repository table", v1_10.AddTaskTable),
		newMigration(100, "update migration repositories' service type", v1_10.UpdateMigrationServiceTypes),
		newMigration(101, "change length of some external login users columns", v1_10.ChangeSomeColumnsLengthOfExternalLoginUser),

		// Gitea 1.10.0 ends at database version 102

		newMigration(102, "update migration repositories' service type", v1_11.DropColumnHeadUserNameOnPullRequest),
		newMigration(103, "Add WhitelistDeployKeys to protected branch", v1_11.AddWhitelistDeployKeysToBranches),
		newMigration(104, "remove unnecessary columns from label", v1_11.RemoveLabelUneededCols),
		newMigration(105, "add includes_all_repositories to teams", v1_11.AddTeamIncludesAllRepositories),
		newMigration(106, "add column `mode` to table watch", v1_11.AddModeColumnToWatch),
		newMigration(107, "Add template options to repository", v1_11.AddTemplateToRepo),
		newMigration(108, "Add comment_id on table notification", v1_11.AddCommentIDOnNotification),
		newMigration(109, "add can_create_org_repo to team", v1_11.AddCanCreateOrgRepoColumnForTeam),
		newMigration(110, "change review content type to text", v1_11.ChangeReviewContentToText),
		newMigration(111, "update branch protection for can push and whitelist enable", v1_11.AddBranchProtectionCanPushAndEnableWhitelist),
		newMigration(112, "remove release attachments which repository deleted", v1_11.RemoveAttachmentMissedRepo),
		newMigration(113, "new feature: change target branch of pull requests", v1_11.FeatureChangeTargetBranch),
		newMigration(114, "Remove authentication credentials from stored URL", v1_11.SanitizeOriginalURL),
		newMigration(115, "add user_id prefix to existing user avatar name", v1_11.RenameExistingUserAvatarName),
		newMigration(116, "Extend TrackedTimes", v1_11.ExtendTrackedTimes),

		// Gitea 1.11.0 ends at database version 117

		newMigration(117, "Add block on rejected reviews branch protection", v1_12.AddBlockOnRejectedReviews),
		newMigration(118, "Add commit id and stale to reviews", v1_12.AddReviewCommitAndStale),
		newMigration(119, "Fix migrated repositories' git service type", v1_12.FixMigratedRepositoryServiceType),
		newMigration(120, "Add owner_name on table repository", v1_12.AddOwnerNameOnRepository),
		newMigration(121, "add is_restricted column for users table", v1_12.AddIsRestricted),
		newMigration(122, "Add Require Signed Commits to ProtectedBranch", v1_12.AddRequireSignedCommits),
		newMigration(123, "Add original information for reactions", v1_12.AddReactionOriginals),
		newMigration(124, "Add columns to user and repository", v1_12.AddUserRepoMissingColumns),
		newMigration(125, "Add some columns on review for migration", v1_12.AddReviewMigrateInfo),
		newMigration(126, "Fix topic repository count", v1_12.FixTopicRepositoryCount),
		newMigration(127, "add repository code language statistics", v1_12.AddLanguageStats),
		newMigration(128, "fix merge base for pull requests", v1_12.FixMergeBase),
		newMigration(129, "remove dependencies from deleted repositories", v1_12.PurgeUnusedDependencies),
		newMigration(130, "Expand webhooks for more granularity", v1_12.ExpandWebhooks),
		newMigration(131, "Add IsSystemWebhook column to webhooks table", v1_12.AddSystemWebhookColumn),
		newMigration(132, "Add Branch Protection Protected Files Column", v1_12.AddBranchProtectionProtectedFilesColumn),
		newMigration(133, "Add EmailHash Table", v1_12.AddEmailHashTable),
		newMigration(134, "Refix merge base for merged pull requests", v1_12.RefixMergeBase),
		newMigration(135, "Add OrgID column to Labels table", v1_12.AddOrgIDLabelColumn),
		newMigration(136, "Add CommitsAhead and CommitsBehind Column to PullRequest Table", v1_12.AddCommitDivergenceToPulls),
		newMigration(137, "Add Branch Protection Block Outdated Branch", v1_12.AddBlockOnOutdatedBranch),
		newMigration(138, "Add ResolveDoerID to Comment table", v1_12.AddResolveDoerIDCommentColumn),
		newMigration(139, "prepend refs/heads/ to issue refs", v1_12.PrependRefsHeadsToIssueRefs),

		// Gitea 1.12.0 ends at database version 140

		newMigration(140, "Save detected language file size to database instead of percent", v1_13.FixLanguageStatsToSaveSize),
		newMigration(141, "Add KeepActivityPrivate to User table", v1_13.AddKeepActivityPrivateUserColumn),
		newMigration(142, "Ensure Repository.IsArchived is not null", v1_13.SetIsArchivedToFalse),
		newMigration(143, "recalculate Stars number for all user", v1_13.RecalculateStars),
		newMigration(144, "update Matrix Webhook http method to 'PUT'", v1_13.UpdateMatrixWebhookHTTPMethod),
		newMigration(145, "Increase Language field to 50 in LanguageStats", v1_13.IncreaseLanguageField),
		newMigration(146, "Add projects info to repository table", v1_13.AddProjectsInfo),
		newMigration(147, "create review for 0 review id code comments", v1_13.CreateReviewsForCodeComments),
		newMigration(148, "remove issue dependency comments who refer to non existing issues", v1_13.PurgeInvalidDependenciesComments),
		newMigration(149, "Add Created and Updated to Milestone table", v1_13.AddCreatedAndUpdatedToMilestones),
		newMigration(150, "add primary key to repo_topic", v1_13.AddPrimaryKeyToRepoTopic),
		newMigration(151, "set default password algorithm to Argon2", v1_13.SetDefaultPasswordToArgon2),
		newMigration(152, "add TrustModel field to Repository", v1_13.AddTrustModelToRepository),
		newMigration(153, "add Team review request support", v1_13.AddTeamReviewRequestSupport),
		newMigration(154, "add timestamps to Star, Label, Follow, Watch and Collaboration", v1_13.AddTimeStamps),

		// Gitea 1.13.0 ends at database version 155

		newMigration(155, "add changed_protected_files column for pull_request table", v1_14.AddChangedProtectedFilesPullRequestColumn),
		newMigration(156, "fix publisher ID for tag releases", v1_14.FixPublisherIDforTagReleases),
		newMigration(157, "ensure repo topics are up-to-date", v1_14.FixRepoTopics),
		newMigration(158, "code comment replies should have the commitID of the review they are replying to", v1_14.UpdateCodeCommentReplies),
		newMigration(159, "update reactions constraint", v1_14.UpdateReactionConstraint),
		newMigration(160, "Add block on official review requests branch protection", v1_14.AddBlockOnOfficialReviewRequests),
		newMigration(161, "Convert task type from int to string", v1_14.ConvertTaskTypeToString),
		newMigration(162, "Convert webhook task type from int to string", v1_14.ConvertWebhookTaskTypeToString),
		newMigration(163, "Convert topic name from 25 to 50", v1_14.ConvertTopicNameFrom25To50),
		newMigration(164, "Add scope and nonce columns to oauth2_grant table", v1_14.AddScopeAndNonceColumnsToOAuth2Grant),
		newMigration(165, "Convert hook task type from char(16) to varchar(16) and trim the column", v1_14.ConvertHookTaskTypeToVarcharAndTrim),
		newMigration(166, "Where Password is Valid with Empty String delete it", v1_14.RecalculateUserEmptyPWD),
		newMigration(167, "Add user redirect", v1_14.AddUserRedirect),
		newMigration(168, "Recreate user table to fix default values", v1_14.RecreateUserTableToFixDefaultValues),
		newMigration(169, "Update DeleteBranch comments to set the old_ref to the commit_sha", v1_14.CommentTypeDeleteBranchUseOldRef),
		newMigration(170, "Add Dismissed to Review table", v1_14.AddDismissedReviewColumn),
		newMigration(171, "Add Sorting to ProjectBoard table", v1_14.AddSortingColToProjectBoard),
		newMigration(172, "Add sessions table for go-chi/session", v1_14.AddSessionTable),
		newMigration(173, "Add time_id column to Comment", v1_14.AddTimeIDCommentColumn),
		newMigration(174, "Create repo transfer table", v1_14.AddRepoTransfer),
		newMigration(175, "Fix Postgres ID Sequences broken by recreate-table", v1_14.FixPostgresIDSequences),
		newMigration(176, "Remove invalid labels from comments", v1_14.RemoveInvalidLabels),
		newMigration(177, "Delete orphaned IssueLabels", v1_14.DeleteOrphanedIssueLabels),

		// Gitea 1.14.0 ends at database version 178

		newMigration(178, "Add LFS columns to Mirror", v1_15.AddLFSMirrorColumns),
		newMigration(179, "Convert avatar url to text", v1_15.ConvertAvatarURLToText),
		newMigration(180, "Delete credentials from past migrations", v1_15.DeleteMigrationCredentials),
		newMigration(181, "Always save primary email on email address table", v1_15.AddPrimaryEmail2EmailAddress),
		newMigration(182, "Add issue resource index table", v1_15.AddIssueResourceIndexTable),
		newMigration(183, "Create PushMirror table", v1_15.CreatePushMirrorTable),
		newMigration(184, "Rename Task errors to message", v1_15.RenameTaskErrorsToMessage),
		newMigration(185, "Add new table repo_archiver", v1_15.AddRepoArchiver),
		newMigration(186, "Create protected tag table", v1_15.CreateProtectedTagTable),
		newMigration(187, "Drop unneeded webhook related columns", v1_15.DropWebhookColumns),
		newMigration(188, "Add key is verified to gpg key", v1_15.AddKeyIsVerified),

		// Gitea 1.15.0 ends at database version 189

		newMigration(189, "Unwrap ldap.Sources", v1_16.UnwrapLDAPSourceCfg),
		newMigration(190, "Add agit flow pull request support", v1_16.AddAgitFlowPullRequest),
		newMigration(191, "Alter issue/comment table TEXT fields to LONGTEXT", v1_16.AlterIssueAndCommentTextFieldsToLongText),
		newMigration(192, "RecreateIssueResourceIndexTable to have a primary key instead of an unique index", v1_16.RecreateIssueResourceIndexTable),
		newMigration(193, "Add repo id column for attachment table", v1_16.AddRepoIDForAttachment),
		newMigration(194, "Add Branch Protection Unprotected Files Column", v1_16.AddBranchProtectionUnprotectedFilesColumn),
		newMigration(195, "Add table commit_status_index", v1_16.AddTableCommitStatusIndex),
		newMigration(196, "Add Color to ProjectBoard table", v1_16.AddColorColToProjectBoard),
		newMigration(197, "Add renamed_branch table", v1_16.AddRenamedBranchTable),
		newMigration(198, "Add issue content history table", v1_16.AddTableIssueContentHistory),
		newMigration(199, "No-op (remote version is using AppState now)", noopMigration),
		newMigration(200, "Add table app_state", v1_16.AddTableAppState),
		newMigration(201, "Drop table remote_version (if exists)", v1_16.DropTableRemoteVersion),
		newMigration(202, "Create key/value table for user settings", v1_16.CreateUserSettingsTable),
		newMigration(203, "Add Sorting to ProjectIssue table", v1_16.AddProjectIssueSorting),
		newMigration(204, "Add key is verified to ssh key", v1_16.AddSSHKeyIsVerified),
		newMigration(205, "Migrate to higher varchar on user struct", v1_16.MigrateUserPasswordSalt),
		newMigration(206, "Add authorize column to team_unit table", v1_16.AddAuthorizeColForTeamUnit),
		newMigration(207, "Add webauthn table and migrate u2f data to webauthn - NO-OPED", v1_16.AddWebAuthnCred),
		newMigration(208, "Use base32.HexEncoding instead of base64 encoding for cred ID as it is case insensitive - NO-OPED", v1_16.UseBase32HexForCredIDInWebAuthnCredential),
		newMigration(209, "Increase WebAuthentication CredentialID size to 410 - NO-OPED", v1_16.IncreaseCredentialIDTo410),
		newMigration(210, "v208 was completely broken - remigrate", v1_16.RemigrateU2FCredentials),

		// Gitea 1.16.2 ends at database version 211

		newMigration(211, "Create ForeignReference table", v1_17.CreateForeignReferenceTable),
		newMigration(212, "Add package tables", v1_17.AddPackageTables),
		newMigration(213, "Add allow edits from maintainers to PullRequest table", v1_17.AddAllowMaintainerEdit),
		newMigration(214, "Add auto merge table", v1_17.AddAutoMergeTable),
		newMigration(215, "allow to view files in PRs", v1_17.AddReviewViewedFiles),
		newMigration(216, "No-op (Improve Action table indices v1)", noopMigration),
		newMigration(217, "Alter hook_task table TEXT fields to LONGTEXT", v1_17.AlterHookTaskTextFieldsToLongText),
		newMigration(218, "Improve Action table indices v2", v1_17.ImproveActionTableIndices),
		newMigration(219, "Add sync_on_commit column to push_mirror table", v1_17.AddSyncOnCommitColForPushMirror),
		newMigration(220, "Add container repository property", v1_17.AddContainerRepositoryProperty),
		newMigration(221, "Store WebAuthentication CredentialID as bytes and increase size to at least 1024", v1_17.StoreWebauthnCredentialIDAsBytes),
		newMigration(222, "Drop old CredentialID column", v1_17.DropOldCredentialIDColumn),
		newMigration(223, "Rename CredentialIDBytes column to CredentialID", v1_17.RenameCredentialIDBytes),

		// Gitea 1.17.0 ends at database version 224

		newMigration(224, "Add badges to users", v1_18.CreateUserBadgesTable),
		newMigration(225, "Alter gpg_key/public_key content TEXT fields to MEDIUMTEXT", v1_18.AlterPublicGPGKeyContentFieldsToMediumText),
		newMigration(226, "Conan and generic packages do not need to be semantically versioned", v1_18.FixPackageSemverField),
		newMigration(227, "Create key/value table for system settings", v1_18.CreateSystemSettingsTable),
		newMigration(228, "Add TeamInvite table", v1_18.AddTeamInviteTable),
		newMigration(229, "Update counts of all open milestones", v1_18.UpdateOpenMilestoneCounts),
		newMigration(230, "Add ConfidentialClient column (default true) to OAuth2Application table", v1_18.AddConfidentialClientColumnToOAuth2ApplicationTable),

		// Gitea 1.18.0 ends at database version 231

		newMigration(231, "Add index for hook_task", v1_19.AddIndexForHookTask),
		newMigration(232, "Alter package_version.metadata_json to LONGTEXT", v1_19.AlterPackageVersionMetadataToLongText),
		newMigration(233, "Add header_authorization_encrypted column to webhook table", v1_19.AddHeaderAuthorizationEncryptedColWebhook),
		newMigration(234, "Add package cleanup rule table", v1_19.CreatePackageCleanupRuleTable),
		newMigration(235, "Add index for access_token", v1_19.AddIndexForAccessToken),
		newMigration(236, "Create secrets table", v1_19.CreateSecretsTable),
		newMigration(237, "Drop ForeignReference table", v1_19.DropForeignReferenceTable),
		newMigration(238, "Add updated unix to LFSMetaObject", v1_19.AddUpdatedUnixToLFSMetaObject),
		newMigration(239, "Add scope for access_token", v1_19.AddScopeForAccessTokens),
		newMigration(240, "Add actions tables", v1_19.AddActionsTables),
		newMigration(241, "Add card_type column to project table", v1_19.AddCardTypeToProjectTable),
		newMigration(242, "Alter gpg_key_import content TEXT field to MEDIUMTEXT", v1_19.AlterPublicGPGKeyImportContentFieldToMediumText),
		newMigration(243, "Add exclusive label", v1_19.AddExclusiveLabel),

		// Gitea 1.19.0 ends at database version 244

		newMigration(244, "Add NeedApproval to actions tables", v1_20.AddNeedApprovalToActionRun),
		newMigration(245, "Rename Webhook org_id to owner_id", v1_20.RenameWebhookOrgToOwner),
		newMigration(246, "Add missed column owner_id for project table", v1_20.AddNewColumnForProject),
		newMigration(247, "Fix incorrect project type", v1_20.FixIncorrectProjectType),
		newMigration(248, "Add version column to action_runner table", v1_20.AddVersionToActionRunner),
		newMigration(249, "Improve Action table indices v3", v1_20.ImproveActionTableIndices),
		newMigration(250, "Change Container Metadata", v1_20.ChangeContainerMetadataMultiArch),
		newMigration(251, "Fix incorrect owner team unit access mode", v1_20.FixIncorrectOwnerTeamUnitAccessMode),
		newMigration(252, "Fix incorrect admin team unit access mode", v1_20.FixIncorrectAdminTeamUnitAccessMode),
		newMigration(253, "Fix ExternalTracker and ExternalWiki accessMode in owner and admin team", v1_20.FixExternalTrackerAndExternalWikiAccessModeInOwnerAndAdminTeam),
		newMigration(254, "Add ActionTaskOutput table", v1_20.AddActionTaskOutputTable),
		newMigration(255, "Add ArchivedUnix Column", v1_20.AddArchivedUnixToRepository),
		newMigration(256, "Add is_internal column to package", v1_20.AddIsInternalColumnToPackage),
		newMigration(257, "Add Actions Artifact table", v1_20.CreateActionArtifactTable),
		newMigration(258, "Add PinOrder Column", v1_20.AddPinOrderToIssue),
		newMigration(259, "Convert scoped access tokens", v1_20.ConvertScopedAccessTokens),

		// Gitea 1.20.0 ends at database version 260

		newMigration(260, "Drop custom_labels column of action_runner table", v1_21.DropCustomLabelsColumnOfActionRunner),
		newMigration(261, "Add variable table", v1_21.CreateVariableTable),
		newMigration(262, "Add TriggerEvent to action_run table", v1_21.AddTriggerEventToActionRun),
		newMigration(263, "Add git_size and lfs_size columns to repository table", v1_21.AddGitSizeAndLFSSizeToRepositoryTable),
		newMigration(264, "Add branch table", v1_21.AddBranchTable),
		newMigration(265, "Alter Actions Artifact table", v1_21.AlterActionArtifactTable),
		newMigration(266, "Reduce commit status", v1_21.ReduceCommitStatus),
		newMigration(267, "Add action_tasks_version table", v1_21.CreateActionTasksVersionTable),
		newMigration(268, "Update Action Ref", v1_21.UpdateActionsRefIndex),
		newMigration(269, "Drop deleted branch table", v1_21.DropDeletedBranchTable),
		newMigration(270, "Fix PackageProperty typo", v1_21.FixPackagePropertyTypo),
		newMigration(271, "Allow archiving labels", v1_21.AddArchivedUnixColumInLabelTable),
		newMigration(272, "Add Version to ActionRun table", v1_21.AddVersionToActionRunTable),
		newMigration(273, "Add Action Schedule Table", v1_21.AddActionScheduleTable),
		newMigration(274, "Add Actions artifacts expiration date", v1_21.AddExpiredUnixColumnInActionArtifactTable),
		newMigration(275, "Add ScheduleID for ActionRun", v1_21.AddScheduleIDForActionRun),
		newMigration(276, "Add RemoteAddress to mirrors", v1_21.AddRemoteAddressToMirrors),
		newMigration(277, "Add Index to issue_user.issue_id", v1_21.AddIndexToIssueUserIssueID),
		newMigration(278, "Add Index to comment.dependent_issue_id", v1_21.AddIndexToCommentDependentIssueID),
		newMigration(279, "Add Index to action.user_id", v1_21.AddIndexToActionUserID),

		// Gitea 1.21.0 ends at database version 280

		newMigration(280, "Rename user themes", v1_22.RenameUserThemes),
		newMigration(281, "Add auth_token table", v1_22.CreateAuthTokenTable),
		newMigration(282, "Add Index to pull_auto_merge.doer_id", v1_22.AddIndexToPullAutoMergeDoerID),
		newMigration(283, "Add combined Index to issue_user.uid and issue_id", v1_22.AddCombinedIndexToIssueUser),
		newMigration(284, "Add ignore stale approval column on branch table", v1_22.AddIgnoreStaleApprovalsColumnToProtectedBranchTable),
		newMigration(285, "Add PreviousDuration to ActionRun", v1_22.AddPreviousDurationToActionRun),
		newMigration(286, "Add support for SHA256 git repositories", v1_22.AdjustDBForSha256),
		newMigration(287, "Use Slug instead of ID for Badges", v1_22.UseSlugInsteadOfIDForBadges),
		newMigration(288, "Add user_blocking table", v1_22.AddUserBlockingTable),
		newMigration(289, "Add default_wiki_branch to repository table", v1_22.AddDefaultWikiBranch),
		newMigration(290, "Add PayloadVersion to HookTask", v1_22.AddPayloadVersionToHookTaskTable),
		newMigration(291, "Add Index to attachment.comment_id", v1_22.AddCommentIDIndexofAttachment),
		newMigration(292, "Ensure every project has exactly one default column - No Op", noopMigration),
		newMigration(293, "Ensure every project has exactly one default column", v1_22.CheckProjectColumnsConsistency),

		// Gitea 1.22.0-rc0 ends at database version 294

		newMigration(294, "Add unique index for project issue table", v1_22.AddUniqueIndexForProjectIssue),
		newMigration(295, "Add commit status summary table", v1_22.AddCommitStatusSummary),
		newMigration(296, "Add missing field of commit status summary table", v1_22.AddCommitStatusSummary2),
		newMigration(297, "Add everyone_access_mode for repo_unit", v1_22.AddRepoUnitEveryoneAccessMode),
		newMigration(298, "Drop wrongly created table o_auth2_application", v1_22.DropWronglyCreatedTable),

		// Gitea 1.22.0-rc1 ends at migration ID number 298 (database version 299)

		newMigration(299, "Add content version to issue and comment table", v1_23.AddContentVersionToIssueAndComment),
		newMigration(300, "Add force-push branch protection support", v1_23.AddForcePushBranchProtection),
		newMigration(301, "Add skip_secondary_authorization option to oauth2 application table", v1_23.AddSkipSecondaryAuthColumnToOAuth2ApplicationTable),
		newMigration(302, "Add index to action_task stopped log_expired", v1_23.AddIndexToActionTaskStoppedLogExpired),
		newMigration(303, "Add metadata column for comment table", v1_23.AddCommentMetaDataColumn),
		newMigration(304, "Add index for release sha1", v1_23.AddIndexForReleaseSha1),
		newMigration(305, "Add Repository Licenses", v1_23.AddRepositoryLicenses),
		newMigration(306, "Add BlockAdminMergeOverride to ProtectedBranch", v1_23.AddBlockAdminMergeOverrideBranchProtection),
		newMigration(307, "Fix milestone deadline_unix when there is no due date", v1_23.FixMilestoneNoDueDate),
		newMigration(308, "Add index(user_id, is_deleted) for action table", v1_23.AddNewIndexForUserDashboard),
		newMigration(309, "Improve Notification table indices", v1_23.ImproveNotificationTableIndices),
		newMigration(310, "Add Priority to ProtectedBranch", v1_23.AddPriorityToProtectedBranch),
		newMigration(311, "Add TimeEstimate to Issue table", v1_23.AddTimeEstimateColumnToIssueTable),

		// Gitea 1.23.0-rc0 ends at migration ID number 311 (database version 312)
		newMigration(312, "Add DeleteBranchAfterMerge to AutoMerge", v1_24.AddDeleteBranchAfterMergeForAutoMerge),
	}
	return preparedMigrations
}

// GetCurrentDBVersion returns the current db version
func GetCurrentDBVersion(x *xorm.Engine) (int64, error) {
	if err := x.Sync(new(Version)); err != nil {
		return -1, fmt.Errorf("sync: %w", err)
	}

	currentVersion := &Version{ID: 1}
	has, err := x.Get(currentVersion)
	if err != nil {
		return -1, fmt.Errorf("get: %w", err)
	}
	if !has {
		return -1, nil
	}
	return currentVersion.Version, nil
}

func calcDBVersion(migrations []*migration) int64 {
	dbVer := int64(minDBVersion + len(migrations))
	if migrations[0].idNumber != minDBVersion {
		panic("migrations should start at minDBVersion")
	}
	if dbVer != migrations[len(migrations)-1].idNumber+1 {
		panic("migrations are not in order")
	}
	return dbVer
}

// ExpectedDBVersion returns the expected db version
func ExpectedDBVersion() int64 {
	return calcDBVersion(prepareMigrationTasks())
}

// EnsureUpToDate will check if the db is at the correct version
func EnsureUpToDate(x *xorm.Engine) error {
	currentDB, err := GetCurrentDBVersion(x)
	if err != nil {
		return err
	}

	if currentDB < 0 {
		return fmt.Errorf("database has not been initialized")
	}

	if minDBVersion > currentDB {
		return fmt.Errorf("DB version %d (<= %d) is too old for auto-migration. Upgrade to Gitea 1.6.4 first then upgrade to this version", currentDB, minDBVersion)
	}

	expectedDB := ExpectedDBVersion()

	if currentDB != expectedDB {
		return fmt.Errorf(`current database version %d is not equal to the expected version %d. Please run "gitea [--config /path/to/app.ini] migrate" to update the database version`, currentDB, expectedDB)
	}

	return nil
}

func getPendingMigrations(curDBVer int64, migrations []*migration) []*migration {
	return migrations[curDBVer-minDBVersion:]
}

func migrationIDNumberToDBVersion(idNumber int64) int64 {
	return idNumber + 1
}

// Migrate database to current version
func Migrate(x *xorm.Engine) error {
	migrations := prepareMigrationTasks()
	maxDBVer := calcDBVersion(migrations)

	// Set a new clean the default mapper to GonicMapper as that is the default for Gitea.
	x.SetMapper(names.GonicMapper{})
	if err := x.Sync(new(Version)); err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	currentVersion := &Version{ID: 1}
	has, err := x.Get(currentVersion)
	if err != nil {
		return fmt.Errorf("get: %w", err)
	} else if !has {
		// If the version record does not exist, it is a fresh installation, and we can skip all migrations.
		// XORM model framework will create all tables when initializing.
		currentVersion.ID = 0
		currentVersion.Version = maxDBVer
		if _, err = x.InsertOne(currentVersion); err != nil {
			return fmt.Errorf("insert: %w", err)
		}
	}

	curDBVer := currentVersion.Version
	// Outdated Gitea database version is not supported
	if curDBVer < minDBVersion {
		log.Fatal(`Gitea no longer supports auto-migration from your previously installed version.
Please try upgrading to a lower version first (suggested v1.6.4), then upgrade to this version.`)
		return nil
	}

	// Downgrading Gitea's database version not supported
	if maxDBVer < curDBVer {
		msg := fmt.Sprintf("Your database (migration version: %d) is for a newer Gitea, you can not use the newer database for this old Gitea release (%d).", curDBVer, maxDBVer)
		msg += "\nGitea will exit to keep your database safe and unchanged. Please use the correct Gitea release, do not change the migration version manually (incorrect manual operation may lose data)."
		if !setting.IsProd {
			msg += fmt.Sprintf("\nIf you are in development and really know what you're doing, you can force changing the migration version by executing: UPDATE version SET version=%d WHERE id=1;", maxDBVer)
		}
		log.Fatal("Migration Error: %s", msg)
		return nil
	}

	// Some migration tasks depend on the git command
	if git.DefaultContext == nil {
		if err = git.InitSimple(context.Background()); err != nil {
			return err
		}
	}

	// Migrate
	for _, m := range getPendingMigrations(curDBVer, migrations) {
		log.Info("Migration[%d]: %s", m.idNumber, m.description)
		// Reset the mapper between each migration - migrations are not supposed to depend on each other
		x.SetMapper(names.GonicMapper{})
		if err = m.Migrate(x); err != nil {
			return fmt.Errorf("migration[%d]: %s failed: %w", m.idNumber, m.description, err)
		}
		currentVersion.Version = migrationIDNumberToDBVersion(m.idNumber)
		if _, err = x.ID(1).Update(currentVersion); err != nil {
			return err
		}
	}
	return nil
}
