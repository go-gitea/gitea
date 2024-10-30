// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/references"
	api "code.gitea.io/gitea/modules/structs"

	"xorm.io/builder"
)

// UpdateConversationCols updates cols of conversation
func UpdateConversationCols(ctx context.Context, conversation *Conversation, cols ...string) error {
	if _, err := db.GetEngine(ctx).ID(conversation.ID).Cols(cols...).Update(conversation); err != nil {
		return err
	}
	return nil
}

// UpdateConversationAttachments update attachments by UUIDs for the conversation
func UpdateConversationAttachments(ctx context.Context, conversationID int64, uuids []string) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	attachments, err := repo_model.GetAttachmentsByUUIDs(ctx, uuids)
	if err != nil {
		return fmt.Errorf("getAttachmentsByUUIDs [uuids: %v]: %w", uuids, err)
	}
	for i := 0; i < len(attachments); i++ {
		attachments[i].ConversationID = conversationID
		if err := repo_model.UpdateAttachment(ctx, attachments[i]); err != nil {
			return fmt.Errorf("update attachment [id: %d]: %w", attachments[i].ID, err)
		}
	}
	return committer.Commit()
}

// NewConversationOptions represents the options of a new conversation.
type NewConversationOptions struct {
	Repo         *repo_model.Repository
	Conversation *Conversation
	LabelIDs     []int64
	Attachments  []string // In UUID format.
	IsPull       bool
}

// UpdateConversationMentions updates conversation-user relations for mentioned users.
func UpdateConversationMentions(ctx context.Context, conversationID int64, mentions []*user_model.User) error {
	if len(mentions) == 0 {
		return nil
	}
	ids := make([]int64, len(mentions))
	for i, u := range mentions {
		ids[i] = u.ID
	}
	if err := UpdateConversationUsersByMentions(ctx, conversationID, ids); err != nil {
		return fmt.Errorf("UpdateConversationUsersByMentions: %w", err)
	}
	return nil
}

// FindAndUpdateConversationMentions finds users mentioned in the given content string, and saves them in the database.
func FindAndUpdateConversationMentions(ctx context.Context, conversation *Conversation, doer *user_model.User, content string) (mentions []*user_model.User, err error) {
	rawMentions := references.FindAllMentionsMarkdown(content)
	mentions, err = ResolveConversationMentionsByVisibility(ctx, conversation, doer, rawMentions)
	if err != nil {
		return nil, fmt.Errorf("UpdateConversationMentions [%d]: %w", conversation.ID, err)
	}

	notBlocked := make([]*user_model.User, 0, len(mentions))
	for _, user := range mentions {
		if !user_model.IsUserBlockedBy(ctx, doer, user.ID) {
			notBlocked = append(notBlocked, user)
		}
	}
	mentions = notBlocked

	if err = UpdateConversationMentions(ctx, conversation.ID, mentions); err != nil {
		return nil, fmt.Errorf("UpdateConversationMentions [%d]: %w", conversation.ID, err)
	}
	return mentions, err
}

// ResolveConversationMentionsByVisibility returns the users mentioned in an conversation, removing those that
// don't have access to reading it. Teams are expanded into their users, but organizations are ignored.
func ResolveConversationMentionsByVisibility(ctx context.Context, conversation *Conversation, doer *user_model.User, mentions []string) (users []*user_model.User, err error) {
	if len(mentions) == 0 {
		return nil, nil
	}
	if err = conversation.LoadRepo(ctx); err != nil {
		return nil, err
	}

	resolved := make(map[string]bool, 10)
	var mentionTeams []string

	if err := conversation.Repo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	repoOwnerIsOrg := conversation.Repo.Owner.IsOrganization()
	if repoOwnerIsOrg {
		mentionTeams = make([]string, 0, 5)
	}

	resolved[doer.LowerName] = true
	for _, name := range mentions {
		name := strings.ToLower(name)
		if _, ok := resolved[name]; ok {
			continue
		}
		if repoOwnerIsOrg && strings.Contains(name, "/") {
			names := strings.Split(name, "/")
			if len(names) < 2 || names[0] != conversation.Repo.Owner.LowerName {
				continue
			}
			mentionTeams = append(mentionTeams, names[1])
			resolved[name] = true
		} else {
			resolved[name] = false
		}
	}

	if conversation.Repo.Owner.IsOrganization() && len(mentionTeams) > 0 {
		teams := make([]*organization.Team, 0, len(mentionTeams))
		if err := db.GetEngine(ctx).
			Join("INNER", "team_repo", "team_repo.team_id = team.id").
			Where("team_repo.repo_id=?", conversation.Repo.ID).
			In("team.lower_name", mentionTeams).
			Find(&teams); err != nil {
			return nil, fmt.Errorf("find mentioned teams: %w", err)
		}
		if len(teams) != 0 {
			checked := make([]int64, 0, len(teams))
			unittype := unit.TypeConversations
			for _, team := range teams {
				if team.AccessMode >= perm.AccessModeAdmin {
					checked = append(checked, team.ID)
					resolved[conversation.Repo.Owner.LowerName+"/"+team.LowerName] = true
					continue
				}
				has, err := db.GetEngine(ctx).Get(&organization.TeamUnit{OrgID: conversation.Repo.Owner.ID, TeamID: team.ID, Type: unittype})
				if err != nil {
					return nil, fmt.Errorf("get team units (%d): %w", team.ID, err)
				}
				if has {
					checked = append(checked, team.ID)
					resolved[conversation.Repo.Owner.LowerName+"/"+team.LowerName] = true
				}
			}
			if len(checked) != 0 {
				teamusers := make([]*user_model.User, 0, 20)
				if err := db.GetEngine(ctx).
					Join("INNER", "team_user", "team_user.uid = `user`.id").
					In("`team_user`.team_id", checked).
					And("`user`.is_active = ?", true).
					And("`user`.prohibit_login = ?", false).
					Find(&teamusers); err != nil {
					return nil, fmt.Errorf("get teams users: %w", err)
				}
				if len(teamusers) > 0 {
					users = make([]*user_model.User, 0, len(teamusers))
					for _, user := range teamusers {
						if already, ok := resolved[user.LowerName]; !ok || !already {
							users = append(users, user)
							resolved[user.LowerName] = true
						}
					}
				}
			}
		}
	}

	// Remove names already in the list to avoid querying the database if pending names remain
	mentionUsers := make([]string, 0, len(resolved))
	for name, already := range resolved {
		if !already {
			mentionUsers = append(mentionUsers, name)
		}
	}
	if len(mentionUsers) == 0 {
		return users, err
	}

	if users == nil {
		users = make([]*user_model.User, 0, len(mentionUsers))
	}

	unchecked := make([]*user_model.User, 0, len(mentionUsers))
	if err := db.GetEngine(ctx).
		Where("`user`.is_active = ?", true).
		And("`user`.prohibit_login = ?", false).
		In("`user`.lower_name", mentionUsers).
		Find(&unchecked); err != nil {
		return nil, fmt.Errorf("find mentioned users: %w", err)
	}
	for _, user := range unchecked {
		if already := resolved[user.LowerName]; already || user.IsOrganization() {
			continue
		}
		// Normal users must have read access to the referencing conversation
		perm, err := access_model.GetUserRepoPermission(ctx, conversation.Repo, user)
		if err != nil {
			return nil, fmt.Errorf("GetUserRepoPermission [%d]: %w", user.ID, err)
		}
		if !perm.CanReadConversations() {
			continue
		}
		users = append(users, user)
	}

	return users, err
}

// UpdateConversationsMigrationsByType updates all migrated repositories' conversations from gitServiceType to replace originalAuthorID to posterID
func UpdateConversationsMigrationsByType(ctx context.Context, gitServiceType api.GitServiceType, originalAuthorID string, posterID int64) error {
	_, err := db.GetEngine(ctx).Table("conversation").
		Where("repo_id IN (SELECT id FROM repository WHERE original_service_type = ?)", gitServiceType).
		And("original_author_id = ?", originalAuthorID).
		Update(map[string]any{
			"poster_id":          posterID,
			"original_author":    "",
			"original_author_id": 0,
		})
	return err
}

// UpdateReactionsMigrationsByType updates all migrated repositories' reactions from gitServiceType to replace originalAuthorID to posterID
func UpdateReactionsMigrationsByType(ctx context.Context, gitServiceType api.GitServiceType, originalAuthorID string, userID int64) error {
	_, err := db.GetEngine(ctx).Table("reaction").
		Where("original_author_id = ?", originalAuthorID).
		And(migratedConversationCond(gitServiceType)).
		Update(map[string]any{
			"user_id":            userID,
			"original_author":    "",
			"original_author_id": 0,
		})
	return err
}

// DeleteConversationsByRepoID deletes conversations by repositories id
func DeleteConversationsByRepoID(ctx context.Context, repoID int64) (attachmentPaths []string, err error) {
	// MariaDB has a performance bug: https://jira.mariadb.org/browse/MDEV-16289
	// so here it uses "DELETE ... WHERE IN" with pre-queried IDs.
	sess := db.GetEngine(ctx)

	for {
		conversationIDs := make([]int64, 0, db.DefaultMaxInSize)

		err := sess.Table(&Conversation{}).Where("repo_id = ?", repoID).OrderBy("id").Limit(db.DefaultMaxInSize).Cols("id").Find(&conversationIDs)
		if err != nil {
			return nil, err
		}

		if len(conversationIDs) == 0 {
			break
		}

		// Delete content histories
		_, err = sess.In("conversation_id", conversationIDs).Delete(&ConversationContentHistory{})
		if err != nil {
			return nil, err
		}

		// Delete comments and attachments
		_, err = sess.In("conversation_id", conversationIDs).Delete(&Comment{})
		if err != nil {
			return nil, err
		}

		_, err = sess.In("conversation_id", conversationIDs).Delete(&ConversationUser{})
		if err != nil {
			return nil, err
		}

		_, err = sess.In("conversation_id", conversationIDs).Delete(&CommentReaction{})
		if err != nil {
			return nil, err
		}

		_, err = sess.In("dependent_conversation_id", conversationIDs).Delete(&Comment{})
		if err != nil {
			return nil, err
		}

		var attachments []*repo_model.Attachment
		err = sess.In("conversation_id", conversationIDs).Find(&attachments)
		if err != nil {
			return nil, err
		}

		for j := range attachments {
			attachmentPaths = append(attachmentPaths, attachments[j].RelativePath())
		}

		_, err = sess.In("conversation_id", conversationIDs).Delete(&repo_model.Attachment{})
		if err != nil {
			return nil, err
		}

		_, err = sess.In("id", conversationIDs).Delete(&Conversation{})
		if err != nil {
			return nil, err
		}
	}

	return attachmentPaths, err
}

// DeleteOrphanedConversations delete conversations without a repo
func DeleteOrphanedConversations(ctx context.Context) error {
	var attachmentPaths []string
	err := db.WithTx(ctx, func(ctx context.Context) error {
		var ids []int64

		if err := db.GetEngine(ctx).Table("conversation").Distinct("conversation.repo_id").
			Join("LEFT", "repository", "conversation.repo_id=repository.id").
			Where(builder.IsNull{"repository.id"}).GroupBy("conversation.repo_id").
			Find(&ids); err != nil {
			return err
		}

		for i := range ids {
			paths, err := DeleteConversationsByRepoID(ctx, ids[i])
			if err != nil {
				return err
			}
			attachmentPaths = append(attachmentPaths, paths...)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Remove conversation attachment files.
	for i := range attachmentPaths {
		system_model.RemoveAllWithNotice(ctx, "Delete conversation attachment", attachmentPaths[i])
	}
	return nil
}

// NewConversationWithIndex creates conversation with given index
func NewConversationWithIndex(ctx context.Context, opts NewConversationOptions) (err error) {
	e := db.GetEngine(ctx)

	if opts.Conversation.Index <= 0 {
		return fmt.Errorf("no conversation index provided")
	}
	if opts.Conversation.ID > 0 {
		return fmt.Errorf("conversation exist")
	}

	if _, err := e.Insert(opts.Conversation); err != nil {
		return err
	}

	if err := repo_model.UpdateRepoConversationNumbers(ctx, opts.Conversation.RepoID, false); err != nil {
		return err
	}

	if err = NewConversationUsers(ctx, opts.Repo, opts.Conversation); err != nil {
		return err
	}

	if len(opts.Attachments) > 0 {
		attachments, err := repo_model.GetAttachmentsByUUIDs(ctx, opts.Attachments)
		if err != nil {
			return fmt.Errorf("getAttachmentsByUUIDs [uuids: %v]: %w", opts.Attachments, err)
		}

		for i := 0; i < len(attachments); i++ {
			attachments[i].ConversationID = opts.Conversation.ID
			if _, err = e.ID(attachments[i].ID).Update(attachments[i]); err != nil {
				return fmt.Errorf("update attachment [id: %d]: %w", attachments[i].ID, err)
			}
		}
	}

	return opts.Conversation.LoadAttributes(ctx)
}

// NewConversation creates new conversation with labels for repository.
func NewConversation(ctx context.Context, repo *repo_model.Repository, conversation *Conversation, uuids []string) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	idx, err := db.GetNextResourceIndex(ctx, "conversation_index", repo.ID)
	if err != nil {
		return fmt.Errorf("generate conversation index failed: %w", err)
	}

	conversation.Index = idx

	if err = NewConversationWithIndex(ctx, NewConversationOptions{
		Repo:         repo,
		Conversation: conversation,
		Attachments:  uuids,
	}); err != nil {
		if repo_model.IsErrUserDoesNotHaveAccessToRepo(err) || IsErrNewConversationInsert(err) {
			return err
		}
		return fmt.Errorf("newConversation: %w", err)
	}

	if err = committer.Commit(); err != nil {
		return fmt.Errorf("Commit: %w", err)
	}

	return nil
}
