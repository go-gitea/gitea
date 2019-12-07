// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/structs"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/webhook"

	"github.com/unknwon/com"
)

// ToEmail convert models.EmailAddress to api.Email
func ToEmail(email *models.EmailAddress) *api.Email {
	return &api.Email{
		Email:    email.Email,
		Verified: email.IsActivated,
		Primary:  email.IsPrimary,
	}
}

// ToBranch convert a git.Commit and git.Branch to an api.Branch
func ToBranch(repo *models.Repository, b *git.Branch, c *git.Commit, bp *models.ProtectedBranch, user *models.User) *api.Branch {
	if bp == nil {
		return &api.Branch{
			Name:                b.Name,
			Commit:              ToCommit(repo, c),
			Protected:           false,
			RequiredApprovals:   0,
			EnableStatusCheck:   false,
			StatusCheckContexts: []string{},
			UserCanPush:         true,
			UserCanMerge:        true,
		}
	}
	return &api.Branch{
		Name:                b.Name,
		Commit:              ToCommit(repo, c),
		Protected:           true,
		RequiredApprovals:   bp.RequiredApprovals,
		EnableStatusCheck:   bp.EnableStatusCheck,
		StatusCheckContexts: bp.StatusCheckContexts,
		UserCanPush:         bp.CanUserPush(user.ID),
		UserCanMerge:        bp.CanUserMerge(user.ID),
	}
}

// ToTag convert a git.Tag to an api.Tag
func ToTag(repo *models.Repository, t *git.Tag) *api.Tag {
	return &api.Tag{
		Name:       t.Name,
		ID:         t.ID.String(),
		Commit:     ToCommitMeta(repo, t),
		ZipballURL: util.URLJoin(repo.HTMLURL(), "archive", t.Name+".zip"),
		TarballURL: util.URLJoin(repo.HTMLURL(), "archive", t.Name+".tar.gz"),
	}
}

// ToCommit convert a git.Commit to api.PayloadCommit
func ToCommit(repo *models.Repository, c *git.Commit) *api.PayloadCommit {
	authorUsername := ""
	if author, err := models.GetUserByEmail(c.Author.Email); err == nil {
		authorUsername = author.Name
	} else if !models.IsErrUserNotExist(err) {
		log.Error("GetUserByEmail: %v", err)
	}

	committerUsername := ""
	if committer, err := models.GetUserByEmail(c.Committer.Email); err == nil {
		committerUsername = committer.Name
	} else if !models.IsErrUserNotExist(err) {
		log.Error("GetUserByEmail: %v", err)
	}

	return &api.PayloadCommit{
		ID:      c.ID.String(),
		Message: c.Message(),
		URL:     util.URLJoin(repo.HTMLURL(), "commit", c.ID.String()),
		Author: &api.PayloadUser{
			Name:     c.Author.Name,
			Email:    c.Author.Email,
			UserName: authorUsername,
		},
		Committer: &api.PayloadUser{
			Name:     c.Committer.Name,
			Email:    c.Committer.Email,
			UserName: committerUsername,
		},
		Timestamp:    c.Author.When,
		Verification: ToVerification(c),
	}
}

// ToVerification convert a git.Commit.Signature to an api.PayloadCommitVerification
func ToVerification(c *git.Commit) *api.PayloadCommitVerification {
	verif := models.ParseCommitWithSignature(c)
	commitVerification := &api.PayloadCommitVerification{
		Verified: verif.Verified,
		Reason:   verif.Reason,
	}
	if c.Signature != nil {
		commitVerification.Signature = c.Signature.Signature
		commitVerification.Payload = c.Signature.Payload
	}
	if verif.SigningUser != nil {
		commitVerification.Signer = &structs.PayloadUser{
			Name:  verif.SigningUser.Name,
			Email: verif.SigningUser.Email,
		}
	}
	return commitVerification
}

// ToPublicKey convert models.PublicKey to api.PublicKey
func ToPublicKey(apiLink string, key *models.PublicKey) *api.PublicKey {
	return &api.PublicKey{
		ID:          key.ID,
		Key:         key.Content,
		URL:         apiLink + com.ToStr(key.ID),
		Title:       key.Name,
		Fingerprint: key.Fingerprint,
		Created:     key.CreatedUnix.AsTime(),
	}
}

// ToGPGKey converts models.GPGKey to api.GPGKey
func ToGPGKey(key *models.GPGKey) *api.GPGKey {
	subkeys := make([]*api.GPGKey, len(key.SubsKey))
	for id, k := range key.SubsKey {
		subkeys[id] = &api.GPGKey{
			ID:                k.ID,
			PrimaryKeyID:      k.PrimaryKeyID,
			KeyID:             k.KeyID,
			PublicKey:         k.Content,
			Created:           k.CreatedUnix.AsTime(),
			Expires:           k.ExpiredUnix.AsTime(),
			CanSign:           k.CanSign,
			CanEncryptComms:   k.CanEncryptComms,
			CanEncryptStorage: k.CanEncryptStorage,
			CanCertify:        k.CanSign,
		}
	}
	emails := make([]*api.GPGKeyEmail, len(key.Emails))
	for i, e := range key.Emails {
		emails[i] = ToGPGKeyEmail(e)
	}
	return &api.GPGKey{
		ID:                key.ID,
		PrimaryKeyID:      key.PrimaryKeyID,
		KeyID:             key.KeyID,
		PublicKey:         key.Content,
		Created:           key.CreatedUnix.AsTime(),
		Expires:           key.ExpiredUnix.AsTime(),
		Emails:            emails,
		SubsKey:           subkeys,
		CanSign:           key.CanSign,
		CanEncryptComms:   key.CanEncryptComms,
		CanEncryptStorage: key.CanEncryptStorage,
		CanCertify:        key.CanSign,
	}
}

// ToGPGKeyEmail convert models.EmailAddress to api.GPGKeyEmail
func ToGPGKeyEmail(email *models.EmailAddress) *api.GPGKeyEmail {
	return &api.GPGKeyEmail{
		Email:    email.Email,
		Verified: email.IsActivated,
	}
}

// ToHook convert models.Webhook to api.Hook
func ToHook(repoLink string, w *models.Webhook) *api.Hook {
	config := map[string]string{
		"url":          w.URL,
		"content_type": w.ContentType.Name(),
	}
	if w.HookTaskType == models.SLACK {
		s := webhook.GetSlackHook(w)
		config["channel"] = s.Channel
		config["username"] = s.Username
		config["icon_url"] = s.IconURL
		config["color"] = s.Color
	}

	return &api.Hook{
		ID:      w.ID,
		Type:    w.HookTaskType.Name(),
		URL:     fmt.Sprintf("%s/settings/hooks/%d", repoLink, w.ID),
		Active:  w.IsActive,
		Config:  config,
		Events:  w.EventsArray(),
		Updated: w.UpdatedUnix.AsTime(),
		Created: w.CreatedUnix.AsTime(),
	}
}

// ToGitHook convert git.Hook to api.GitHook
func ToGitHook(h *git.Hook) *api.GitHook {
	return &api.GitHook{
		Name:     h.Name(),
		IsActive: h.IsActive,
		Content:  h.Content,
	}
}

// ToDeployKey convert models.DeployKey to api.DeployKey
func ToDeployKey(apiLink string, key *models.DeployKey) *api.DeployKey {
	return &api.DeployKey{
		ID:          key.ID,
		KeyID:       key.KeyID,
		Key:         key.Content,
		Fingerprint: key.Fingerprint,
		URL:         apiLink + com.ToStr(key.ID),
		Title:       key.Name,
		Created:     key.CreatedUnix.AsTime(),
		ReadOnly:    key.Mode == models.AccessModeRead, // All deploy keys are read-only.
	}
}

// ToOrganization convert models.User to api.Organization
func ToOrganization(org *models.User) *api.Organization {
	return &api.Organization{
		ID:                        org.ID,
		AvatarURL:                 org.AvatarLink(),
		UserName:                  org.Name,
		FullName:                  org.FullName,
		Description:               org.Description,
		Website:                   org.Website,
		Location:                  org.Location,
		Visibility:                org.Visibility.String(),
		RepoAdminChangeTeamAccess: org.RepoAdminChangeTeamAccess,
	}
}

// ToTeam convert models.Team to api.Team
func ToTeam(team *models.Team) *api.Team {
	return &api.Team{
		ID:                      team.ID,
		Name:                    team.Name,
		Description:             team.Description,
		IncludesAllRepositories: team.IncludesAllRepositories,
		CanCreateOrgRepo:        team.CanCreateOrgRepo,
		Permission:              team.Authorize.String(),
		Units:                   team.GetUnitNames(),
	}
}

// ToUser convert models.User to api.User
// signed shall only be set if requester is logged in. authed shall only be set if user is site admin or user himself
func ToUser(user *models.User, signed, authed bool) *api.User {
	result := &api.User{
		UserName:  user.Name,
		AvatarURL: user.AvatarLink(),
		FullName:  markup.Sanitize(user.FullName),
		Created:   user.CreatedUnix.AsTime(),
	}
	// hide primary email if API caller is anonymous or user keep email private
	if signed && (!user.KeepEmailPrivate || authed) {
		result.Email = user.Email
	}
	// only site admin will get these information and possibly user himself
	if authed {
		result.ID = user.ID
		result.IsAdmin = user.IsAdmin
		result.LastLogin = user.LastLoginUnix.AsTime()
		result.Language = user.Language
	}
	return result
}

// ToAnnotatedTag convert git.Tag to api.AnnotatedTag
func ToAnnotatedTag(repo *models.Repository, t *git.Tag, c *git.Commit) *api.AnnotatedTag {
	return &api.AnnotatedTag{
		Tag:          t.Name,
		SHA:          t.ID.String(),
		Object:       ToAnnotatedTagObject(repo, c),
		Message:      t.Message,
		URL:          util.URLJoin(repo.APIURL(), "git/tags", t.ID.String()),
		Tagger:       ToCommitUser(t.Tagger),
		Verification: ToVerification(c),
	}
}

// ToAnnotatedTagObject convert a git.Commit to an api.AnnotatedTagObject
func ToAnnotatedTagObject(repo *models.Repository, commit *git.Commit) *api.AnnotatedTagObject {
	return &api.AnnotatedTagObject{
		SHA:  commit.ID.String(),
		Type: string(git.ObjectCommit),
		URL:  util.URLJoin(repo.APIURL(), "git/commits", commit.ID.String()),
	}
}

// ToCommitUser convert a git.Signature to an api.CommitUser
func ToCommitUser(sig *git.Signature) *api.CommitUser {
	return &api.CommitUser{
		Identity: api.Identity{
			Name:  sig.Name,
			Email: sig.Email,
		},
		Date: sig.When.UTC().Format(time.RFC3339),
	}
}

// ToCommitMeta convert a git.Tag to an api.CommitMeta
func ToCommitMeta(repo *models.Repository, tag *git.Tag) *api.CommitMeta {
	return &api.CommitMeta{
		SHA: tag.Object.String(),
		// TODO: Add the /commits API endpoint and use it here (https://developer.github.com/v3/repos/commits/#get-a-single-commit)
		URL: util.URLJoin(repo.APIURL(), "git/commits", tag.ID.String()),
	}
}

// ToTopicResponse convert from models.Topic to api.TopicResponse
func ToTopicResponse(topic *models.Topic) *api.TopicResponse {
	return &api.TopicResponse{
		ID:        topic.ID,
		Name:      topic.Name,
		RepoCount: topic.RepoCount,
		Created:   topic.CreatedUnix.AsTime(),
		Updated:   topic.UpdatedUnix.AsTime(),
	}
}
