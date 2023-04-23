// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"os"

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

// Use noopMigration when there is a migration that has been no-oped
var noopMigration = func(_ *xorm.Engine) error { return nil }

// This is a sequence of migrations. Add new migrations to the bottom of the list.
// If you want to "retire" a migration, remove it from the top of the list and
// update minDBVersion accordingly
var migrations = []Migration{
	// Gitea 1.5.0 ends at v69

	// v70 -> v71
	NewMigration("add issue_dependencies", v1_6.AddIssueDependencies),
	// v71 -> v72
	NewMigration("protect each scratch token", v1_6.AddScratchHash),
	// v72 -> v73
	NewMigration("add review", v1_6.AddReview),

	// Gitea 1.6.0 ends at v73

	// v73 -> v74
	NewMigration("add must_change_password column for users table", v1_7.AddMustChangePassword),
	// v74 -> v75
	NewMigration("add approval whitelists to protected branches", v1_7.AddApprovalWhitelistsToProtectedBranches),
	// v75 -> v76
	NewMigration("clear nonused data which not deleted when user was deleted", v1_7.ClearNonusedData),

	// Gitea 1.7.0 ends at v76

	// v76 -> v77
	NewMigration("add pull request rebase with merge commit", v1_8.AddPullRequestRebaseWithMerge),
	// v77 -> v78
	NewMigration("add theme to users", v1_8.AddUserDefaultTheme),
	// v78 -> v79
	NewMigration("rename repo is_bare to repo is_empty", v1_8.RenameRepoIsBareToIsEmpty),
	// v79 -> v80
	NewMigration("add can close issues via commit in any branch", v1_8.AddCanCloseIssuesViaCommitInAnyBranch),
	// v80 -> v81
	NewMigration("add is locked to issues", v1_8.AddIsLockedToIssues),
	// v81 -> v82
	NewMigration("update U2F counter type", v1_8.ChangeU2FCounterType),

	// Gitea 1.8.0 ends at v82

	// v82 -> v83
	NewMigration("hot fix for wrong release sha1 on release table", v1_9.FixReleaseSha1OnReleaseTable),
	// v83 -> v84
	NewMigration("add uploader id for table attachment", v1_9.AddUploaderIDForAttachment),
	// v84 -> v85
	NewMigration("add table to store original imported gpg keys", v1_9.AddGPGKeyImport),
	// v85 -> v86
	NewMigration("hash application token", v1_9.HashAppToken),
	// v86 -> v87
	NewMigration("add http method to webhook", v1_9.AddHTTPMethodToWebhook),
	// v87 -> v88
	NewMigration("add avatar field to repository", v1_9.AddAvatarFieldToRepository),

	// Gitea 1.9.0 ends at v88

	// v88 -> v89
	NewMigration("add commit status context field to commit_status", v1_10.AddCommitStatusContext),
	// v89 -> v90
	NewMigration("add original author/url migration info to issues, comments, and repo ", v1_10.AddOriginalMigrationInfo),
	// v90 -> v91
	NewMigration("change length of some repository columns", v1_10.ChangeSomeColumnsLengthOfRepo),
	// v91 -> v92
	NewMigration("add index on owner_id of repository and type, review_id of comment", v1_10.AddIndexOnRepositoryAndComment),
	// v92 -> v93
	NewMigration("remove orphaned repository index statuses", v1_10.RemoveLingeringIndexStatus),
	// v93 -> v94
	NewMigration("add email notification enabled preference to user", v1_10.AddEmailNotificationEnabledToUser),
	// v94 -> v95
	NewMigration("add enable_status_check, status_check_contexts to protected_branch", v1_10.AddStatusCheckColumnsForProtectedBranches),
	// v95 -> v96
	NewMigration("add table columns for cross referencing issues", v1_10.AddCrossReferenceColumns),
	// v96 -> v97
	NewMigration("delete orphaned attachments", v1_10.DeleteOrphanedAttachments),
	// v97 -> v98
	NewMigration("add repo_admin_change_team_access to user", v1_10.AddRepoAdminChangeTeamAccessColumnForUser),
	// v98 -> v99
	NewMigration("add original author name and id on migrated release", v1_10.AddOriginalAuthorOnMigratedReleases),
	// v99 -> v100
	NewMigration("add task table and status column for repository table", v1_10.AddTaskTable),
	// v100 -> v101
	NewMigration("update migration repositories' service type", v1_10.UpdateMigrationServiceTypes),
	// v101 -> v102
	NewMigration("change length of some external login users columns", v1_10.ChangeSomeColumnsLengthOfExternalLoginUser),

	// Gitea 1.10.0 ends at v102

	// v102 -> v103
	NewMigration("update migration repositories' service type", v1_11.DropColumnHeadUserNameOnPullRequest),
	// v103 -> v104
	NewMigration("Add WhitelistDeployKeys to protected branch", v1_11.AddWhitelistDeployKeysToBranches),
	// v104 -> v105
	NewMigration("remove unnecessary columns from label", v1_11.RemoveLabelUneededCols),
	// v105 -> v106
	NewMigration("add includes_all_repositories to teams", v1_11.AddTeamIncludesAllRepositories),
	// v106 -> v107
	NewMigration("add column `mode` to table watch", v1_11.AddModeColumnToWatch),
	// v107 -> v108
	NewMigration("Add template options to repository", v1_11.AddTemplateToRepo),
	// v108 -> v109
	NewMigration("Add comment_id on table notification", v1_11.AddCommentIDOnNotification),
	// v109 -> v110
	NewMigration("add can_create_org_repo to team", v1_11.AddCanCreateOrgRepoColumnForTeam),
	// v110 -> v111
	NewMigration("change review content type to text", v1_11.ChangeReviewContentToText),
	// v111 -> v112
	NewMigration("update branch protection for can push and whitelist enable", v1_11.AddBranchProtectionCanPushAndEnableWhitelist),
	// v112 -> v113
	NewMigration("remove release attachments which repository deleted", v1_11.RemoveAttachmentMissedRepo),
	// v113 -> v114
	NewMigration("new feature: change target branch of pull requests", v1_11.FeatureChangeTargetBranch),
	// v114 -> v115
	NewMigration("Remove authentication credentials from stored URL", v1_11.SanitizeOriginalURL),
	// v115 -> v116
	NewMigration("add user_id prefix to existing user avatar name", v1_11.RenameExistingUserAvatarName),
	// v116 -> v117
	NewMigration("Extend TrackedTimes", v1_11.ExtendTrackedTimes),

	// Gitea 1.11.0 ends at v117

	// v117 -> v118
	NewMigration("Add block on rejected reviews branch protection", v1_12.AddBlockOnRejectedReviews),
	// v118 -> v119
	NewMigration("Add commit id and stale to reviews", v1_12.AddReviewCommitAndStale),
	// v119 -> v120
	NewMigration("Fix migrated repositories' git service type", v1_12.FixMigratedRepositoryServiceType),
	// v120 -> v121
	NewMigration("Add owner_name on table repository", v1_12.AddOwnerNameOnRepository),
	// v121 -> v122
	NewMigration("add is_restricted column for users table", v1_12.AddIsRestricted),
	// v122 -> v123
	NewMigration("Add Require Signed Commits to ProtectedBranch", v1_12.AddRequireSignedCommits),
	// v123 -> v124
	NewMigration("Add original information for reactions", v1_12.AddReactionOriginals),
	// v124 -> v125
	NewMigration("Add columns to user and repository", v1_12.AddUserRepoMissingColumns),
	// v125 -> v126
	NewMigration("Add some columns on review for migration", v1_12.AddReviewMigrateInfo),
	// v126 -> v127
	NewMigration("Fix topic repository count", v1_12.FixTopicRepositoryCount),
	// v127 -> v128
	NewMigration("add repository code language statistics", v1_12.AddLanguageStats),
	// v128 -> v129
	NewMigration("fix merge base for pull requests", v1_12.FixMergeBase),
	// v129 -> v130
	NewMigration("remove dependencies from deleted repositories", v1_12.PurgeUnusedDependencies),
	// v130 -> v131
	NewMigration("Expand webhooks for more granularity", v1_12.ExpandWebhooks),
	// v131 -> v132
	NewMigration("Add IsSystemWebhook column to webhooks table", v1_12.AddSystemWebhookColumn),
	// v132 -> v133
	NewMigration("Add Branch Protection Protected Files Column", v1_12.AddBranchProtectionProtectedFilesColumn),
	// v133 -> v134
	NewMigration("Add EmailHash Table", v1_12.AddEmailHashTable),
	// v134 -> v135
	NewMigration("Refix merge base for merged pull requests", v1_12.RefixMergeBase),
	// v135 -> v136
	NewMigration("Add OrgID column to Labels table", v1_12.AddOrgIDLabelColumn),
	// v136 -> v137
	NewMigration("Add CommitsAhead and CommitsBehind Column to PullRequest Table", v1_12.AddCommitDivergenceToPulls),
	// v137 -> v138
	NewMigration("Add Branch Protection Block Outdated Branch", v1_12.AddBlockOnOutdatedBranch),
	// v138 -> v139
	NewMigration("Add ResolveDoerID to Comment table", v1_12.AddResolveDoerIDCommentColumn),
	// v139 -> v140
	NewMigration("prepend refs/heads/ to issue refs", v1_12.PrependRefsHeadsToIssueRefs),

	// Gitea 1.12.0 ends at v140

	// v140 -> v141
	NewMigration("Save detected language file size to database instead of percent", v1_13.FixLanguageStatsToSaveSize),
	// v141 -> v142
	NewMigration("Add KeepActivityPrivate to User table", v1_13.AddKeepActivityPrivateUserColumn),
	// v142 -> v143
	NewMigration("Ensure Repository.IsArchived is not null", v1_13.SetIsArchivedToFalse),
	// v143 -> v144
	NewMigration("recalculate Stars number for all user", v1_13.RecalculateStars),
	// v144 -> v145
	NewMigration("update Matrix Webhook http method to 'PUT'", v1_13.UpdateMatrixWebhookHTTPMethod),
	// v145 -> v146
	NewMigration("Increase Language field to 50 in LanguageStats", v1_13.IncreaseLanguageField),
	// v146 -> v147
	NewMigration("Add projects info to repository table", v1_13.AddProjectsInfo),
	// v147 -> v148
	NewMigration("create review for 0 review id code comments", v1_13.CreateReviewsForCodeComments),
	// v148 -> v149
	NewMigration("remove issue dependency comments who refer to non existing issues", v1_13.PurgeInvalidDependenciesComments),
	// v149 -> v150
	NewMigration("Add Created and Updated to Milestone table", v1_13.AddCreatedAndUpdatedToMilestones),
	// v150 -> v151
	NewMigration("add primary key to repo_topic", v1_13.AddPrimaryKeyToRepoTopic),
	// v151 -> v152
	NewMigration("set default password algorithm to Argon2", v1_13.SetDefaultPasswordToArgon2),
	// v152 -> v153
	NewMigration("add TrustModel field to Repository", v1_13.AddTrustModelToRepository),
	// v153 > v154
	NewMigration("add Team review request support", v1_13.AddTeamReviewRequestSupport),
	// v154 > v155
	NewMigration("add timestamps to Star, Label, Follow, Watch and Collaboration", v1_13.AddTimeStamps),

	// Gitea 1.13.0 ends at v155

	// v155 -> v156
	NewMigration("add changed_protected_files column for pull_request table", v1_14.AddChangedProtectedFilesPullRequestColumn),
	// v156 -> v157
	NewMigration("fix publisher ID for tag releases", v1_14.FixPublisherIDforTagReleases),
	// v157 -> v158
	NewMigration("ensure repo topics are up-to-date", v1_14.FixRepoTopics),
	// v158 -> v159
	NewMigration("code comment replies should have the commitID of the review they are replying to", v1_14.UpdateCodeCommentReplies),
	// v159 -> v160
	NewMigration("update reactions constraint", v1_14.UpdateReactionConstraint),
	// v160 -> v161
	NewMigration("Add block on official review requests branch protection", v1_14.AddBlockOnOfficialReviewRequests),
	// v161 -> v162
	NewMigration("Convert task type from int to string", v1_14.ConvertTaskTypeToString),
	// v162 -> v163
	NewMigration("Convert webhook task type from int to string", v1_14.ConvertWebhookTaskTypeToString),
	// v163 -> v164
	NewMigration("Convert topic name from 25 to 50", v1_14.ConvertTopicNameFrom25To50),
	// v164 -> v165
	NewMigration("Add scope and nonce columns to oauth2_grant table", v1_14.AddScopeAndNonceColumnsToOAuth2Grant),
	// v165 -> v166
	NewMigration("Convert hook task type from char(16) to varchar(16) and trim the column", v1_14.ConvertHookTaskTypeToVarcharAndTrim),
	// v166 -> v167
	NewMigration("Where Password is Valid with Empty String delete it", v1_14.RecalculateUserEmptyPWD),
	// v167 -> v168
	NewMigration("Add user redirect", v1_14.AddUserRedirect),
	// v168 -> v169
	NewMigration("Recreate user table to fix default values", v1_14.RecreateUserTableToFixDefaultValues),
	// v169 -> v170
	NewMigration("Update DeleteBranch comments to set the old_ref to the commit_sha", v1_14.CommentTypeDeleteBranchUseOldRef),
	// v170 -> v171
	NewMigration("Add Dismissed to Review table", v1_14.AddDismissedReviewColumn),
	// v171 -> v172
	NewMigration("Add Sorting to ProjectBoard table", v1_14.AddSortingColToProjectBoard),
	// v172 -> v173
	NewMigration("Add sessions table for go-chi/session", v1_14.AddSessionTable),
	// v173 -> v174
	NewMigration("Add time_id column to Comment", v1_14.AddTimeIDCommentColumn),
	// v174 -> v175
	NewMigration("Create repo transfer table", v1_14.AddRepoTransfer),
	// v175 -> v176
	NewMigration("Fix Postgres ID Sequences broken by recreate-table", v1_14.FixPostgresIDSequences),
	// v176 -> v177
	NewMigration("Remove invalid labels from comments", v1_14.RemoveInvalidLabels),
	// v177 -> v178
	NewMigration("Delete orphaned IssueLabels", v1_14.DeleteOrphanedIssueLabels),

	// Gitea 1.14.0 ends at v178

	// v178 -> v179
	NewMigration("Add LFS columns to Mirror", v1_15.AddLFSMirrorColumns),
	// v179 -> v180
	NewMigration("Convert avatar url to text", v1_15.ConvertAvatarURLToText),
	// v180 -> v181
	NewMigration("Delete credentials from past migrations", v1_15.DeleteMigrationCredentials),
	// v181 -> v182
	NewMigration("Always save primary email on email address table", v1_15.AddPrimaryEmail2EmailAddress),
	// v182 -> v183
	NewMigration("Add issue resource index table", v1_15.AddIssueResourceIndexTable),
	// v183 -> v184
	NewMigration("Create PushMirror table", v1_15.CreatePushMirrorTable),
	// v184 -> v185
	NewMigration("Rename Task errors to message", v1_15.RenameTaskErrorsToMessage),
	// v185 -> v186
	NewMigration("Add new table repo_archiver", v1_15.AddRepoArchiver),
	// v186 -> v187
	NewMigration("Create protected tag table", v1_15.CreateProtectedTagTable),
	// v187 -> v188
	NewMigration("Drop unneeded webhook related columns", v1_15.DropWebhookColumns),
	// v188 -> v189
	NewMigration("Add key is verified to gpg key", v1_15.AddKeyIsVerified),

	// Gitea 1.15.0 ends at v189

	// v189 -> v190
	NewMigration("Unwrap ldap.Sources", v1_16.UnwrapLDAPSourceCfg),
	// v190 -> v191
	NewMigration("Add agit flow pull request support", v1_16.AddAgitFlowPullRequest),
	// v191 -> v192
	NewMigration("Alter issue/comment table TEXT fields to LONGTEXT", v1_16.AlterIssueAndCommentTextFieldsToLongText),
	// v192 -> v193
	NewMigration("RecreateIssueResourceIndexTable to have a primary key instead of an unique index", v1_16.RecreateIssueResourceIndexTable),
	// v193 -> v194
	NewMigration("Add repo id column for attachment table", v1_16.AddRepoIDForAttachment),
	// v194 -> v195
	NewMigration("Add Branch Protection Unprotected Files Column", v1_16.AddBranchProtectionUnprotectedFilesColumn),
	// v195 -> v196
	NewMigration("Add table commit_status_index", v1_16.AddTableCommitStatusIndex),
	// v196 -> v197
	NewMigration("Add Color to ProjectBoard table", v1_16.AddColorColToProjectBoard),
	// v197 -> v198
	NewMigration("Add renamed_branch table", v1_16.AddRenamedBranchTable),
	// v198 -> v199
	NewMigration("Add issue content history table", v1_16.AddTableIssueContentHistory),
	// v199 -> v200
	NewMigration("No-op (remote version is using AppState now)", noopMigration),
	// v200 -> v201
	NewMigration("Add table app_state", v1_16.AddTableAppState),
	// v201 -> v202
	NewMigration("Drop table remote_version (if exists)", v1_16.DropTableRemoteVersion),
	// v202 -> v203
	NewMigration("Create key/value table for user settings", v1_16.CreateUserSettingsTable),
	// v203 -> v204
	NewMigration("Add Sorting to ProjectIssue table", v1_16.AddProjectIssueSorting),
	// v204 -> v205
	NewMigration("Add key is verified to ssh key", v1_16.AddSSHKeyIsVerified),
	// v205 -> v206
	NewMigration("Migrate to higher varchar on user struct", v1_16.MigrateUserPasswordSalt),
	// v206 -> v207
	NewMigration("Add authorize column to team_unit table", v1_16.AddAuthorizeColForTeamUnit),
	// v207 -> v208
	NewMigration("Add webauthn table and migrate u2f data to webauthn - NO-OPED", v1_16.AddWebAuthnCred),
	// v208 -> v209
	NewMigration("Use base32.HexEncoding instead of base64 encoding for cred ID as it is case insensitive - NO-OPED", v1_16.UseBase32HexForCredIDInWebAuthnCredential),
	// v209 -> v210
	NewMigration("Increase WebAuthentication CredentialID size to 410 - NO-OPED", v1_16.IncreaseCredentialIDTo410),
	// v210 -> v211
	NewMigration("v208 was completely broken - remigrate", v1_16.RemigrateU2FCredentials),

	// Gitea 1.16.2 ends at v211

	// v211 -> v212
	NewMigration("Create ForeignReference table", v1_17.CreateForeignReferenceTable),
	// v212 -> v213
	NewMigration("Add package tables", v1_17.AddPackageTables),
	// v213 -> v214
	NewMigration("Add allow edits from maintainers to PullRequest table", v1_17.AddAllowMaintainerEdit),
	// v214 -> v215
	NewMigration("Add auto merge table", v1_17.AddAutoMergeTable),
	// v215 -> v216
	NewMigration("allow to view files in PRs", v1_17.AddReviewViewedFiles),
	// v216 -> v217
	NewMigration("No-op (Improve Action table indices v1)", noopMigration),
	// v217 -> v218
	NewMigration("Alter hook_task table TEXT fields to LONGTEXT", v1_17.AlterHookTaskTextFieldsToLongText),
	// v218 -> v219
	NewMigration("Improve Action table indices v2", v1_17.ImproveActionTableIndices),
	// v219 -> v220
	NewMigration("Add sync_on_commit column to push_mirror table", v1_17.AddSyncOnCommitColForPushMirror),
	// v220 -> v221
	NewMigration("Add container repository property", v1_17.AddContainerRepositoryProperty),
	// v221 -> v222
	NewMigration("Store WebAuthentication CredentialID as bytes and increase size to at least 1024", v1_17.StoreWebauthnCredentialIDAsBytes),
	// v222 -> v223
	NewMigration("Drop old CredentialID column", v1_17.DropOldCredentialIDColumn),
	// v223 -> v224
	NewMigration("Rename CredentialIDBytes column to CredentialID", v1_17.RenameCredentialIDBytes),

	// Gitea 1.17.0 ends at v224

	// v224 -> v225
	NewMigration("Add badges to users", v1_18.CreateUserBadgesTable),
	// v225 -> v226
	NewMigration("Alter gpg_key/public_key content TEXT fields to MEDIUMTEXT", v1_18.AlterPublicGPGKeyContentFieldsToMediumText),
	// v226 -> v227
	NewMigration("Conan and generic packages do not need to be semantically versioned", v1_18.FixPackageSemverField),
	// v227 -> v228
	NewMigration("Create key/value table for system settings", v1_18.CreateSystemSettingsTable),
	// v228 -> v229
	NewMigration("Add TeamInvite table", v1_18.AddTeamInviteTable),
	// v229 -> v230
	NewMigration("Update counts of all open milestones", v1_18.UpdateOpenMilestoneCounts),
	// v230 -> v231
	NewMigration("Add ConfidentialClient column (default true) to OAuth2Application table", v1_18.AddConfidentialClientColumnToOAuth2ApplicationTable),

	// Gitea 1.18.0 ends at v231

	// v231 -> v232
	NewMigration("Add index for hook_task", v1_19.AddIndexForHookTask),
	// v232 -> v233
	NewMigration("Alter package_version.metadata_json to LONGTEXT", v1_19.AlterPackageVersionMetadataToLongText),
	// v233 -> v234
	NewMigration("Add header_authorization_encrypted column to webhook table", v1_19.AddHeaderAuthorizationEncryptedColWebhook),
	// v234 -> v235
	NewMigration("Add package cleanup rule table", v1_19.CreatePackageCleanupRuleTable),
	// v235 -> v236
	NewMigration("Add index for access_token", v1_19.AddIndexForAccessToken),
	// v236 -> v237
	NewMigration("Create secrets table", v1_19.CreateSecretsTable),
	// v237 -> v238
	NewMigration("Drop ForeignReference table", v1_19.DropForeignReferenceTable),
	// v238 -> v239
	NewMigration("Add updated unix to LFSMetaObject", v1_19.AddUpdatedUnixToLFSMetaObject),
	// v239 -> v240
	NewMigration("Add scope for access_token", v1_19.AddScopeForAccessTokens),
	// v240 -> v241
	NewMigration("Add actions tables", v1_19.AddActionsTables),
	// v241 -> v242
	NewMigration("Add card_type column to project table", v1_19.AddCardTypeToProjectTable),
	// v242 -> v243
	NewMigration("Alter gpg_key_import content TEXT field to MEDIUMTEXT", v1_19.AlterPublicGPGKeyImportContentFieldToMediumText),
	// v243 -> v244
	NewMigration("Add exclusive label", v1_19.AddExclusiveLabel),

	// Gitea 1.19.0 ends at v244

	// v244 -> v245
	NewMigration("Add NeedApproval to actions tables", v1_20.AddNeedApprovalToActionRun),
	// v245 -> v246
	NewMigration("Rename Webhook org_id to owner_id", v1_20.RenameWebhookOrgToOwner),
	// v246 -> v247
	NewMigration("Add missed column owner_id for project table", v1_20.AddNewColumnForProject),
	// v247 -> v248
	NewMigration("Fix incorrect project type", v1_20.FixIncorrectProjectType),
	// v248 -> v249
	NewMigration("Add version column to action_runner table", v1_20.AddVersionToActionRunner),
	// v249 -> v250
	NewMigration("Improve Action table indices v3", v1_20.ImproveActionTableIndices),
	// v250 -> v251
	NewMigration("Change Container Metadata", v1_20.ChangeContainerMetadataMultiArch),
	// v251 -> v252
	NewMigration("Fix incorrect owner team unit access mode", v1_20.FixIncorrectOwnerTeamUnitAccessMode),
	// v252 -> v253
	NewMigration("Fix incorrect admin team unit access mode", v1_20.FixIncorrectAdminTeamUnitAccessMode),
	// v253 -> v254
	NewMigration("Fix ExternalTracker and ExternalWiki accessMode in owner and admin team", v1_20.FixExternalTrackerAndExternalWikiAccessModeInOwnerAndAdminTeam),
	// v254 -> v255
	NewMigration("Add ActionTaskOutput table", v1_20.AddActionTaskOutputTable),
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
		return fmt.Errorf("Database has not been initialized")
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
		// If the version record does not exist we think
		// it is a fresh installation and we can skip all migrations.
		currentVersion.ID = 0
		currentVersion.Version = int64(minDBVersion + len(migrations))

		if _, err = x.InsertOne(currentVersion); err != nil {
			return fmt.Errorf("insert: %w", err)
		}
	}

	v := currentVersion.Version
	if minDBVersion > v {
		log.Fatal(`Gitea no longer supports auto-migration from your previously installed version.
Please try upgrading to a lower version first (suggested v1.6.4), then upgrade to this version.`)
		return nil
	}

	// Downgrading Gitea's database version not supported
	if int(v-minDBVersion) > len(migrations) {
		msg := fmt.Sprintf("Your database (migration version: %d) is for a newer Gitea, you can not use the newer database for this old Gitea release (%d).", v, minDBVersion+len(migrations))
		msg += "\nGitea will exit to keep your database safe and unchanged. Please use the correct Gitea release, do not change the migration version manually (incorrect manual operation may lose data)."
		if !setting.IsProd {
			msg += fmt.Sprintf("\nIf you are in development and really know what you're doing, you can force changing the migration version by executing: UPDATE version SET version=%d WHERE id=1;", minDBVersion+len(migrations))
		}
		_, _ = fmt.Fprintln(os.Stderr, msg)
		log.Fatal(msg)
		return nil
	}

	// Some migration tasks depend on the git command
	if git.DefaultContext == nil {
		if err = git.InitSimple(context.Background()); err != nil {
			return err
		}
	}

	// Migrate
	for i, m := range migrations[v-minDBVersion:] {
		log.Info("Migration[%d]: %s", v+int64(i), m.Description())
		// Reset the mapper between each migration - migrations are not supposed to depend on each other
		x.SetMapper(names.GonicMapper{})
		if err = m.Migrate(x); err != nil {
			return fmt.Errorf("migration[%d]: %s failed: %w", v+int64(i), m.Description(), err)
		}
		currentVersion.Version = v + int64(i) + 1
		if _, err = x.ID(1).Update(currentVersion); err != nil {
			return err
		}
	}
	return nil
}
