// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"fmt"

	"github.com/Unknwon/com"

	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/util"
)

// ToEmail convert models.EmailAddress to api.Email
func ToEmail(email *models.EmailAddress) *api.Email {
	return &api.Email{
		Email:    email.Email,
		Verified: email.IsActivated,
		Primary:  email.IsPrimary,
	}
}

// ToBranch convert a commit and branch to an api.Branch
func ToBranch(repo *models.Repository, b *models.Branch, c *git.Commit) *api.Branch {
	return &api.Branch{
		Name:   b.Name,
		Commit: ToCommit(repo, c),
	}
}

// ToTag convert a tag to an api.Tag
func ToTag(repo *models.Repository, t *git.Tag) *api.Tag {
	return &api.Tag{
		Name: t.Name,
		Commit: struct {
			SHA string `json:"sha"`
			URL string `json:"url"`
		}{
			SHA: t.ID.String(),
			URL: util.URLJoin(repo.Link(), "commit", t.ID.String()),
		},
		ZipballURL: util.URLJoin(repo.Link(), "archive", t.Name+".zip"),
		TarballURL: util.URLJoin(repo.Link(), "archive", t.Name+".tar.gz"),
	}
}

// ToCommit convert a commit to api.PayloadCommit
func ToCommit(repo *models.Repository, c *git.Commit) *api.PayloadCommit {
	authorUsername := ""
	if author, err := models.GetUserByEmail(c.Author.Email); err == nil {
		authorUsername = author.Name
	} else if !models.IsErrUserNotExist(err) {
		log.Error(4, "GetUserByEmail: %v", err)
	}

	committerUsername := ""
	if committer, err := models.GetUserByEmail(c.Committer.Email); err == nil {
		committerUsername = committer.Name
	} else if !models.IsErrUserNotExist(err) {
		log.Error(4, "GetUserByEmail: %v", err)
	}

	verif := models.ParseCommitWithSignature(c)
	var signature, payload string
	if c.Signature != nil {
		signature = c.Signature.Signature
		payload = c.Signature.Payload
	}

	return &api.PayloadCommit{
		ID:      c.ID.String(),
		Message: c.Message(),
		URL:     util.URLJoin(repo.Link(), "commit", c.ID.String()),
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
		Timestamp: c.Author.When,
		Verification: &api.PayloadCommitVerification{
			Verified:  verif.Verified,
			Reason:    verif.Reason,
			Signature: signature,
			Payload:   payload,
		},
	}
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
		s := w.GetSlackHook()
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
		ID:          org.ID,
		AvatarURL:   org.AvatarLink(),
		UserName:    org.Name,
		FullName:    org.FullName,
		Description: org.Description,
		Website:     org.Website,
		Location:    org.Location,
	}
}

// ToTeam convert models.Team to api.Team
func ToTeam(team *models.Team) *api.Team {
	return &api.Team{
		ID:          team.ID,
		Name:        team.Name,
		Description: team.Description,
		Permission:  team.Authorize.String(),
		Units:       team.GetUnitNames(),
	}
}

// ToUser convert models.User to api.User
func ToUser(user *models.User, signed, admin bool) *api.User {
	result := &api.User{
		ID:        user.ID,
		UserName:  user.Name,
		AvatarURL: user.AvatarLink(),
		FullName:  markup.Sanitize(user.FullName),
		IsAdmin:   user.IsAdmin,
	}
	if signed && (!user.KeepEmailPrivate || admin) {
		result.Email = user.Email
	}
	return result
}
