// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"time"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/translation"
	"gitea.dev/modules/util"
	sender_service "gitea.dev/services/mailer/sender"
)

const tplSecurityAdvisory templates.TplName = "mail/repo/security_advisory"

// generateMessageIDForAdvisory without a suffix is the ID of the report mail, which the other mails reply to
func generateMessageIDForAdvisory(a *advisory_model.Advisory, suffix string) string {
	return fmt.Sprintf("<%s/security/advisories/%s%s@%s>", a.Repo.FullName(), a.Identifier, suffix, setting.AppDomain)
}

// advisoryAdminIDs are the repository admins and the members of its security teams
func advisoryAdminIDs(ctx context.Context, a *advisory_model.Advisory) (container.Set[int64], error) {
	ids, err := access_model.GetUserIDsWithAnyUnitAccess(ctx, a.Repo, perm.AccessModeAdmin, unit.TypeCode)
	if err != nil {
		return nil, err
	}
	memberIDs, err := organization.GetSecurityTeamMemberIDs(ctx, a.RepoID)
	if err != nil {
		return nil, err
	}
	ids.AddMultiple(memberIDs...)
	return ids, nil
}

// MailSecurityAdvisoryReported notifies the repository admins about a private vulnerability report
func MailSecurityAdvisoryReported(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory) {
	if setting.MailService == nil || !loadAdvisoryRepo(ctx, a) {
		return
	}
	adminIDs, err := advisoryAdminIDs(ctx, a)
	if err != nil {
		log.Error("advisoryAdminIDs(%d): %v", a.ID, err)
		return
	}
	recipients := advisoryMailRecipients(ctx, doer, a, adminIDs, true, canSeeDiscussion)
	sendAdvisoryMail(doer, a, recipients, "mail.security_advisory.reported", "")
}

// MailSecurityAdvisoryComment notifies the participants and repository admins about a new comment
func MailSecurityAdvisoryComment(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, c *advisory_model.Comment) {
	if setting.MailService == nil || !loadAdvisoryRepo(ctx, a) {
		return
	}
	ids, err := advisory_model.ParticipantIDs(ctx, a)
	if err != nil {
		log.Error("ParticipantIDs(%d): %v", a.ID, err)
		return
	}
	adminIDs, err := advisoryAdminIDs(ctx, a)
	if err != nil {
		log.Error("advisoryAdminIDs(%d): %v", a.ID, err)
		return
	}
	ids.AddMultiple(adminIDs.Values()...)
	recipients := advisoryMailRecipients(ctx, doer, a, ids, false, func(p advisory_model.Permissions) bool {
		return util.Iif(c.IsInternal, p.CanSeeInternal, p.CanSeeDiscussion)
	})
	sendAdvisoryMail(doer, a, recipients, "mail.security_advisory.comment", fmt.Sprintf("/comments/%d", c.ID))
}

// MailSecurityAdvisoryPublished notifies the participants and credited users about a published advisory
func MailSecurityAdvisoryPublished(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory) {
	if setting.MailService == nil {
		return
	}
	if err := a.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes(%d): %v", a.ID, err)
		return
	}
	ids, err := advisory_model.ParticipantIDs(ctx, a)
	if err != nil {
		log.Error("ParticipantIDs(%d): %v", a.ID, err)
		return
	}
	for _, c := range a.Credits {
		ids.Add(c.UserID)
	}
	recipients := advisoryMailRecipients(ctx, doer, a, ids, false, func(p advisory_model.Permissions) bool { return p.CanView })
	sendAdvisoryMail(doer, a, recipients, "mail.security_advisory.published", "/published")
}

// MailSecurityAdvisoryCollaboratorAdded notifies a user or the members of a team granted access to an advisory
func MailSecurityAdvisoryCollaboratorAdded(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, u *user_model.User, team *organization.Team) {
	if setting.MailService == nil || !loadAdvisoryRepo(ctx, a) {
		return
	}
	ids := make(container.Set[int64])
	if u != nil {
		ids.Add(u.ID)
	} else {
		if err := team.LoadMembers(ctx); err != nil {
			log.Error("LoadMembers(%d): %v", team.ID, err)
			return
		}
		ids.AddMultiple(user_model.UserList(team.Members).GetUserIDs()...)
	}
	recipients := advisoryMailRecipients(ctx, doer, a, ids, true, canSeeDiscussion)
	sendAdvisoryMail(doer, a, recipients, "mail.security_advisory.invited", fmt.Sprintf("/invited/%d", time.Now().UnixNano()))
}

func loadAdvisoryRepo(ctx context.Context, a *advisory_model.Advisory) bool {
	if err := a.LoadRepo(ctx); err != nil {
		log.Error("LoadRepo(%d): %v", a.ID, err)
		return false
	}
	return true
}

func canSeeDiscussion(p advisory_model.Permissions) bool {
	return p.CanSeeDiscussion
}

// advisoryMailRecipients returns the mailable users without the doer, isDirect mails concern the recipients directly like mentions,
// canRead is checked again for each mail because users can lose access after they have been added
func advisoryMailRecipients(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, ids container.Set[int64], isDirect bool, canRead func(advisory_model.Permissions) bool) []*user_model.User {
	recipients, err := user_model.GetMailableUsersByIDs(ctx, ids.Values(), isDirect)
	if err != nil {
		log.Error("GetMailableUsersByIDs: %v", err)
		return nil
	}
	return slices.DeleteFunc(recipients, func(u *user_model.User) bool {
		if u.ID == doer.ID {
			return true
		}
		perms, err := advisory_model.UserPermissions(ctx, u, a)
		if err != nil {
			log.Error("UserPermissions(%d, %d): %v", u.ID, a.ID, err)
		}
		return !canRead(perms)
	})
}

// sendAdvisoryMail only notifies, mails are not end-to-end encrypted, the mail with an empty threadSuffix starts the thread
func sendAdvisoryMail(doer *user_model.User, a *advisory_model.Advisory, recipients []*user_model.User, localeKey, threadSuffix string) {
	if len(recipients) == 0 {
		return
	}

	langMap := make(map[string][]*user_model.User)
	for _, u := range recipients {
		langMap[u.Language] = append(langMap[u.Language], u)
	}
	for lang, tos := range langMap {
		mailAdvisoryToLang(doer, a, localeKey, threadSuffix, lang, tos)
	}
}

func mailAdvisoryToLang(doer *user_model.User, a *advisory_model.Advisory, localeKey, threadSuffix, lang string, tos []*user_model.User) {
	locale := translation.NewLocale(lang)
	title := fmt.Sprintf("#%d", a.Index) // the summary of a non-public advisory can reveal the vulnerability
	if a.IsPublic() {
		title = fmt.Sprintf("%s (#%d)", a.Summary, a.Index)
	}
	subject := locale.TrString(localeKey+".subject", a.Repo.FullName(), title)
	mailMeta := map[string]any{
		"locale":   locale,
		"Subject":  subject,
		"TextKey":  localeKey + ".text",
		"Language": locale.Language(),
		"DoerName": doer.Name,
		"Title":    title,
		"Link":     a.HTMLURL(),
		"RepoName": a.Repo.FullName(),
		"RepoLink": a.Repo.HTMLURL(),
	}

	var mailBody bytes.Buffer
	if err := LoadedTemplates().BodyTemplates.ExecuteTemplate(&mailBody, string(tplSecurityAdvisory), mailMeta); err != nil {
		log.Error("ExecuteTemplate [%s]: %v", tplSecurityAdvisory, err)
		return
	}

	threadID, messageID := generateMessageIDForAdvisory(a, ""), generateMessageIDForAdvisory(a, threadSuffix)
	msgs := make([]*sender_service.Message, 0, len(tos))
	for _, to := range tos {
		msg := sender_service.NewMessageFrom(to.EmailTo(), fromDisplayName(doer), setting.MailService.FromEmail, subject, mailBody.String())
		msg.Info = subject
		msg.SetHeader("Message-ID", messageID)
		if messageID != threadID {
			msg.SetHeader("In-Reply-To", threadID)
			msg.SetHeader("References", threadID)
		}
		msgs = append(msgs, msg)
	}
	SendAsync(msgs...)
}
