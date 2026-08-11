// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

type Action string

const (
	UserImpersonation               Action = "user:impersonation"
	UserCreate                      Action = "user:create"
	UserDelete                      Action = "user:delete"
	UserAuthenticationFailTwoFactor Action = "user:authentication:fail:twofactor"
	UserAuthenticationSource        Action = "user:authentication:source"
	UserActive                      Action = "user:active"
	UserRestricted                  Action = "user:restricted"
	UserAdmin                       Action = "user:admin"
	UserName                        Action = "user:name"
	UserPassword                    Action = "user:password"
	UserPasswordResetRequest        Action = "user:password:resetrequest"
	UserVisibility                  Action = "user:visibility"
	UserEmailPrimaryChange          Action = "user:email:primary"
	UserEmailAdd                    Action = "user:email:add"
	UserEmailActivate               Action = "user:email:activate"
	UserEmailRemove                 Action = "user:email:remove"
	UserTwoFactorEnable             Action = "user:twofactor:enable"
	UserTwoFactorRegenerate         Action = "user:twofactor:regenerate"
	UserTwoFactorDisable            Action = "user:twofactor:disable"
	UserWebAuthAdd                  Action = "user:webauth:add"
	UserWebAuthRemove               Action = "user:webauth:remove"
	UserExternalLoginAdd            Action = "user:externallogin:add"
	UserExternalLoginRemove         Action = "user:externallogin:remove"
	UserOpenIDAdd                   Action = "user:openid:add"
	UserOpenIDRemove                Action = "user:openid:remove"
	UserAccessTokenAdd              Action = "user:accesstoken:add"
	UserAccessTokenRemove           Action = "user:accesstoken:remove"
	UserOAuth2ApplicationAdd        Action = "user:oauth2application:add"
	UserOAuth2ApplicationUpdate     Action = "user:oauth2application:update"
	UserOAuth2ApplicationSecret     Action = "user:oauth2application:secret"
	UserOAuth2ApplicationGrant      Action = "user:oauth2application:grant"
	UserOAuth2ApplicationRevoke     Action = "user:oauth2application:revoke"
	UserOAuth2ApplicationRemove     Action = "user:oauth2application:remove"
	UserKeySSHAdd                   Action = "user:key:ssh:add"
	UserKeySSHRemove                Action = "user:key:ssh:remove"
	UserKeyPrincipalAdd             Action = "user:key:principal:add"
	UserKeyPrincipalRemove          Action = "user:key:principal:remove"
	UserKeyGPGAdd                   Action = "user:key:gpg:add"
	UserKeyGPGRemove                Action = "user:key:gpg:remove"
	UserSecretAdd                   Action = "user:secret:add"
	UserSecretUpdate                Action = "user:secret:update"
	UserSecretRemove                Action = "user:secret:remove"
	UserWebhookAdd                  Action = "user:webhook:add"
	UserWebhookUpdate               Action = "user:webhook:update"
	UserWebhookRemove               Action = "user:webhook:remove"

	OrganizationCreate                  Action = "organization:create"
	OrganizationDelete                  Action = "organization:delete"
	OrganizationName                    Action = "organization:name"
	OrganizationVisibility              Action = "organization:visibility"
	OrganizationTeamAdd                 Action = "organization:team:add"
	OrganizationTeamUpdate              Action = "organization:team:update"
	OrganizationTeamRemove              Action = "organization:team:remove"
	OrganizationTeamPermission          Action = "organization:team:permission"
	OrganizationTeamMemberAdd           Action = "organization:team:member:add"
	OrganizationTeamMemberRemove        Action = "organization:team:member:remove"
	OrganizationOAuth2ApplicationAdd    Action = "organization:oauth2application:add"
	OrganizationOAuth2ApplicationUpdate Action = "organization:oauth2application:update"
	OrganizationOAuth2ApplicationSecret Action = "organization:oauth2application:secret"
	OrganizationOAuth2ApplicationRemove Action = "organization:oauth2application:remove"
	OrganizationSecretAdd               Action = "organization:secret:add"
	OrganizationSecretUpdate            Action = "organization:secret:update"
	OrganizationSecretRemove            Action = "organization:secret:remove"
	OrganizationWebhookAdd              Action = "organization:webhook:add"
	OrganizationWebhookUpdate           Action = "organization:webhook:update"
	OrganizationWebhookRemove           Action = "organization:webhook:remove"

	RepositoryCreate                 Action = "repository:create"
	RepositoryCreateFork             Action = "repository:create:fork"
	RepositoryArchive                Action = "repository:archive"
	RepositoryUnarchive              Action = "repository:unarchive"
	RepositoryDelete                 Action = "repository:delete"
	RepositoryName                   Action = "repository:name"
	RepositoryVisibility             Action = "repository:visibility"
	RepositoryConvertFork            Action = "repository:convert:fork"
	RepositoryConvertMirror          Action = "repository:convert:mirror"
	RepositoryMirrorPushAdd          Action = "repository:mirror:push:add"
	RepositoryMirrorPushRemove       Action = "repository:mirror:push:remove"
	RepositorySigningVerification    Action = "repository:signingverification"
	RepositoryTransferStart          Action = "repository:transfer:start"
	RepositoryTransferFinish         Action = "repository:transfer:finish"
	RepositoryTransferCancel         Action = "repository:transfer:cancel"
	RepositoryWikiDelete             Action = "repository:wiki:delete"
	RepositoryCollaboratorAdd        Action = "repository:collaborator:add"
	RepositoryCollaboratorAccess     Action = "repository:collaborator:access"
	RepositoryCollaboratorRemove     Action = "repository:collaborator:remove"
	RepositoryCollaboratorTeamAdd    Action = "repository:collaborator:team:add"
	RepositoryCollaboratorTeamRemove Action = "repository:collaborator:team:remove"
	RepositoryBranchDefault          Action = "repository:branch:default"
	RepositoryBranchProtectionAdd    Action = "repository:branch:protection:add"
	RepositoryBranchProtectionUpdate Action = "repository:branch:protection:update"
	RepositoryBranchProtectionRemove Action = "repository:branch:protection:remove"
	RepositoryTagProtectionAdd       Action = "repository:tag:protection:add"
	RepositoryTagProtectionUpdate    Action = "repository:tag:protection:update"
	RepositoryTagProtectionRemove    Action = "repository:tag:protection:remove"
	RepositoryWebhookAdd             Action = "repository:webhook:add"
	RepositoryWebhookUpdate          Action = "repository:webhook:update"
	RepositoryWebhookRemove          Action = "repository:webhook:remove"
	RepositoryDeployKeyAdd           Action = "repository:deploykey:add"
	RepositoryDeployKeyRemove        Action = "repository:deploykey:remove"
	RepositorySecretAdd              Action = "repository:secret:add"
	RepositorySecretUpdate           Action = "repository:secret:update"
	RepositorySecretRemove           Action = "repository:secret:remove"

	IssueCreate        Action = "issue:create"
	IssueDelete        Action = "issue:delete"
	IssueCommentCreate Action = "issue:comment:create"
	IssueCommentDelete Action = "issue:comment:delete"

	PullRequestCreate        Action = "pr:create"
	PullRequestDelete        Action = "pr:delete"
	PullRequestMerge         Action = "pr:merge"
	PullRequestCommentCreate Action = "pr:comment:create"
	PullRequestCommentDelete Action = "pr:comment:delete"

	ProjectCreate Action = "project:create"
	ProjectUpdate Action = "project:update"
	ProjectDelete Action = "project:delete"

	WikiPageCreate Action = "wiki:page:create"
	WikiPageUpdate Action = "wiki:page:update"
	WikiPageDelete Action = "wiki:page:delete"

	ActionsWorkflowEnable   Action = "actions:workflow:enable"
	ActionsWorkflowDisable  Action = "actions:workflow:disable"
	ActionsWorkflowDispatch Action = "actions:workflow:dispatch"

	SystemStartup                    Action = "system:startup"
	SystemShutdown                   Action = "system:shutdown"
	SystemWebhookAdd                 Action = "system:webhook:add"
	SystemWebhookUpdate              Action = "system:webhook:update"
	SystemWebhookRemove              Action = "system:webhook:remove"
	SystemAuthenticationSourceAdd    Action = "system:authenticationsource:add"
	SystemAuthenticationSourceUpdate Action = "system:authenticationsource:update"
	SystemAuthenticationSourceRemove Action = "system:authenticationsource:remove"
	SystemOAuth2ApplicationAdd       Action = "system:oauth2application:add"
	SystemOAuth2ApplicationUpdate    Action = "system:oauth2application:update"
	SystemOAuth2ApplicationSecret    Action = "system:oauth2application:secret"
	SystemOAuth2ApplicationRemove    Action = "system:oauth2application:remove"
)

// AllActions returns every declared action. Keeping the list here means a new
// action is caught by the tests that walk it, rather than silently shipping
// without a message template or documentation entry.
func AllActions() []Action {
	return []Action{
		UserImpersonation,
		UserCreate,
		UserDelete,
		UserAuthenticationFailTwoFactor,
		UserAuthenticationSource,
		UserActive,
		UserRestricted,
		UserAdmin,
		UserName,
		UserPassword,
		UserPasswordResetRequest,
		UserVisibility,
		UserEmailPrimaryChange,
		UserEmailAdd,
		UserEmailActivate,
		UserEmailRemove,
		UserTwoFactorEnable,
		UserTwoFactorRegenerate,
		UserTwoFactorDisable,
		UserWebAuthAdd,
		UserWebAuthRemove,
		UserExternalLoginAdd,
		UserExternalLoginRemove,
		UserOpenIDAdd,
		UserOpenIDRemove,
		UserAccessTokenAdd,
		UserAccessTokenRemove,
		UserOAuth2ApplicationAdd,
		UserOAuth2ApplicationUpdate,
		UserOAuth2ApplicationSecret,
		UserOAuth2ApplicationGrant,
		UserOAuth2ApplicationRevoke,
		UserOAuth2ApplicationRemove,
		UserKeySSHAdd,
		UserKeySSHRemove,
		UserKeyPrincipalAdd,
		UserKeyPrincipalRemove,
		UserKeyGPGAdd,
		UserKeyGPGRemove,
		UserSecretAdd,
		UserSecretUpdate,
		UserSecretRemove,
		UserWebhookAdd,
		UserWebhookUpdate,
		UserWebhookRemove,

		OrganizationCreate,
		OrganizationDelete,
		OrganizationName,
		OrganizationVisibility,
		OrganizationTeamAdd,
		OrganizationTeamUpdate,
		OrganizationTeamRemove,
		OrganizationTeamPermission,
		OrganizationTeamMemberAdd,
		OrganizationTeamMemberRemove,
		OrganizationOAuth2ApplicationAdd,
		OrganizationOAuth2ApplicationUpdate,
		OrganizationOAuth2ApplicationSecret,
		OrganizationOAuth2ApplicationRemove,
		OrganizationSecretAdd,
		OrganizationSecretUpdate,
		OrganizationSecretRemove,
		OrganizationWebhookAdd,
		OrganizationWebhookUpdate,
		OrganizationWebhookRemove,

		RepositoryCreate,
		RepositoryCreateFork,
		RepositoryArchive,
		RepositoryUnarchive,
		RepositoryDelete,
		RepositoryName,
		RepositoryVisibility,
		RepositoryConvertFork,
		RepositoryConvertMirror,
		RepositoryMirrorPushAdd,
		RepositoryMirrorPushRemove,
		RepositorySigningVerification,
		RepositoryTransferStart,
		RepositoryTransferFinish,
		RepositoryTransferCancel,
		RepositoryWikiDelete,
		RepositoryCollaboratorAdd,
		RepositoryCollaboratorAccess,
		RepositoryCollaboratorRemove,
		RepositoryCollaboratorTeamAdd,
		RepositoryCollaboratorTeamRemove,
		RepositoryBranchDefault,
		RepositoryBranchProtectionAdd,
		RepositoryBranchProtectionUpdate,
		RepositoryBranchProtectionRemove,
		RepositoryTagProtectionAdd,
		RepositoryTagProtectionUpdate,
		RepositoryTagProtectionRemove,
		RepositoryWebhookAdd,
		RepositoryWebhookUpdate,
		RepositoryWebhookRemove,
		RepositoryDeployKeyAdd,
		RepositoryDeployKeyRemove,
		RepositorySecretAdd,
		RepositorySecretUpdate,
		RepositorySecretRemove,

		IssueCreate,
		IssueDelete,
		IssueCommentCreate,
		IssueCommentDelete,

		PullRequestCreate,
		PullRequestDelete,
		PullRequestMerge,
		PullRequestCommentCreate,
		PullRequestCommentDelete,

		ProjectCreate,
		ProjectUpdate,
		ProjectDelete,

		WikiPageCreate,
		WikiPageUpdate,
		WikiPageDelete,

		ActionsWorkflowEnable,
		ActionsWorkflowDisable,
		ActionsWorkflowDispatch,

		SystemStartup,
		SystemShutdown,
		SystemWebhookAdd,
		SystemWebhookUpdate,
		SystemWebhookRemove,
		SystemAuthenticationSourceAdd,
		SystemAuthenticationSourceUpdate,
		SystemAuthenticationSourceRemove,
		SystemOAuth2ApplicationAdd,
		SystemOAuth2ApplicationUpdate,
		SystemOAuth2ApplicationSecret,
		SystemOAuth2ApplicationRemove,
	}
}
