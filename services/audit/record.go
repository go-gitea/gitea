// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"fmt"
	"time"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	audit_model "code.gitea.io/gitea/models/audit"
	auth_model "code.gitea.io/gitea/models/auth"
	git_model "code.gitea.io/gitea/models/git"
	organization_model "code.gitea.io/gitea/models/organization"
	perm_model "code.gitea.io/gitea/models/perm"
	repository_model "code.gitea.io/gitea/models/repo"
	secret_model "code.gitea.io/gitea/models/secret"
	user_model "code.gitea.io/gitea/models/user"
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

type Event struct {
	Action    audit_model.Action `json:"action"`
	Actor     TypeDescriptor     `json:"actor"`
	Scope     TypeDescriptor     `json:"scope"`
	Target    TypeDescriptor     `json:"target"`
	Message   string             `json:"message"`
	Time      time.Time          `json:"time"`
	IPAddress string             `json:"ip_address"`
}

func buildEvent(ctx context.Context, action audit_model.Action, actor *user_model.User, scope, target any, message string, v ...any) *Event {
	return &Event{
		Action:    action,
		Actor:     typeToDescription(actor),
		Scope:     scopeToDescription(scope),
		Target:    typeToDescription(target),
		Message:   fmt.Sprintf(message, v...),
		Time:      time.Now(),
		IPAddress: httplib.TryGetIPAddress(ctx),
	}
}

func record(ctx context.Context, action audit_model.Action, actor *user_model.User, scope, target any, message string, v ...any) {
	if !setting.Audit.Enabled {
		return
	}

	e := buildEvent(ctx, action, actor, scope, target, message, v...)

	if err := writeToFile(e); err != nil {
		log.Error("Error writing audit event to file: %v", err)
	}
	if err := writeToDatabase(ctx, e); err != nil {
		log.Error("Error writing audit event %+v to database: %v", e, err)
	}
}

func RecordUserImpersonation(ctx context.Context, impersonator, target *user_model.User) {
	record(ctx, audit_model.UserImpersonation, impersonator, impersonator, target, "User %s impersonating user %s.", impersonator.Name, target.Name)
}

func RecordUserCreate(ctx context.Context, doer, user *user_model.User) {
	if user.IsOrganization() {
		record(ctx, audit_model.OrganizationCreate, doer, user, user, "Created organization %s.", user.Name)
	} else {
		record(ctx, audit_model.UserCreate, doer, user, user, "Created user %s.", user.Name)
	}
}

func RecordUserDelete(ctx context.Context, doer, user *user_model.User) {
	if user.IsOrganization() {
		record(ctx, audit_model.OrganizationDelete, doer, user, user, "Deleted organization %s.", user.Name)
	} else {
		record(ctx, audit_model.UserDelete, doer, user, user, "Deleted user %s.", user.Name)
	}
}

func RecordUserAuthenticationFailTwoFactor(ctx context.Context, user *user_model.User) {
	record(ctx, audit_model.UserAuthenticationFailTwoFactor, user, user, user, "Failed two-factor authentication for user %s.", user.Name)
}

func RecordUserAuthenticationSource(ctx context.Context, doer, user *user_model.User) {
	record(ctx, audit_model.UserAuthenticationSource, doer, user, user, "Changed authentication source of user %s.", user.Name)
}

func RecordUserActive(ctx context.Context, doer, user *user_model.User) {
	status := "active"
	if !user.IsActive {
		status = "inactive"
	}

	record(ctx, audit_model.UserActive, doer, user, user, "Changed activation status of user %s to %s.", user.Name, status)
}

func RecordUserRestricted(ctx context.Context, doer, user *user_model.User) {
	status := "restricted"
	if !user.IsRestricted {
		status = "unrestricted"
	}

	record(ctx, audit_model.UserRestricted, doer, user, user, "Changed restricted status of user %s to %s.", user.Name, status)
}

func RecordUserAdmin(ctx context.Context, doer, user *user_model.User) {
	status := "admin"
	if !user.IsAdmin {
		status = "normal user"
	}

	record(ctx, audit_model.UserAdmin, doer, user, user, "Changed admin status of user %s to %s.", user.Name, status)
}

func RecordUserName(ctx context.Context, doer, user *user_model.User) {
	if user.IsOrganization() {
		record(ctx, audit_model.OrganizationName, doer, user, user, "Changed organization name to %s.", user.Name)
	} else {
		record(ctx, audit_model.UserName, doer, user, user, "Changed user name to %s.", user.Name)
	}
}

func RecordUserPassword(ctx context.Context, doer, user *user_model.User) {
	record(ctx, audit_model.UserPassword, doer, user, user, "Changed password of user %s.", user.Name)
}

func RecordUserPasswordResetRequest(ctx context.Context, doer, user *user_model.User) {
	record(ctx, audit_model.UserPasswordResetRequest, doer, user, user, "Requested password reset for user %s.", user.Name)
}

func RecordUserVisibility(ctx context.Context, doer, user *user_model.User) {
	if user.IsOrganization() {
		record(ctx, audit_model.OrganizationVisibility, doer, user, user, "Changed visibility of organization %s to %s.", user.Name, user.Visibility.String())
	} else {
		record(ctx, audit_model.UserVisibility, doer, user, user, "Changed visibility of user %s to %s.", user.Name, user.Visibility.String())
	}
}

func RecordUserEmailPrimaryChange(ctx context.Context, doer, user *user_model.User, email *user_model.EmailAddress) {
	record(ctx, audit_model.UserEmailPrimaryChange, doer, user, email, "Changed primary email of user %s to %s.", user.Name, email.Email)
}

func RecordUserEmailAdd(ctx context.Context, doer, user *user_model.User, email *user_model.EmailAddress) {
	record(ctx, audit_model.UserEmailAdd, doer, user, email, "Added email %s to user %s.", email.Email, user.Name)
}

func RecordUserEmailActivate(ctx context.Context, doer, user *user_model.User, email *user_model.EmailAddress) {
	status := "active"
	if !email.IsActivated {
		status = "inactive"
	}

	record(ctx, audit_model.UserEmailActivate, doer, user, email, "Changed activation status of email %s of user %s to %s.", email.Email, user.Name, status)
}

func RecordUserEmailRemove(ctx context.Context, doer, user *user_model.User, email *user_model.EmailAddress) {
	record(ctx, audit_model.UserEmailRemove, doer, user, email, "Removed email %s from user %s.", email.Email, user.Name)
}

func RecordUserTwoFactorEnable(ctx context.Context, doer, user *user_model.User) {
	record(ctx, audit_model.UserTwoFactorEnable, doer, user, user, "Enabled two-factor authentication for user %s.", user.Name)
}

func RecordUserTwoFactorRegenerate(ctx context.Context, doer, user *user_model.User, tf *auth_model.TwoFactor) {
	record(ctx, audit_model.UserTwoFactorRegenerate, doer, user, tf, "Regenerated two-factor authentication secret for user %s.", user.Name)
}

func RecordUserTwoFactorDisable(ctx context.Context, doer, user *user_model.User, tf *auth_model.TwoFactor) {
	record(ctx, audit_model.UserTwoFactorDisable, doer, user, tf, "Disabled two-factor authentication for user %s.", user.Name)
}

func RecordUserWebAuthAdd(ctx context.Context, doer, user *user_model.User, authn *auth_model.WebAuthnCredential) {
	record(ctx, audit_model.UserWebAuthAdd, doer, user, authn, "Added WebAuthn key %s for user %s.", authn.Name, user.Name)
}

func RecordUserWebAuthRemove(ctx context.Context, doer, user *user_model.User, authn *auth_model.WebAuthnCredential) {
	record(ctx, audit_model.UserWebAuthRemove, doer, user, authn, "Removed WebAuthn key %s from user %s.", authn.Name, user.Name)
}

func RecordUserExternalLoginAdd(ctx context.Context, doer, user *user_model.User, externalLogin *user_model.ExternalLoginUser) {
	record(ctx, audit_model.UserExternalLoginAdd, doer, user, "Added external login %s for user %s using provider %s.", externalLogin.ExternalID, user.Name, externalLogin.Provider)
}

func RecordUserExternalLoginRemove(ctx context.Context, doer, user *user_model.User, externalLogin *user_model.ExternalLoginUser) {
	record(ctx, audit_model.UserExternalLoginRemove, doer, user, "Removed external login %s for user %s from provider.", externalLogin.ExternalID, user.Name, externalLogin.Provider)
}

func RecordUserOpenIDAdd(ctx context.Context, doer, user *user_model.User, oid *user_model.UserOpenID) {
	record(ctx, audit_model.UserOpenIDAdd, doer, user, oid, "Associated OpenID %s to user %s.", oid.URI, user.Name)
}

func RecordUserOpenIDRemove(ctx context.Context, doer, user *user_model.User, oid *user_model.UserOpenID) {
	record(ctx, audit_model.UserOpenIDRemove, doer, user, oid, "Removed OpenID %s from user %s.", oid.URI, user.Name)
}

func RecordUserAccessTokenAdd(ctx context.Context, doer, user *user_model.User, token *auth_model.AccessToken) {
	record(ctx, audit_model.UserAccessTokenAdd, doer, user, token, "Added access token %s for user %s with scope %s.", token.Name, user.Name, token.Scope)
}

func RecordUserAccessTokenRemove(ctx context.Context, doer, user *user_model.User, token *auth_model.AccessToken) {
	record(ctx, audit_model.UserAccessTokenRemove, doer, user, token, "Removed access token %s from user %s.", token.Name, user.Name)
}

func RecordOAuth2ApplicationAdd(ctx context.Context, doer, user *user_model.User, app *auth_model.OAuth2Application) {
	if user == nil {
		record(ctx, audit_model.SystemOAuth2ApplicationAdd, doer, &systemObject, app, "Created instance-wide OAuth2 application %s", app.Name)
	} else if user.IsOrganization() {
		record(ctx, audit_model.OrganizationOAuth2ApplicationAdd, doer, user, app, "Created OAuth2 application %s for organization %s", app.Name, user.Name)
	} else {
		record(ctx, audit_model.UserOAuth2ApplicationAdd, doer, user, app, "Created OAuth2 application %s for user %s", app.Name, user.Name)
	}
}

func RecordOAuth2ApplicationUpdate(ctx context.Context, doer, user *user_model.User, app *auth_model.OAuth2Application) {
	if user == nil {
		record(ctx, audit_model.SystemOAuth2ApplicationUpdate, doer, &systemObject, app, "Updated instance-wide OAuth2 application %s", app.Name)
	} else if user.IsOrganization() {
		record(ctx, audit_model.OrganizationOAuth2ApplicationUpdate, doer, user, app, "Updated OAuth2 application %s of organization %s", app.Name, user.Name)
	} else {
		record(ctx, audit_model.UserOAuth2ApplicationUpdate, doer, user, app, "Updated OAuth2 application %s of user %s", app.Name, user.Name)
	}
}

func RecordOAuth2ApplicationSecret(ctx context.Context, doer, user *user_model.User, app *auth_model.OAuth2Application) {
	if user == nil {
		record(ctx, audit_model.SystemOAuth2ApplicationSecret, doer, &systemObject, app, "Regenerated secret for instance-wide OAuth2 application %s", app.Name)
	} else if user.IsOrganization() {
		record(ctx, audit_model.OrganizationOAuth2ApplicationSecret, doer, user, app, "Regenerated secret for OAuth2 application %s of organization %s", app.Name, user.Name)
	} else {
		record(ctx, audit_model.UserOAuth2ApplicationSecret, doer, user, app, "Regenerated secret for OAuth2 application %s of user %s", app.Name, user.Name)
	}
}

func RecordUserOAuth2ApplicationGrant(ctx context.Context, doer, owner *user_model.User, app *auth_model.OAuth2Application, grant *auth_model.OAuth2Grant) {
	record(ctx, audit_model.UserOAuth2ApplicationGrant, doer, owner, grant, "Granted OAuth2 access to application %s of user %s.", app.Name, owner.Name)
}

func RecordUserOAuth2ApplicationRevoke(ctx context.Context, doer, owner *user_model.User, app *auth_model.OAuth2Application, grant *auth_model.OAuth2Grant) {
	record(ctx, audit_model.UserOAuth2ApplicationRevoke, doer, owner, grant, "Revoked OAuth2 grant for application %s of user %s.", app.Name, owner.Name)
}

func RecordOAuth2ApplicationRemove(ctx context.Context, doer, user *user_model.User, app *auth_model.OAuth2Application) {
	if user == nil {
		record(ctx, audit_model.SystemOAuth2ApplicationRemove, doer, &systemObject, app, "Removed instance-wide OAuth2 application %s", app.Name)
	} else if user.IsOrganization() {
		record(ctx, audit_model.OrganizationOAuth2ApplicationRemove, doer, user, app, "Removed OAuth2 application %s of organization %s", app.Name, user.Name)
	} else {
		record(ctx, audit_model.UserOAuth2ApplicationRemove, doer, user, app, "Removed OAuth2 application %s of user %s", app.Name, user.Name)
	}
}

func RecordUserKeySSHAdd(ctx context.Context, doer, user *user_model.User, key *asymkey_model.PublicKey) {
	record(ctx, audit_model.UserKeySSHAdd, doer, user, key, "Added SSH key %s for user %s.", key.Fingerprint, user.Name)
}

func RecordUserKeySSHRemove(ctx context.Context, doer, user *user_model.User, key *asymkey_model.PublicKey) {
	record(ctx, audit_model.UserKeySSHRemove, doer, user, key, "Removed SSH key %s of user %s.", key.Fingerprint, user.Name)
}

func RecordUserKeyPrincipalAdd(ctx context.Context, doer, user *user_model.User, key *asymkey_model.PublicKey) {
	record(ctx, audit_model.UserKeyPrincipalAdd, doer, user, key, "Added principal key %s for user %s.", key.Name, user.Name)
}

func RecordUserKeyPrincipalRemove(ctx context.Context, doer, user *user_model.User, key *asymkey_model.PublicKey) {
	record(ctx, audit_model.UserKeyPrincipalRemove, doer, user, key, "Removed principal key %s of user %s.", key.Name, user.Name)
}

func RecordUserKeyGPGAdd(ctx context.Context, doer, user *user_model.User, key *asymkey_model.GPGKey) {
	record(ctx, audit_model.UserKeyGPGAdd, doer, user, key, "Added GPG key %s for user %s.", key.KeyID, user.Name)
}

func RecordUserKeyGPGRemove(ctx context.Context, doer, user *user_model.User, key *asymkey_model.GPGKey) {
	record(ctx, audit_model.UserKeyGPGRemove, doer, user, key, "Removed GPG key %s of user %s.", key.KeyID, user.Name)
}

func RecordSecretAdd(ctx context.Context, doer, owner *user_model.User, repo *repository_model.Repository, secret *secret_model.Secret) {
	if owner == nil {
		record(ctx, audit_model.RepositorySecretAdd, doer, repo, secret, "Added secret %s for repository %s.", secret.Name, repo.FullName())
	} else if owner.IsOrganization() {
		record(ctx, audit_model.OrganizationSecretAdd, doer, owner, secret, "Added secret %s for organization %s.", secret.Name, owner.Name)
	} else {
		record(ctx, audit_model.UserSecretAdd, doer, owner, secret, "Added secret %s for user %s.", secret.Name, owner.Name)
	}
}

func RecordSecretUpdate(ctx context.Context, doer, owner *user_model.User, repo *repository_model.Repository, secret *secret_model.Secret) {
	if owner == nil {
		record(ctx, audit_model.RepositorySecretUpdate, doer, repo, secret, "Updated secret %s of repository %s.", secret.Name, repo.FullName())
	} else if owner.IsOrganization() {
		record(ctx, audit_model.OrganizationSecretUpdate, doer, owner, secret, "Updated secret %s of organization %s.", secret.Name, owner.Name)
	} else {
		record(ctx, audit_model.UserSecretUpdate, doer, owner, secret, "Updated secret %s of user %s.", secret.Name, owner.Name)
	}
}

func RecordSecretRemove(ctx context.Context, doer, owner *user_model.User, repo *repository_model.Repository, secret *secret_model.Secret) {
	if owner == nil {
		record(ctx, audit_model.RepositorySecretRemove, doer, repo, secret, "Removed secret %s of repository %s.", secret.Name, repo.FullName())
	} else if owner.IsOrganization() {
		record(ctx, audit_model.OrganizationSecretRemove, doer, owner, secret, "Removed secret %s of organization %s.", secret.Name, owner.Name)
	} else {
		record(ctx, audit_model.UserSecretRemove, doer, owner, secret, "Removed secret %s of user %s.", secret.Name, owner.Name)
	}
}

func RecordWebhookAdd(ctx context.Context, doer, owner *user_model.User, repo *repository_model.Repository, hook *webhook_model.Webhook) {
	if owner == nil && repo == nil {
		record(ctx, audit_model.SystemWebhookAdd, doer, &systemObject, hook, "Added instance-wide webhook %s.", hook.URL)
	} else if repo != nil {
		record(ctx, audit_model.RepositoryWebhookAdd, doer, repo, hook, "Added webhook %s for repository %s.", hook.URL, repo.FullName())
	} else if owner.IsOrganization() {
		record(ctx, audit_model.OrganizationWebhookAdd, doer, owner, hook, "Added webhook %s for organization %s.", hook.URL, owner.Name)
	} else {
		record(ctx, audit_model.UserWebhookAdd, doer, owner, hook, "Added webhook %s for user %s.", hook.URL, owner.Name)
	}
}

func RecordWebhookUpdate(ctx context.Context, doer, owner *user_model.User, repo *repository_model.Repository, hook *webhook_model.Webhook) {
	if owner == nil && repo == nil {
		record(ctx, audit_model.SystemWebhookUpdate, doer, &systemObject, hook, "Updated instance-wide webhook %s.", hook.URL)
	} else if repo != nil {
		record(ctx, audit_model.RepositoryWebhookUpdate, doer, repo, hook, "Updated webhook %s of repository %s.", hook.URL, repo.FullName())
	} else if owner.IsOrganization() {
		record(ctx, audit_model.OrganizationWebhookUpdate, doer, owner, hook, "Updated webhook %s of organization %s.", hook.URL, owner.Name)
	} else {
		record(ctx, audit_model.UserWebhookUpdate, doer, owner, hook, "Updated webhook %s of user %s.", hook.URL, owner.Name)
	}
}

func RecordWebhookRemove(ctx context.Context, doer, owner *user_model.User, repo *repository_model.Repository, hook *webhook_model.Webhook) {
	if owner == nil && repo == nil {
		record(ctx, audit_model.SystemWebhookRemove, doer, &systemObject, hook, "Removed instance-wide webhook %s.", hook.URL)
	} else if repo != nil {
		record(ctx, audit_model.RepositoryWebhookRemove, doer, repo, hook, "Removed webhook %s of repository %s.", hook.URL, repo.FullName())
	} else if owner.IsOrganization() {
		record(ctx, audit_model.OrganizationWebhookRemove, doer, owner, hook, "Removed webhook %s of organization %s.", hook.URL, owner.Name)
	} else {
		record(ctx, audit_model.UserWebhookRemove, doer, owner, hook, "Removed webhook %s of user %s.", hook.URL, owner.Name)
	}
}

func RecordOrganizationTeamAdd(ctx context.Context, doer *user_model.User, org *organization_model.Organization, team *organization_model.Team) {
	record(ctx, audit_model.OrganizationTeamAdd, doer, org, team, "Added team %s to organization %s.", team.Name, org.Name)
}

func RecordOrganizationTeamUpdate(ctx context.Context, doer *user_model.User, org *organization_model.Organization, team *organization_model.Team) {
	record(ctx, audit_model.OrganizationTeamUpdate, doer, org, team, "Updated settings of team %s/%s.", org.Name, team.Name)
}

func RecordOrganizationTeamRemove(ctx context.Context, doer *user_model.User, org *organization_model.Organization, team *organization_model.Team) {
	record(ctx, audit_model.OrganizationTeamRemove, doer, org, team, "Removed team %s from organization %s.", team.Name, org.Name)
}

func RecordOrganizationTeamPermission(ctx context.Context, doer *user_model.User, org *organization_model.Organization, team *organization_model.Team) {
	record(ctx, audit_model.OrganizationTeamPermission, doer, org, team, "Changed permission of team %s/%s to %s.", org.Name, team.Name, team.AccessMode.ToString())
}

func RecordOrganizationTeamMemberAdd(ctx context.Context, doer *user_model.User, org *organization_model.Organization, team *organization_model.Team, member *user_model.User) {
	record(ctx, audit_model.OrganizationTeamMemberAdd, doer, org, team, "Added user %s to team %s/%s.", member.Name, org.Name, team.Name)
}

func RecordOrganizationTeamMemberRemove(ctx context.Context, doer *user_model.User, org *organization_model.Organization, team *organization_model.Team, member *user_model.User) {
	record(ctx, audit_model.OrganizationTeamMemberRemove, doer, org, team, "Removed user %s from team %s/%s.", member.Name, org.Name, team.Name)
}

func RecordRepositoryCreate(ctx context.Context, doer *user_model.User, repo *repository_model.Repository) {
	record(ctx, audit_model.RepositoryCreate, doer, repo, repo, "Created repository %s.", repo.FullName())
}

func RecordRepositoryCreateFork(ctx context.Context, doer *user_model.User, repo, baseRepo *repository_model.Repository) {
	record(ctx, audit_model.RepositoryCreateFork, doer, repo, repo, "Created fork %s of repository %s.", repo.FullName(), baseRepo.FullName())
}

func RecordRepositoryArchive(ctx context.Context, doer *user_model.User, repo *repository_model.Repository) {
	record(ctx, audit_model.RepositoryArchive, doer, repo, repo, "Archived repository %s.", repo.FullName())
}

func RecordRepositoryUnarchive(ctx context.Context, doer *user_model.User, repo *repository_model.Repository) {
	record(ctx, audit_model.RepositoryUnarchive, doer, repo, repo, "Unarchived repository %s.", repo.FullName())
}

func RecordRepositoryDelete(ctx context.Context, doer *user_model.User, repo *repository_model.Repository) {
	record(ctx, audit_model.RepositoryDelete, doer, repo, repo, "Deleted repository %s.", repo.FullName())
}

func RecordRepositoryName(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, previousName string) {
	record(ctx, audit_model.RepositoryName, doer, repo, repo, "Changed repository name from %s to %s.", previousName, repo.FullName())
}

func RecordRepositoryVisibility(ctx context.Context, doer *user_model.User, repo *repository_model.Repository) {
	status := "public"
	if repo.IsPrivate {
		status = "private"
	}

	record(ctx, audit_model.RepositoryVisibility, doer, repo, repo, "Changed visibility of repository %s to %s.", repo.FullName(), status)
}

func RecordRepositoryConvertFork(ctx context.Context, doer *user_model.User, repo *repository_model.Repository) {
	record(ctx, audit_model.RepositoryConvertFork, doer, repo, repo, "Converted repository %s from fork to regular repository.", repo.FullName())
}

func RecordRepositoryConvertMirror(ctx context.Context, doer *user_model.User, repo *repository_model.Repository) {
	record(ctx, audit_model.RepositoryConvertMirror, doer, repo, repo, "Converted repository %s from pull mirror to regular repository.", repo.FullName())
}

func RecordRepositoryMirrorPushAdd(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, mirror *repository_model.PushMirror) {
	record(ctx, audit_model.RepositoryMirrorPushAdd, doer, repo, mirror, "Added push mirror to %s for repository %s.", mirror.RemoteAddress, repo.FullName())
}

func RecordRepositoryMirrorPushRemove(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, mirror *repository_model.PushMirror) {
	record(ctx, audit_model.RepositoryMirrorPushRemove, doer, repo, mirror, "Removed push mirror to %s for repository %s.", mirror.RemoteAddress, repo.FullName())
}

func RecordRepositorySigningVerification(ctx context.Context, doer *user_model.User, repo *repository_model.Repository) {
	record(ctx, audit_model.RepositorySigningVerification, doer, repo, repo, "Changed signing verification of repository %s to %s.", repo.FullName(), repo.TrustModel.String())
}

func RecordRepositoryTransferStart(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, newOwner *user_model.User) {
	record(ctx, audit_model.RepositoryTransferStart, doer, repo, repo, "Started repository transfer of %s to %s.", repo.FullName(), newOwner.Name)
}

func RecordRepositoryTransferFinish(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, oldOwner *user_model.User) {
	record(ctx, audit_model.RepositoryTransferFinish, doer, repo, repo, "Transferred repository %s from %s to %s.", repo.FullName(), oldOwner.Name, repo.OwnerName)
}

func RecordRepositoryTransferCancel(ctx context.Context, doer *user_model.User, repo *repository_model.Repository) {
	record(ctx, audit_model.RepositoryTransferCancel, doer, repo, repo, "Canceled transfer of repository %s.", repo.FullName())
}

func RecordRepositoryWikiDelete(ctx context.Context, doer *user_model.User, repo *repository_model.Repository) {
	record(ctx, audit_model.RepositoryWikiDelete, doer, repo, repo, "Deleted wiki of repository %s.", repo.FullName())
}

func RecordRepositoryCollaboratorAdd(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, collaborator *user_model.User) {
	record(ctx, audit_model.RepositoryCollaboratorAdd, doer, repo, collaborator, "Added user %s as collaborator for repository %s.", collaborator.Name, repo.FullName())
}

func RecordRepositoryCollaboratorAccess(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, collaborator *user_model.User, accessMode perm_model.AccessMode) {
	record(ctx, audit_model.RepositoryCollaboratorAccess, doer, repo, collaborator, "Changed access mode of collaborator %s of repository %s to %s.", collaborator.Name, repo.FullName(), accessMode.ToString())
}

func RecordRepositoryCollaboratorRemove(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, collaborator *user_model.User) {
	record(ctx, audit_model.RepositoryCollaboratorRemove, doer, repo, collaborator, "Removed collaborator %s from repository %s.", collaborator.Name, repo.FullName())
}

func RecordRepositoryCollaboratorTeamAdd(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, team *organization_model.Team) {
	record(ctx, audit_model.RepositoryCollaboratorTeamAdd, doer, repo, team, "Added team %s as collaborator for repository %s.", team.Name, repo.FullName())
}

func RecordRepositoryCollaboratorTeamRemove(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, team *organization_model.Team) {
	record(ctx, audit_model.RepositoryCollaboratorTeamRemove, doer, repo, team, "Removed team %s as collaborator from repository %s.", team.Name, repo.FullName())
}

func RecordRepositoryBranchDefault(ctx context.Context, doer *user_model.User, repo *repository_model.Repository) {
	record(ctx, audit_model.RepositoryBranchDefault, doer, repo, repo, "Changed default branch of repository %s to %s.", repo.FullName(), repo.DefaultBranch)
}

func RecordRepositoryBranchProtectionAdd(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, protectBranch *git_model.ProtectedBranch) {
	record(ctx, audit_model.RepositoryBranchProtectionAdd, doer, repo, protectBranch, "Added branch protection %s for repository %s.", protectBranch.RuleName, repo.FullName())
}

func RecordRepositoryBranchProtectionUpdate(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, protectBranch *git_model.ProtectedBranch) {
	record(ctx, audit_model.RepositoryBranchProtectionUpdate, doer, repo, protectBranch, "Updated branch protection %s for repository %s.", protectBranch.RuleName, repo.FullName())
}

func RecordRepositoryBranchProtectionRemove(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, protectBranch *git_model.ProtectedBranch) {
	record(ctx, audit_model.RepositoryBranchProtectionRemove, doer, repo, protectBranch, "Removed branch protection %s from repository %s.", protectBranch.RuleName, repo.FullName())
}

func RecordRepositoryTagProtectionAdd(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, protectedTag *git_model.ProtectedTag) {
	record(ctx, audit_model.RepositoryTagProtectionAdd, doer, repo, protectedTag, "Added tag protection %s for repository %s.", protectedTag.NamePattern, repo.FullName())
}

func RecordRepositoryTagProtectionUpdate(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, protectedTag *git_model.ProtectedTag) {
	record(ctx, audit_model.RepositoryTagProtectionUpdate, doer, repo, protectedTag, "Updated tag protection %s for repository %s.", protectedTag.NamePattern, repo.FullName())
}

func RecordRepositoryTagProtectionRemove(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, protectedTag *git_model.ProtectedTag) {
	record(ctx, audit_model.RepositoryTagProtectionRemove, doer, repo, protectedTag, "Removed tag protection %s for repository %s.", protectedTag.NamePattern, repo.FullName())
}

func RecordRepositoryDeployKeyAdd(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, deployKey *asymkey_model.DeployKey) {
	record(ctx, audit_model.RepositoryDeployKeyAdd, doer, repo, deployKey, "Added deploy key %s for repository %s.", deployKey.Name, repo.FullName())
}

func RecordRepositoryDeployKeyRemove(ctx context.Context, doer *user_model.User, repo *repository_model.Repository, deployKey *asymkey_model.DeployKey) {
	record(ctx, audit_model.RepositoryDeployKeyRemove, doer, repo, deployKey, "Removed deploy key %s from repository %s.", deployKey.Name, repo.FullName())
}

func RecordSystemStartup(ctx context.Context, doer *user_model.User, version string) {
	// Do not change this message anymore. We guarantee the stability of this message for users wanting to parse the log themselves to be able to trace back events across gitea versions.
	record(ctx, audit_model.SystemStartup, doer, &systemObject, &systemObject, "System started [Gitea %s]", version)
}

func RecordSystemShutdown(ctx context.Context, doer *user_model.User) {
	record(ctx, audit_model.SystemShutdown, doer, &systemObject, &systemObject, "System shutdown")
}

func RecordSystemAuthenticationSourceAdd(ctx context.Context, doer *user_model.User, authSource *auth_model.Source) {
	record(ctx, audit_model.SystemAuthenticationSourceAdd, doer, &systemObject, authSource, "Created authentication source %s of type %s.", authSource.Name, authSource.Type.String())
}

func RecordSystemAuthenticationSourceUpdate(ctx context.Context, doer *user_model.User, authSource *auth_model.Source) {
	record(ctx, audit_model.SystemAuthenticationSourceUpdate, doer, &systemObject, authSource, "Updated authentication source %s.", authSource.Name)
}

func RecordSystemAuthenticationSourceRemove(ctx context.Context, doer *user_model.User, authSource *auth_model.Source) {
	record(ctx, audit_model.SystemAuthenticationSourceRemove, doer, &systemObject, authSource, "Removed authentication source %s.", authSource.Name)
}
