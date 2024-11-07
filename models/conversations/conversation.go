// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations

// Someone should decouple Comment from issues, and rename it something like ConversationEvent (@RedCocoon, 2024)
// Much of the functions here are reimplemented from models/issues/issue.go but simplified

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrConversationNotExist represents a "ConversationNotExist" kind of error.
type ErrConversationNotExist struct {
	ID     int64
	RepoID int64
	Index  int64
}

// IsErrConversationNotExist checks if an error is a ErrConversationNotExist.
func IsErrConversationNotExist(err error) bool {
	_, ok := err.(ErrConversationNotExist)
	return ok
}

func (err ErrConversationNotExist) Error() string {
	return fmt.Sprintf("conversation does not exist [id: %d, repo_id: %d, index: %d]", err.ID, err.RepoID, err.Index)
}

func (err ErrConversationNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrConversationIsClosed represents a "ConversationIsClosed" kind of error.
type ErrConversationIsClosed struct {
	ID     int64
	RepoID int64
	Index  int64
}

// IsErrConversationIsClosed checks if an error is a ErrConversationNotExist.
func IsErrConversationIsClosed(err error) bool {
	_, ok := err.(ErrConversationIsClosed)
	return ok
}

func (err ErrConversationIsClosed) Error() string {
	return fmt.Sprintf("conversation is closed [id: %d, repo_id: %d, index: %d]", err.ID, err.RepoID, err.Index)
}

// ErrNewConversationInsert is used when the INSERT statement in newConversation fails
type ErrNewConversationInsert struct {
	OriginalError error
}

// IsErrNewConversationInsert checks if an error is a ErrNewConversationInsert.
func IsErrNewConversationInsert(err error) bool {
	_, ok := err.(ErrNewConversationInsert)
	return ok
}

func (err ErrNewConversationInsert) Error() string {
	return err.OriginalError.Error()
}

// ErrConversationWasClosed is used when close a closed conversation
type ErrConversationWasClosed struct {
	ID    int64
	Index int64
}

// IsErrConversationWasClosed checks if an error is a ErrConversationWasClosed.
func IsErrConversationWasClosed(err error) bool {
	_, ok := err.(ErrConversationWasClosed)
	return ok
}

func (err ErrConversationWasClosed) Error() string {
	return fmt.Sprintf("Conversation [%d] %d was already closed", err.ID, err.Index)
}

var ErrConversationAlreadyChanged = util.NewInvalidArgumentErrorf("the conversation is already changed")

type ConversationType int

// CommentTypeUndefined is used to search for comments of any type
const ConversationTypeUndefined CommentType = -1

const (
	ConversationTypeCommit ConversationType = iota
)

// Conversation represents a conversation.
type Conversation struct {
	ID     int64                  `xorm:"pk autoincr"`
	Index  int64                  `xorm:"UNIQUE(repo_index)"`
	RepoID int64                  `xorm:"INDEX UNIQUE(repo_index)"`
	Repo   *repo_model.Repository `xorm:"-"`
	Type   ConversationType       `xorm:"INDEX"`

	NumComments int

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	LockedUnix  timeutil.TimeStamp `xorm:"INDEX"`

	IsLocked bool `xorm:"INDEX DEFAULT false"`

	Comments CommentList `xorm:"-"`

	CommitSha string `xorm:"VARCHAR(64)"`
	IsRead    bool   `xorm:"-"`
}

// ConversationIndex represents the conversation index table
type ConversationIndex db.ResourceIndex

func init() {
	db.RegisterModel(new(Conversation))
	db.RegisterModel(new(ConversationIndex))
}

// In the future if there are more than one type of conversations
// Add a Type argument to Conversation to differentiate them
func (conversation *Conversation) Link() string {
	switch conversation.Type {
	default:
		return fmt.Sprintf("%s/%s/%s", conversation.Repo.Link(), "commit", conversation.CommitSha)
	}
}

func (conversation *Conversation) loadComments(ctx context.Context) (err error) {
	conversation.Comments, err = FindComments(ctx, &FindCommentsOptions{
		ConversationID: conversation.ID,
	})

	return err
}

func (conversation *Conversation) loadCommentsByType(ctx context.Context, tp CommentType) (err error) {
	if conversation.Comments != nil {
		return nil
	}

	conversation.Comments, err = FindComments(ctx, &FindCommentsOptions{
		ConversationID: conversation.ID,
		Type:           tp,
	})

	return err
}

// GetConversationByID returns an conversation by given ID.
func GetConversationByID(ctx context.Context, id int64) (*Conversation, error) {
	conversation := new(Conversation)
	has, err := db.GetEngine(ctx).ID(id).Get(conversation)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrConversationNotExist{id, 0, 0}
	}
	return conversation, nil
}

// GetConversationByIndex returns raw conversation without loading attributes by index in a repository.
func GetConversationByIndex(ctx context.Context, repoID, index int64) (*Conversation, error) {
	if index < 1 {
		return nil, ErrConversationNotExist{}
	}
	conversation := &Conversation{
		RepoID: repoID,
		Index:  index,
	}
	has, err := db.GetEngine(ctx).Get(conversation)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrConversationNotExist{0, repoID, index}
	}
	return conversation, nil
}

// LoadDiscussComments loads discuss comments
func (conversation *Conversation) LoadDiscussComments(ctx context.Context) error {
	return conversation.loadCommentsByType(ctx, CommentTypeComment)
}

// LoadAttributes loads the attribute of this conversation.
func (conversation *Conversation) LoadAttributes(ctx context.Context) (err error) {
	if err = conversation.LoadRepo(ctx); err != nil {
		return err
	}

	if err = conversation.loadComments(ctx); err != nil {
		return err
	}

	if err = conversation.loadReactions(ctx); err != nil {
		return err
	}

	return conversation.Comments.LoadAttributes(ctx)
}

// LoadRepo loads conversation's repository
func (conversation *Conversation) LoadRepo(ctx context.Context) (err error) {
	if conversation.Repo == nil && conversation.RepoID != 0 {
		conversation.Repo, err = repo_model.GetRepositoryByID(ctx, conversation.RepoID)
		if err != nil {
			return fmt.Errorf("getRepositoryByID [%d]: %w", conversation.RepoID, err)
		}
	}
	return nil
}

// GetConversationIDsByRepoID returns all conversation ids by repo id
func GetConversationIDsByRepoID(ctx context.Context, repoID int64) ([]int64, error) {
	ids := make([]int64, 0, 10)
	err := db.GetEngine(ctx).Table("conversation").Cols("id").Where("repo_id = ?", repoID).Find(&ids)
	return ids, err
}

// GetConversationsByIDs return conversations with the given IDs.
// If keepOrder is true, the order of the returned Conversations will be the same as the given IDs.
func GetConversationsByIDs(ctx context.Context, conversationIDs []int64, keepOrder ...bool) (ConversationList, error) {
	conversations := make([]*Conversation, 0, len(conversationIDs))

	if err := db.GetEngine(ctx).In("id", conversationIDs).Find(&conversations); err != nil {
		return nil, err
	}

	if len(keepOrder) > 0 && keepOrder[0] {
		m := make(map[int64]*Conversation, len(conversations))
		appended := container.Set[int64]{}
		for _, conversation := range conversations {
			m[conversation.ID] = conversation
		}
		conversations = conversations[:0]
		for _, id := range conversationIDs {
			if conversation, ok := m[id]; ok && !appended.Contains(id) { // make sure the id is existed and not appended
				appended.Add(id)
				conversations = append(conversations, conversation)
			}
		}
	}

	return conversations, nil
}

func GetConversationByCommitID(ctx context.Context, commitID string) (*Conversation, error) {
	conversation := &Conversation{
		CommitSha: commitID,
	}
	has, err := db.GetEngine(ctx).Get(conversation)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrConversationNotExist{0, 0, 0}
	}
	err = conversation.LoadAttributes(ctx)
	if err != nil {
		return nil, err
	}

	return conversation, nil
}

// GetConversationWithAttrsByIndex returns conversation by index in a repository.
func GetConversationWithAttrsByIndex(ctx context.Context, repoID, index int64) (*Conversation, error) {
	conversation, err := GetConversationByIndex(ctx, repoID, index)
	if err != nil {
		return nil, err
	}
	return conversation, conversation.LoadAttributes(ctx)
}

func migratedConversationCond(tp api.GitServiceType) builder.Cond {
	return builder.In("conversation_id",
		builder.Select("conversation.id").
			From("conversation").
			InnerJoin("repository", "conversation.repo_id = repository.id").
			Where(builder.Eq{
				"repository.original_service_type": tp,
			}),
	)
}

// HTMLURL returns the absolute URL to this conversation.
func (conversation *Conversation) HTMLURL() string {
	return fmt.Sprintf("%s/%s/%s", conversation.Repo.HTMLURL(), "commit", conversation.CommitSha)
}

// APIURL returns the absolute APIURL to this conversation.
func (conversation *Conversation) APIURL(ctx context.Context) string {
	if conversation.Repo == nil {
		err := conversation.LoadRepo(ctx)
		if err != nil {
			log.Error("Conversation[%d].APIURL(): %v", conversation.ID, err)
			return ""
		}
	}
	return fmt.Sprintf("%s/commit/%s", conversation.Repo.APIURL(), conversation.CommitSha)
}

func (conversation *Conversation) loadReactions(ctx context.Context) (err error) {
	reactions, _, err := FindReactions(ctx, FindReactionsOptions{
		ConversationID: conversation.ID,
	})
	if err != nil {
		return err
	}
	if err = conversation.LoadRepo(ctx); err != nil {
		return err
	}
	// Load reaction user data
	if _, err := reactions.LoadUsers(ctx, conversation.Repo); err != nil {
		return err
	}

	// Cache comments to map
	comments := make(map[int64]*ConversationComment)
	for _, comment := range conversation.Comments {
		comments[comment.ID] = comment
	}
	// Add reactions to comment
	for _, react := range reactions {
		if comment, ok := comments[react.CommentID]; ok {
			comment.Reactions = append(comment.Reactions, react)
		}
	}
	return nil
}

// InsertConversations insert issues to database
func InsertConversations(ctx context.Context, conversations ...*Conversation) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	for _, conversation := range conversations {
		if err := insertConversation(ctx, conversation); err != nil {
			return err
		}
	}
	return committer.Commit()
}

func insertConversation(ctx context.Context, conversation *Conversation) error {
	sess := db.GetEngine(ctx)
	if _, err := sess.NoAutoTime().Insert(conversation); err != nil {
		return err
	}
	return nil
}
