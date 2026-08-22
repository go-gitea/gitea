// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

type Action string

var (
	actionMessages = map[Action]string{}
	allActions     []Action
)

func define(id, message string) Action {
	a := Action(id)
	if _, exists := actionMessages[a]; exists {
		panic("duplicate audit action: " + id)
	}
	actionMessages[a] = message
	allActions = append(allActions, a)
	return a
}

// MessageTemplate returns the message template registered for an action.
func MessageTemplate(a Action) (string, bool) {
	m, ok := actionMessages[a]
	return m, ok
}

// AllActions returns every registered action.
func AllActions() []Action {
	return allActions
}

var (
	UserImpersonation               = define("user:impersonation", "User {actor} impersonating user {scope}.")
	UserImpersonationExit           = define("user:impersonation:exit", "User {actor} stopped impersonating user {scope}.")
	UserCreate                      = define("user:create", "Created user {scope}.")
	UserDelete                      = define("user:delete", "Deleted user {scope}.")
	UserAuthenticationFailTwoFactor = define("user:authentication:fail:twofactor", "Failed two-factor authentication for user {scope}.")
	UserAuthenticationSource        = define("user:authentication:source", "Changed authentication source of user {scope}.")
	UserActive                      = define("user:active", "Changed activation status of user {scope}.")
	UserRestricted                  = define("user:restricted", "Changed restricted status of user {scope}.")
	UserAdmin                       = define("user:admin", "Changed admin status of user {scope}.")
	UserName                        = define("user:name", "Changed user name to {scope}.")
	UserPassword                    = define("user:password", "Changed password of user {scope}.")
	UserPasswordResetRequest        = define("user:password:resetrequest", "Requested password reset for user {scope}.")
	UserVisibility                  = define("user:visibility", "Changed visibility of user {scope}.")
	UserEmailPrimaryChange          = define("user:email:primary", "Changed primary email of user {scope} to {email}.")
	UserEmailAdd                    = define("user:email:add", "Added email {email} to user {scope}.")
	UserEmailActivate               = define("user:email:activate", "Changed activation status of email {email} of user {scope}.")
	UserEmailRemove                 = define("user:email:remove", "Removed email {email} from user {scope}.")
	UserTwoFactorEnable             = define("user:twofactor:enable", "Enabled two-factor authentication for user {scope}.")
	UserTwoFactorRegenerate         = define("user:twofactor:regenerate", "Regenerated two-factor authentication secret for user {scope}.")
	UserTwoFactorDisable            = define("user:twofactor:disable", "Disabled two-factor authentication for user {scope}.")
	UserWebAuthAdd                  = define("user:webauth:add", "Added WebAuthn key {credential} for user {scope}.")
	UserWebAuthRemove               = define("user:webauth:remove", "Removed WebAuthn key {credential} from user {scope}.")
	UserExternalLoginAdd            = define("user:externallogin:add", "Added external login {external_id} for user {scope} using provider {provider}.")
	UserExternalLoginRemove         = define("user:externallogin:remove", "Removed external login from authentication source {auth_source_id} for user {scope}.")
	UserOpenIDAdd                   = define("user:openid:add", "Associated OpenID {openid} to user {scope}.")
	UserOpenIDRemove                = define("user:openid:remove", "Removed OpenID {openid} from user {scope}.")
	UserAccessTokenAdd              = define("user:accesstoken:add", "Added access token {token} for user {scope} with scope {token_scope}.")
	UserAccessTokenRemove           = define("user:accesstoken:remove", "Removed access token {token} from user {scope}.")
	UserOAuth2ApplicationAdd        = define("user:oauth2application:add", "Added OAuth2 application {oauth2_application} for user {scope}.")
	UserOAuth2ApplicationUpdate     = define("user:oauth2application:update", "Updated OAuth2 application {oauth2_application} of user {scope}.")
	UserOAuth2ApplicationSecret     = define("user:oauth2application:secret", "Regenerated secret for OAuth2 application {oauth2_application} of user {scope}.")
	UserOAuth2ApplicationGrant      = define("user:oauth2application:grant", "Granted OAuth2 application {oauth2_application} access to user {scope}.")
	UserOAuth2ApplicationRevoke     = define("user:oauth2application:revoke", "Revoked OAuth2 grant for application {oauth2_application} of user {scope}.")
	UserOAuth2ApplicationRemove     = define("user:oauth2application:remove", "Removed OAuth2 application {oauth2_application} of user {scope}.")
	UserKeySSHAdd                   = define("user:key:ssh:add", "Added SSH key {fingerprint} for user {scope}.")
	UserKeySSHRemove                = define("user:key:ssh:remove", "Removed SSH key {fingerprint} of user {scope}.")
	UserKeyPrincipalAdd             = define("user:key:principal:add", "Added principal key {key} for user {scope}.")
	UserKeyPrincipalRemove          = define("user:key:principal:remove", "Removed principal key {key} of user {scope}.")
	UserKeyGPGAdd                   = define("user:key:gpg:add", "Added GPG key {gpg_key_id} for user {scope}.")
	UserKeyGPGRemove                = define("user:key:gpg:remove", "Removed GPG key {gpg_key_id} of user {scope}.")
	UserSecretAdd                   = define("user:secret:add", "Added secret {secret} to user {scope}.")
	UserSecretUpdate                = define("user:secret:update", "Updated secret {secret} of user {scope}.")
	UserSecretRemove                = define("user:secret:remove", "Removed secret {secret} from user {scope}.")
	UserWebhookAdd                  = define("user:webhook:add", "Added webhook {webhook} to user {scope}.")
	UserWebhookUpdate               = define("user:webhook:update", "Updated webhook {webhook} of user {scope}.")
	UserWebhookRemove               = define("user:webhook:remove", "Removed webhook {webhook} of user {scope}.")

	OrganizationCreate                  = define("organization:create", "Created organization {scope}.")
	OrganizationDelete                  = define("organization:delete", "Deleted organization {scope}.")
	OrganizationName                    = define("organization:name", "Changed organization name to {scope}.")
	OrganizationVisibility              = define("organization:visibility", "Changed visibility of organization {scope} to {new_visibility}.")
	OrganizationTeamAdd                 = define("organization:team:add", "Added team {team} to organization {scope}.")
	OrganizationTeamUpdate              = define("organization:team:update", "Updated settings of team {scope}/{team}.")
	OrganizationTeamRemove              = define("organization:team:remove", "Removed team {team} from organization {scope}.")
	OrganizationTeamPermission          = define("organization:team:permission", "Changed permission of team {scope}/{team} to {permission}.")
	OrganizationTeamMemberAdd           = define("organization:team:member:add", "Added user {member} to team {scope}/{team}.")
	OrganizationTeamMemberRemove        = define("organization:team:member:remove", "Removed user {member} from team {scope}/{team}.")
	OrganizationOAuth2ApplicationAdd    = define("organization:oauth2application:add", "Added OAuth2 application {oauth2_application} for organization {scope}.")
	OrganizationOAuth2ApplicationUpdate = define("organization:oauth2application:update", "Updated OAuth2 application {oauth2_application} of organization {scope}.")
	OrganizationOAuth2ApplicationSecret = define("organization:oauth2application:secret", "Regenerated secret for OAuth2 application {oauth2_application} of organization {scope}.")
	OrganizationOAuth2ApplicationRemove = define("organization:oauth2application:remove", "Removed OAuth2 application {oauth2_application} of organization {scope}.")
	OrganizationSecretAdd               = define("organization:secret:add", "Added secret {secret} to organization {scope}.")
	OrganizationSecretUpdate            = define("organization:secret:update", "Updated secret {secret} of organization {scope}.")
	OrganizationSecretRemove            = define("organization:secret:remove", "Removed secret {secret} from organization {scope}.")
	OrganizationWebhookAdd              = define("organization:webhook:add", "Added webhook {webhook} to organization {scope}.")
	OrganizationWebhookUpdate           = define("organization:webhook:update", "Updated webhook {webhook} of organization {scope}.")
	OrganizationWebhookRemove           = define("organization:webhook:remove", "Removed webhook {webhook} of organization {scope}.")

	RepositoryCreate                 = define("repository:create", "Created repository {scope}.")
	RepositoryCreateFork             = define("repository:create:fork", "Created fork {scope} of repository {base_repo}.")
	RepositoryArchive                = define("repository:archive", "Archived repository {scope}.")
	RepositoryUnarchive              = define("repository:unarchive", "Unarchived repository {scope}.")
	RepositoryDelete                 = define("repository:delete", "Deleted repository {scope}.")
	RepositoryName                   = define("repository:name", "Changed repository name from {previous_name} to {scope}.")
	RepositoryVisibility             = define("repository:visibility", "Changed visibility of repository {scope}.")
	RepositoryConvertFork            = define("repository:convert:fork", "Converted repository {scope} from fork to regular repository.")
	RepositoryConvertMirror          = define("repository:convert:mirror", "Converted repository {scope} from pull mirror to regular repository.")
	RepositoryMirrorPushAdd          = define("repository:mirror:push:add", "Added push mirror to {remote_address} for repository {scope}.")
	RepositoryMirrorPushRemove       = define("repository:mirror:push:remove", "Removed push mirror to {remote_address} for repository {scope}.")
	RepositorySigningVerification    = define("repository:signingverification", "Changed signing verification of repository {scope} to {trust_model}.")
	RepositoryTransferStart          = define("repository:transfer:start", "Started repository transfer of {scope} to {new_owner}.")
	RepositoryTransferFinish         = define("repository:transfer:finish", "Transferred repository {scope} from {old_owner} to {new_owner}.")
	RepositoryTransferCancel         = define("repository:transfer:cancel", "Canceled transfer of repository {scope}.")
	RepositoryWikiDelete             = define("repository:wiki:delete", "Deleted wiki of repository {scope}.")
	RepositoryCollaboratorAdd        = define("repository:collaborator:add", "Added user {collaborator} as collaborator for repository {scope} with access mode {access_mode}.")
	RepositoryCollaboratorAccess     = define("repository:collaborator:access", "Changed access mode of collaborator {collaborator} of repository {scope} to {access_mode}.")
	RepositoryCollaboratorRemove     = define("repository:collaborator:remove", "Removed collaborator {collaborator} from repository {scope}.")
	RepositoryCollaboratorTeamAdd    = define("repository:collaborator:team:add", "Added team {team} as collaborator for repository {scope}.")
	RepositoryCollaboratorTeamRemove = define("repository:collaborator:team:remove", "Removed team {team} as collaborator from repository {scope}.")
	RepositoryBranchDefault          = define("repository:branch:default", "Changed default branch of repository {scope} to {default_branch}.")
	RepositoryBranchProtectionAdd    = define("repository:branch:protection:add", "Added branch protection {rule} for repository {scope}.")
	RepositoryBranchProtectionUpdate = define("repository:branch:protection:update", "Updated branch protection {rule} for repository {scope}.")
	RepositoryBranchProtectionRemove = define("repository:branch:protection:remove", "Removed branch protection {rule} from repository {scope}.")
	RepositoryTagProtectionAdd       = define("repository:tag:protection:add", "Added tag protection {pattern} for repository {scope}.")
	RepositoryTagProtectionUpdate    = define("repository:tag:protection:update", "Updated tag protection {pattern} for repository {scope}.")
	RepositoryTagProtectionRemove    = define("repository:tag:protection:remove", "Removed tag protection {pattern} from repository {scope}.")
	RepositoryWebhookAdd             = define("repository:webhook:add", "Added webhook {webhook} to repository {scope}.")
	RepositoryWebhookUpdate          = define("repository:webhook:update", "Updated webhook {webhook} of repository {scope}.")
	RepositoryWebhookRemove          = define("repository:webhook:remove", "Removed webhook {webhook} of repository {scope}.")
	RepositoryDeployKeyAdd           = define("repository:deploykey:add", "Added deploy key {deploy_key} for repository {scope}.")
	RepositoryDeployKeyRemove        = define("repository:deploykey:remove", "Removed deploy key {deploy_key} from repository {scope}.")
	RepositorySecretAdd              = define("repository:secret:add", "Added secret {secret} to repository {scope}.")
	RepositorySecretUpdate           = define("repository:secret:update", "Updated secret {secret} of repository {scope}.")
	RepositorySecretRemove           = define("repository:secret:remove", "Removed secret {secret} from repository {scope}.")

	IssueCreate        = define("issue:create", "Created issue {issue} in repository {scope}.")
	IssueDelete        = define("issue:delete", "Deleted issue {issue} from repository {scope}.")
	IssueCommentCreate = define("issue:comment:create", "Added comment {comment_id} to issue {issue} in repository {scope}.")
	IssueCommentDelete = define("issue:comment:delete", "Deleted comment {comment_id} from issue {issue} in repository {scope}.")

	PullRequestCreate        = define("pr:create", "Created pull request {pull_request} in repository {scope}.")
	PullRequestDelete        = define("pr:delete", "Deleted pull request {pull_request} from repository {scope}.")
	PullRequestMerge         = define("pr:merge", "Merged pull request {pull_request} in repository {scope}.")
	PullRequestCommentCreate = define("pr:comment:create", "Added comment {comment_id} to pull request {pull_request} in repository {scope}.")
	PullRequestCommentDelete = define("pr:comment:delete", "Deleted comment {comment_id} from pull request {pull_request} in repository {scope}.")

	ProjectCreate = define("project:create", "Created project {project} in {scope}.")
	ProjectUpdate = define("project:update", "Updated project {project} in {scope}.")
	ProjectDelete = define("project:delete", "Deleted project {project} from {scope}.")

	WikiPageCreate = define("wiki:page:create", "Created wiki page {page} in repository {scope}.")
	WikiPageUpdate = define("wiki:page:update", "Updated wiki page {page} in repository {scope}.")
	WikiPageDelete = define("wiki:page:delete", "Deleted wiki page {page} from repository {scope}.")

	ActionsWorkflowEnable   = define("actions:workflow:enable", "Enabled Actions workflow {workflow} in repository {scope}.")
	ActionsWorkflowDisable  = define("actions:workflow:disable", "Disabled Actions workflow {workflow} in repository {scope}.")
	ActionsWorkflowDispatch = define("actions:workflow:dispatch", "Dispatched Actions workflow {workflow} on {ref} in repository {scope}.")

	// Do not change the startup message anymore. We guarantee the stability of this message for
	// users wanting to parse the log themselves to be able to trace back events across gitea versions.
	SystemStartup                    = define("system:startup", "System started [Gitea {version}]")
	SystemShutdown                   = define("system:shutdown", "System shutdown")
	SystemWebhookAdd                 = define("system:webhook:add", "Added instance-wide webhook {webhook}.")
	SystemWebhookUpdate              = define("system:webhook:update", "Updated instance-wide webhook {webhook}.")
	SystemWebhookRemove              = define("system:webhook:remove", "Removed instance-wide webhook {webhook}.")
	SystemAuthenticationSourceAdd    = define("system:authenticationsource:add", "Created authentication source {auth_source}.")
	SystemAuthenticationSourceUpdate = define("system:authenticationsource:update", "Updated authentication source {auth_source}.")
	SystemAuthenticationSourceRemove = define("system:authenticationsource:remove", "Removed authentication source {auth_source}.")
	SystemOAuth2ApplicationAdd       = define("system:oauth2application:add", "Added instance-wide OAuth2 application {oauth2_application}.")
	SystemOAuth2ApplicationUpdate    = define("system:oauth2application:update", "Updated instance-wide OAuth2 application {oauth2_application}.")
	SystemOAuth2ApplicationSecret    = define("system:oauth2application:secret", "Regenerated secret for instance-wide OAuth2 application {oauth2_application}.")
	SystemOAuth2ApplicationRemove    = define("system:oauth2application:remove", "Removed instance-wide OAuth2 application {oauth2_application}.")
)
