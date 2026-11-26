// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"fmt"
	"maps"
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
func generateMessageIDForAdvisory(advisory *advisory_model.Advisory, suffix string) string {
	return fmt.Sprintf("<%s/security/advisories/%s%s@%s>", advisory.Repo.FullName(), advisory.Identifier, suffix, setting.AppDomain)
}

func advisoryAdminIDs(ctx context.Context, advisory *advisory_model.Advisory) (container.Set[int64], error) {
	ids, err := access_model.GetUserIDsWithAnyUnitAccess(ctx, advisory.Repo, perm.AccessModeAdmin, unit.TypeCode)
	if err != nil {
		return nil, err
	}
	memberIDs, err := organization.GetSecurityTeamMemberIDs(ctx, advisory.RepoID)
	if err != nil {
		return nil, err
	}
	ids.AddMultiple(memberIDs...)
	return ids, nil
}

func MailSecurityAdvisoryReported(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory) {
	if setting.MailService == nil || !loadAdvisoryRepo(ctx, advisory) {
		return
	}
	adminIDs, err := advisoryAdminIDs(ctx, advisory)
	if err != nil {
		log.Error("advisoryAdminIDs(%d): %v", advisory.ID, err)
		return
	}
	recipients := advisoryMailRecipients(ctx, doer, advisory, adminIDs, true, canSeeDiscussion)
	sendAdvisoryMail(doer, advisory, recipients, "mail.security_advisory.reported", "")
}

func MailSecurityAdvisoryComment(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, c *advisory_model.Comment) {
	if setting.MailService == nil || !loadAdvisoryRepo(ctx, advisory) {
		return
	}
	ids, err := advisory_model.ParticipantIDs(ctx, advisory)
	if err != nil {
		log.Error("ParticipantIDs(%d): %v", advisory.ID, err)
		return
	}
	adminIDs, err := advisoryAdminIDs(ctx, advisory)
	if err != nil {
		log.Error("advisoryAdminIDs(%d): %v", advisory.ID, err)
		return
	}
	maps.Copy(ids, adminIDs)
	recipients := advisoryMailRecipients(ctx, doer, advisory, ids, false, func(perms advisory_model.Permissions) bool {
		return util.Iif(c.IsConfidential, perms.CanSeeConfidential, perms.CanSeeDiscussion)
	})
	sendAdvisoryMail(doer, advisory, recipients, "mail.security_advisory.comment", fmt.Sprintf("/comments/%d", c.ID))
}

func MailSecurityAdvisoryPublished(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory) {
	if setting.MailService == nil {
		return
	}
	if err := advisory.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes(%d): %v", advisory.ID, err)
		return
	}
	ids, err := advisory_model.ParticipantIDs(ctx, advisory)
	if err != nil {
		log.Error("ParticipantIDs(%d): %v", advisory.ID, err)
		return
	}
	for _, credit := range advisory.Credits {
		ids.Add(credit.UserID)
	}
	recipients := advisoryMailRecipients(ctx, doer, advisory, ids, false, func(perms advisory_model.Permissions) bool { return perms.CanView })
	sendAdvisoryMail(doer, advisory, recipients, "mail.security_advisory.published", "/published")
}

func MailSecurityAdvisoryCollaboratorAdded(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, user *user_model.User, team *organization.Team) {
	if setting.MailService == nil || !loadAdvisoryRepo(ctx, advisory) {
		return
	}
	ids := make(container.Set[int64])
	if user != nil {
		ids.Add(user.ID)
	} else {
		if err := team.LoadMembers(ctx); err != nil {
			log.Error("LoadMembers(%d): %v", team.ID, err)
			return
		}
		ids.AddMultiple(user_model.UserList(team.Members).GetUserIDs()...)
	}
	recipients := advisoryMailRecipients(ctx, doer, advisory, ids, true, canSeeDiscussion)
	sendAdvisoryMail(doer, advisory, recipients, "mail.security_advisory.invited", fmt.Sprintf("/invited/%d", time.Now().UnixNano()))
}

func loadAdvisoryRepo(ctx context.Context, advisory *advisory_model.Advisory) bool {
	if err := advisory.LoadRepo(ctx); err != nil {
		log.Error("LoadRepo(%d): %v", advisory.ID, err)
		return false
	}
	return true
}

func canSeeDiscussion(perms advisory_model.Permissions) bool {
	return perms.CanSeeDiscussion
}

// advisoryMailRecipients rechecks access because recipients may have lost it since being added.
func advisoryMailRecipients(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory, ids container.Set[int64], isDirect bool, canRead func(advisory_model.Permissions) bool) []*user_model.User {
	recipients, err := user_model.GetMailableUsersByIDs(ctx, ids.Values(), isDirect)
	if err != nil {
		log.Error("GetMailableUsersByIDs: %v", err)
		return nil
	}
	return slices.DeleteFunc(recipients, func(user *user_model.User) bool {
		if user.ID == doer.ID {
			return true
		}
		perms, err := advisory_model.UserPermissions(ctx, user, advisory)
		if err != nil {
			log.Error("UserPermissions(%d, %d): %v", user.ID, advisory.ID, err)
		}
		return !canRead(perms)
	})
}

func sendAdvisoryMail(doer *user_model.User, advisory *advisory_model.Advisory, recipients []*user_model.User, localeKey, threadSuffix string) {
	langMap := make(map[string][]*user_model.User)
	for _, user := range recipients {
		langMap[user.Language] = append(langMap[user.Language], user)
	}
	for lang, tos := range langMap {
		mailAdvisoryToLang(doer, advisory, localeKey, threadSuffix, lang, tos)
	}
}

func mailAdvisoryToLang(doer *user_model.User, advisory *advisory_model.Advisory, localeKey, threadSuffix, lang string, tos []*user_model.User) {
	locale := translation.NewLocale(lang)
	title := fmt.Sprintf("#%d", advisory.Index) // the summary of a non-public advisory can reveal the vulnerability
	if advisory.IsPublic() {
		title = fmt.Sprintf("%s (#%d)", advisory.Summary, advisory.Index)
	}
	subject := locale.TrString(localeKey+".subject", advisory.Repo.FullName(), title)
	mailMeta := map[string]any{
		"locale":   locale,
		"Subject":  subject,
		"TextKey":  localeKey + ".text",
		"Language": locale.Language(),
		"DoerName": doer.Name,
		"Title":    title,
		"Link":     advisory.HTMLURL(),
		"RepoName": advisory.Repo.FullName(),
		"RepoLink": advisory.Repo.HTMLURL(),
	}

	var mailBody bytes.Buffer
	if err := LoadedTemplates().BodyTemplates.ExecuteTemplate(&mailBody, string(tplSecurityAdvisory), mailMeta); err != nil {
		log.Error("ExecuteTemplate [%s]: %v", tplSecurityAdvisory, err)
		return
	}

	threadID, messageID := generateMessageIDForAdvisory(advisory, ""), generateMessageIDForAdvisory(advisory, threadSuffix)
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
