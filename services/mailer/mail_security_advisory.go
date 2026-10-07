// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"slices"
	"time"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	"gitea.dev/models/renderhelper"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/log"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/translation"
	sender_service "gitea.dev/services/mailer/sender"
)

const tplSecurityAdvisory templates.TplName = "mail/repo/security_advisory"

// generateMessageIDForAdvisory returns the ID of the report mail, the other mails reply to it
func generateMessageIDForAdvisory(a *advisory_model.Advisory, suffix string) string {
	return fmt.Sprintf("<%s/security/advisories/%s%s@%s>", a.Repo.FullName(), a.Identifier, suffix, setting.AppDomain)
}

type advisoryMail struct {
	doer     *user_model.User
	advisory *advisory_model.Advisory
	kind     string // reported, comment, published or invited
	content  string // markdown rendered into the mail body
	idSuffix string // empty for the report mail
	direct   bool   // the mail concerns the recipient directly like a mention
	public   bool   // the recipients only need to be able to read published advisories
}

func advisoryAdminIDs(ctx context.Context, a *advisory_model.Advisory) (container.Set[int64], error) {
	return access_model.GetUserIDsWithAnyUnitAccess(ctx, a.Repo, perm.AccessModeAdmin, unit.TypeCode)
}

// MailSecurityAdvisoryReported notifies the repository admins about a private vulnerability report
func MailSecurityAdvisoryReported(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory) {
	sendAdvisoryMail(ctx, &advisoryMail{doer: doer, advisory: a, kind: "reported", content: a.Description, direct: true}, func() (container.Set[int64], error) {
		return advisoryAdminIDs(ctx, a)
	})
}

// MailSecurityAdvisoryComment notifies the participants and repository admins about a new comment
func MailSecurityAdvisoryComment(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, c *advisory_model.Comment) {
	m := &advisoryMail{doer: doer, advisory: a, kind: "comment", content: c.Content, idSuffix: fmt.Sprintf("/comments/%d", c.ID)}
	sendAdvisoryMail(ctx, m, func() (container.Set[int64], error) {
		ids, err := advisory_model.ParticipantIDs(ctx, a)
		if err != nil {
			return nil, err
		}
		adminIDs, err := advisoryAdminIDs(ctx, a)
		ids.AddMultiple(adminIDs.Values()...)
		return ids, err
	})
}

// MailSecurityAdvisoryPublished notifies the participants and credited users about a published advisory
func MailSecurityAdvisoryPublished(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory) {
	sendAdvisoryMail(ctx, &advisoryMail{doer: doer, advisory: a, kind: "published", idSuffix: "/published", public: true}, func() (container.Set[int64], error) {
		if err := a.LoadAttributes(ctx); err != nil {
			return nil, err
		}
		ids, err := advisory_model.ParticipantIDs(ctx, a)
		if err != nil {
			return nil, err
		}
		for _, c := range a.Credits {
			ids.Add(c.UserID)
		}
		return ids, nil
	})
}

// MailSecurityAdvisoryCollaboratorAdded notifies a user or the members of a team granted access to an advisory
func MailSecurityAdvisoryCollaboratorAdded(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory, u *user_model.User, team *organization.Team) {
	m := &advisoryMail{doer: doer, advisory: a, kind: "invited", idSuffix: fmt.Sprintf("/invited/%d", time.Now().UnixNano()), direct: true}
	sendAdvisoryMail(ctx, m, func() (container.Set[int64], error) {
		if u != nil {
			return container.SetOf(u.ID), nil
		}
		if err := team.LoadMembers(ctx); err != nil {
			return nil, err
		}
		return container.SetOf(user_model.UserList(team.Members).GetUserIDs()...), nil
	})
}

func sendAdvisoryMail(ctx context.Context, m *advisoryMail, recipientIDs func() (container.Set[int64], error)) {
	if setting.MailService == nil {
		return
	}
	a := m.advisory
	if err := a.LoadRepo(ctx); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}
	ids, err := recipientIDs()
	if err != nil {
		log.Error("security advisory %d %s mail recipients: %v", a.ID, m.kind, err)
		return
	}
	recipients, err := user_model.GetMailableUsersByIDs(ctx, ids.Values(), m.direct)
	if err != nil {
		log.Error("GetMailableUsersByIDs: %v", err)
		return
	}
	// access is checked again because users can lose it after they have been added
	recipients = slices.DeleteFunc(recipients, func(u *user_model.User) bool {
		if u.ID == m.doer.ID {
			return true
		}
		if m.public {
			return !access_model.CheckRepoUnitUser(ctx, a.Repo, u, unit.TypeSecurityAdvisories)
		}
		ok, err := advisory_model.UserCanSeeDiscussion(ctx, u, a)
		if err != nil {
			log.Error("UserCanSeeDiscussion(%d, %d): %v", u.ID, a.ID, err)
		}
		return !ok
	})
	if len(recipients) == 0 {
		return
	}

	var content template.HTML
	if m.content != "" {
		rctx := renderhelper.NewRenderContextRepoComment(ctx, a.Repo).WithUseAbsoluteLink(true)
		if content, err = markdown.RenderString(rctx, m.content); err != nil {
			log.Error("markdown.RenderString(%d): %v", a.RepoID, err)
			return
		}
	}

	langMap := make(map[string][]*user_model.User)
	for _, u := range recipients {
		langMap[u.Language] = append(langMap[u.Language], u)
	}
	for lang, tos := range langMap {
		mailAdvisoryToLang(m, content, lang, tos)
	}
}

func mailAdvisoryToLang(m *advisoryMail, content template.HTML, lang string, tos []*user_model.User) {
	a := m.advisory
	locale := translation.NewLocale(lang)
	subject := locale.TrString("mail.security_advisory."+m.kind+".subject", a.Repo.FullName(), a.Summary)
	mailMeta := map[string]any{
		"locale":   locale,
		"Subject":  subject,
		"TextKey":  "mail.security_advisory." + m.kind + ".text",
		"Language": locale.Language(),
		"DoerName": m.doer.Name,
		"Summary":  a.Summary,
		"Content":  content,
		"Link":     a.HTMLURL(),
		"RepoName": a.Repo.FullName(),
		"RepoLink": a.Repo.HTMLURL(),
	}

	var mailBody bytes.Buffer
	if err := LoadedTemplates().BodyTemplates.ExecuteTemplate(&mailBody, string(tplSecurityAdvisory), mailMeta); err != nil {
		log.Error("ExecuteTemplate [%s]: %v", tplSecurityAdvisory, err)
		return
	}

	rootID, messageID := generateMessageIDForAdvisory(a, ""), generateMessageIDForAdvisory(a, m.idSuffix)
	msgs := make([]*sender_service.Message, 0, len(tos))
	for _, to := range tos {
		msg := sender_service.NewMessageFrom(to.EmailTo(), fromDisplayName(m.doer), setting.MailService.FromEmail, subject, mailBody.String())
		msg.Info = subject
		msg.SetHeader("Message-ID", messageID)
		if m.idSuffix != "" {
			msg.SetHeader("In-Reply-To", rootID)
			msg.SetHeader("References", rootID)
		}
		msgs = append(msgs, msg)
	}
	SendAsync(msgs...)
}
