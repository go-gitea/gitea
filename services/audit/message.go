// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"fmt"
	"strings"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/modules/log"
)

// Reserved placeholders, filled from the event itself rather than from metadata.
const (
	placeholderScope = "scope"
	placeholderActor = "actor"
)

// messages maps every action to the template used to render its message. The
// message is derived instead of written at the call site so it can never drift
// from the action, and so a new action only needs one entry here.
//
// {placeholders} are filled from the event metadata, except for the reserved
// {scope} and {actor}, which come from the event's entity references. The
// rendered text is persisted and included in JSONL exports, so treat it as a
// stable interface for log parsers.
var messages = map[audit_model.Action]string{
	audit_model.UserImpersonation:               "User {actor} impersonating user {scope}.",
	audit_model.UserCreate:                      "Created user {scope}.",
	audit_model.UserDelete:                      "Deleted user {scope}.",
	audit_model.UserAuthenticationFailTwoFactor: "Failed two-factor authentication for user {scope}.",
	audit_model.UserAuthenticationSource:        "Changed authentication source of user {scope}.",
	audit_model.UserActive:                      "Changed activation status of user {scope}.",
	audit_model.UserRestricted:                  "Changed restricted status of user {scope}.",
	audit_model.UserAdmin:                       "Changed admin status of user {scope}.",
	audit_model.UserName:                        "Changed user name to {scope}.",
	audit_model.UserPassword:                    "Changed password of user {scope}.",
	audit_model.UserPasswordResetRequest:        "Requested password reset for user {scope}.",
	audit_model.UserVisibility:                  "Changed visibility of user {scope}.",
	audit_model.UserEmailPrimaryChange:          "Changed primary email of user {scope} to {email}.",
	audit_model.UserEmailAdd:                    "Added email {email} to user {scope}.",
	audit_model.UserEmailActivate:               "Changed activation status of email {email} of user {scope}.",
	audit_model.UserEmailRemove:                 "Removed email {email} from user {scope}.",
	audit_model.UserTwoFactorEnable:             "Enabled two-factor authentication for user {scope}.",
	audit_model.UserTwoFactorRegenerate:         "Regenerated two-factor authentication secret for user {scope}.",
	audit_model.UserTwoFactorDisable:            "Disabled two-factor authentication for user {scope}.",
	audit_model.UserWebAuthAdd:                  "Added WebAuthn key {credential} for user {scope}.",
	audit_model.UserWebAuthRemove:               "Removed WebAuthn key {credential} from user {scope}.",
	audit_model.UserExternalLoginAdd:            "Added external login {external_id} for user {scope} using provider {provider}.",
	audit_model.UserExternalLoginRemove:         "Removed external login from authentication source {auth_source_id} for user {scope}.",
	audit_model.UserOpenIDAdd:                   "Associated OpenID {openid} to user {scope}.",
	audit_model.UserOpenIDRemove:                "Removed OpenID {openid} from user {scope}.",
	audit_model.UserAccessTokenAdd:              "Added access token {token} for user {scope} with scope {token_scope}.",
	audit_model.UserAccessTokenRemove:           "Removed access token {token} from user {scope}.",
	audit_model.UserOAuth2ApplicationAdd:        "Added OAuth2 application {oauth2_application} for user {scope}.",
	audit_model.UserOAuth2ApplicationUpdate:     "Updated OAuth2 application {oauth2_application} of user {scope}.",
	audit_model.UserOAuth2ApplicationSecret:     "Regenerated secret for OAuth2 application {oauth2_application} of user {scope}.",
	audit_model.UserOAuth2ApplicationGrant:      "Granted OAuth2 application {oauth2_application} access to user {scope}.",
	audit_model.UserOAuth2ApplicationRevoke:     "Revoked OAuth2 grant for application {oauth2_application} of user {scope}.",
	audit_model.UserOAuth2ApplicationRemove:     "Removed OAuth2 application {oauth2_application} of user {scope}.",
	audit_model.UserKeySSHAdd:                   "Added SSH key {fingerprint} for user {scope}.",
	audit_model.UserKeySSHRemove:                "Removed SSH key {fingerprint} of user {scope}.",
	audit_model.UserKeyPrincipalAdd:             "Added principal key {key} for user {scope}.",
	audit_model.UserKeyPrincipalRemove:          "Removed principal key {key} of user {scope}.",
	audit_model.UserKeyGPGAdd:                   "Added GPG key {gpg_key_id} for user {scope}.",
	audit_model.UserKeyGPGRemove:                "Removed GPG key {gpg_key_id} of user {scope}.",
	audit_model.UserSecretAdd:                   "Added secret {secret} to user {scope}.",
	audit_model.UserSecretUpdate:                "Updated secret {secret} of user {scope}.",
	audit_model.UserSecretRemove:                "Removed secret {secret} from user {scope}.",
	audit_model.UserWebhookAdd:                  "Added webhook {webhook} to user {scope}.",
	audit_model.UserWebhookUpdate:               "Updated webhook {webhook} of user {scope}.",
	audit_model.UserWebhookRemove:               "Removed webhook {webhook} of user {scope}.",

	audit_model.OrganizationCreate:                  "Created organization {scope}.",
	audit_model.OrganizationDelete:                  "Deleted organization {scope}.",
	audit_model.OrganizationName:                    "Changed organization name to {scope}.",
	audit_model.OrganizationVisibility:              "Changed visibility of organization {scope} to {new_visibility}.",
	audit_model.OrganizationTeamAdd:                 "Added team {team} to organization {scope}.",
	audit_model.OrganizationTeamUpdate:              "Updated settings of team {scope}/{team}.",
	audit_model.OrganizationTeamRemove:              "Removed team {team} from organization {scope}.",
	audit_model.OrganizationTeamPermission:          "Changed permission of team {scope}/{team} to {permission}.",
	audit_model.OrganizationTeamMemberAdd:           "Added user {member} to team {scope}/{team}.",
	audit_model.OrganizationTeamMemberRemove:        "Removed user {member} from team {scope}/{team}.",
	audit_model.OrganizationOAuth2ApplicationAdd:    "Added OAuth2 application {oauth2_application} for organization {scope}.",
	audit_model.OrganizationOAuth2ApplicationUpdate: "Updated OAuth2 application {oauth2_application} of organization {scope}.",
	audit_model.OrganizationOAuth2ApplicationSecret: "Regenerated secret for OAuth2 application {oauth2_application} of organization {scope}.",
	audit_model.OrganizationOAuth2ApplicationRemove: "Removed OAuth2 application {oauth2_application} of organization {scope}.",
	audit_model.OrganizationSecretAdd:               "Added secret {secret} to organization {scope}.",
	audit_model.OrganizationSecretUpdate:            "Updated secret {secret} of organization {scope}.",
	audit_model.OrganizationSecretRemove:            "Removed secret {secret} from organization {scope}.",
	audit_model.OrganizationWebhookAdd:              "Added webhook {webhook} to organization {scope}.",
	audit_model.OrganizationWebhookUpdate:           "Updated webhook {webhook} of organization {scope}.",
	audit_model.OrganizationWebhookRemove:           "Removed webhook {webhook} of organization {scope}.",

	audit_model.RepositoryCreate:                 "Created repository {scope}.",
	audit_model.RepositoryCreateFork:             "Created fork {scope} of repository {base_repo}.",
	audit_model.RepositoryArchive:                "Archived repository {scope}.",
	audit_model.RepositoryUnarchive:              "Unarchived repository {scope}.",
	audit_model.RepositoryDelete:                 "Deleted repository {scope}.",
	audit_model.RepositoryName:                   "Changed repository name from {previous_name} to {scope}.",
	audit_model.RepositoryVisibility:             "Changed visibility of repository {scope}.",
	audit_model.RepositoryConvertFork:            "Converted repository {scope} from fork to regular repository.",
	audit_model.RepositoryConvertMirror:          "Converted repository {scope} from pull mirror to regular repository.",
	audit_model.RepositoryMirrorPushAdd:          "Added push mirror to {remote_address} for repository {scope}.",
	audit_model.RepositoryMirrorPushRemove:       "Removed push mirror to {remote_address} for repository {scope}.",
	audit_model.RepositorySigningVerification:    "Changed signing verification of repository {scope} to {trust_model}.",
	audit_model.RepositoryTransferStart:          "Started repository transfer of {scope} to {new_owner}.",
	audit_model.RepositoryTransferFinish:         "Transferred repository {scope} from {old_owner} to {new_owner}.",
	audit_model.RepositoryTransferCancel:         "Canceled transfer of repository {scope}.",
	audit_model.RepositoryWikiDelete:             "Deleted wiki of repository {scope}.",
	audit_model.RepositoryCollaboratorAdd:        "Added user {collaborator} as collaborator for repository {scope} with access mode {access_mode}.",
	audit_model.RepositoryCollaboratorAccess:     "Changed access mode of collaborator {collaborator} of repository {scope} to {access_mode}.",
	audit_model.RepositoryCollaboratorRemove:     "Removed collaborator {collaborator} from repository {scope}.",
	audit_model.RepositoryCollaboratorTeamAdd:    "Added team {team} as collaborator for repository {scope}.",
	audit_model.RepositoryCollaboratorTeamRemove: "Removed team {team} as collaborator from repository {scope}.",
	audit_model.RepositoryBranchDefault:          "Changed default branch of repository {scope} to {default_branch}.",
	audit_model.RepositoryBranchProtectionAdd:    "Added branch protection {rule} for repository {scope}.",
	audit_model.RepositoryBranchProtectionUpdate: "Updated branch protection {rule} for repository {scope}.",
	audit_model.RepositoryBranchProtectionRemove: "Removed branch protection {rule} from repository {scope}.",
	audit_model.RepositoryTagProtectionAdd:       "Added tag protection {pattern} for repository {scope}.",
	audit_model.RepositoryTagProtectionUpdate:    "Updated tag protection {pattern} for repository {scope}.",
	audit_model.RepositoryTagProtectionRemove:    "Removed tag protection {pattern} from repository {scope}.",
	audit_model.RepositoryWebhookAdd:             "Added webhook {webhook} to repository {scope}.",
	audit_model.RepositoryWebhookUpdate:          "Updated webhook {webhook} of repository {scope}.",
	audit_model.RepositoryWebhookRemove:          "Removed webhook {webhook} of repository {scope}.",
	audit_model.RepositoryDeployKeyAdd:           "Added deploy key {deploy_key} for repository {scope}.",
	audit_model.RepositoryDeployKeyRemove:        "Removed deploy key {deploy_key} from repository {scope}.",
	audit_model.RepositorySecretAdd:              "Added secret {secret} to repository {scope}.",
	audit_model.RepositorySecretUpdate:           "Updated secret {secret} of repository {scope}.",
	audit_model.RepositorySecretRemove:           "Removed secret {secret} from repository {scope}.",

	audit_model.IssueCreate:        "Created issue {issue} in repository {scope}.",
	audit_model.IssueDelete:        "Deleted issue {issue} from repository {scope}.",
	audit_model.IssueCommentCreate: "Added comment {comment_id} to issue {issue} in repository {scope}.",
	audit_model.IssueCommentDelete: "Deleted comment {comment_id} from issue {issue} in repository {scope}.",

	audit_model.PullRequestCreate:        "Created pull request {pull_request} in repository {scope}.",
	audit_model.PullRequestDelete:        "Deleted pull request {pull_request} from repository {scope}.",
	audit_model.PullRequestMerge:         "Merged pull request {pull_request} in repository {scope}.",
	audit_model.PullRequestCommentCreate: "Added comment {comment_id} to pull request {pull_request} in repository {scope}.",
	audit_model.PullRequestCommentDelete: "Deleted comment {comment_id} from pull request {pull_request} in repository {scope}.",

	audit_model.ProjectCreate: "Created project {project} in {scope}.",
	audit_model.ProjectUpdate: "Updated project {project} in {scope}.",
	audit_model.ProjectDelete: "Deleted project {project} from {scope}.",

	audit_model.WikiPageCreate: "Created wiki page {page} in repository {scope}.",
	audit_model.WikiPageUpdate: "Updated wiki page {page} in repository {scope}.",
	audit_model.WikiPageDelete: "Deleted wiki page {page} from repository {scope}.",

	audit_model.ActionsWorkflowEnable:   "Enabled Actions workflow {workflow} in repository {scope}.",
	audit_model.ActionsWorkflowDisable:  "Disabled Actions workflow {workflow} in repository {scope}.",
	audit_model.ActionsWorkflowDispatch: "Dispatched Actions workflow {workflow} on {ref} in repository {scope}.",

	// Do not change the startup message anymore. We guarantee the stability of this message for
	// users wanting to parse the log themselves to be able to trace back events across gitea versions.
	audit_model.SystemStartup:                    "System started [Gitea {version}]",
	audit_model.SystemShutdown:                   "System shutdown",
	audit_model.SystemWebhookAdd:                 "Added instance-wide webhook {webhook}.",
	audit_model.SystemWebhookUpdate:              "Updated instance-wide webhook {webhook}.",
	audit_model.SystemWebhookRemove:              "Removed instance-wide webhook {webhook}.",
	audit_model.SystemAuthenticationSourceAdd:    "Created authentication source {auth_source}.",
	audit_model.SystemAuthenticationSourceUpdate: "Updated authentication source {auth_source}.",
	audit_model.SystemAuthenticationSourceRemove: "Removed authentication source {auth_source}.",
	audit_model.SystemOAuth2ApplicationAdd:       "Added instance-wide OAuth2 application {oauth2_application}.",
	audit_model.SystemOAuth2ApplicationUpdate:    "Updated instance-wide OAuth2 application {oauth2_application}.",
	audit_model.SystemOAuth2ApplicationSecret:    "Regenerated secret for instance-wide OAuth2 application {oauth2_application}.",
	audit_model.SystemOAuth2ApplicationRemove:    "Removed instance-wide OAuth2 application {oauth2_application}.",
}

// renderMessage fills the action's template from the event's scope, actor and
// metadata. A missing template or an unresolved placeholder is logged and
// rendered as the bare key: audit recording must never fail the request that
// triggered it, and a partial message is more useful than none.
func renderMessage(action audit_model.Action, actor, scope EntityRef, metadata map[string]any) string {
	tmpl, ok := messages[action]
	if !ok {
		log.Error("audit: no message template for action %q", action)
		return string(action)
	}

	var sb strings.Builder
	rest := tmpl
	for {
		start := strings.IndexByte(rest, '{')
		if start < 0 {
			break
		}
		end := strings.IndexByte(rest[start:], '}')
		if end < 0 {
			break
		}
		end += start

		key := rest[start+1 : end]
		sb.WriteString(rest[:start])
		sb.WriteString(resolvePlaceholder(action, key, actor, scope, metadata))
		rest = rest[end+1:]
	}
	sb.WriteString(rest)

	return sb.String()
}

func resolvePlaceholder(action audit_model.Action, key string, actor, scope EntityRef, metadata map[string]any) string {
	switch key {
	case placeholderScope:
		return scope.DisplayName()
	case placeholderActor:
		return actor.DisplayName()
	}
	if v, ok := metadata[key]; ok {
		return fmt.Sprint(v)
	}
	log.Error("audit: action %q has no metadata for placeholder %q", action, key)
	return key
}
