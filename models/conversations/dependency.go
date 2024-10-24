// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// ErrDependencyExists represents a "DependencyAlreadyExists" kind of error.
type ErrDependencyExists struct {
	ConversationID int64
	DependencyID   int64
}

// IsErrDependencyExists checks if an error is a ErrDependencyExists.
func IsErrDependencyExists(err error) bool {
	_, ok := err.(ErrDependencyExists)
	return ok
}

func (err ErrDependencyExists) Error() string {
	return fmt.Sprintf("conversation dependency does already exist [conversation id: %d, dependency id: %d]", err.ConversationID, err.DependencyID)
}

func (err ErrDependencyExists) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrDependencyNotExists represents a "DependencyAlreadyExists" kind of error.
type ErrDependencyNotExists struct {
	ConversationID int64
	DependencyID   int64
}

// IsErrDependencyNotExists checks if an error is a ErrDependencyExists.
func IsErrDependencyNotExists(err error) bool {
	_, ok := err.(ErrDependencyNotExists)
	return ok
}

func (err ErrDependencyNotExists) Error() string {
	return fmt.Sprintf("conversation dependency does not exist [conversation id: %d, dependency id: %d]", err.ConversationID, err.DependencyID)
}

func (err ErrDependencyNotExists) Unwrap() error {
	return util.ErrNotExist
}

// ErrCircularDependency represents a "DependencyCircular" kind of error.
type ErrCircularDependency struct {
	ConversationID int64
	DependencyID   int64
}

// IsErrCircularDependency checks if an error is a ErrCircularDependency.
func IsErrCircularDependency(err error) bool {
	_, ok := err.(ErrCircularDependency)
	return ok
}

func (err ErrCircularDependency) Error() string {
	return fmt.Sprintf("circular dependencies exists (two conversations blocking each other) [conversation id: %d, dependency id: %d]", err.ConversationID, err.DependencyID)
}

// ErrDependenciesLeft represents an error where the conversation you're trying to close still has dependencies left.
type ErrDependenciesLeft struct {
	ConversationID int64
}

// IsErrDependenciesLeft checks if an error is a ErrDependenciesLeft.
func IsErrDependenciesLeft(err error) bool {
	_, ok := err.(ErrDependenciesLeft)
	return ok
}

func (err ErrDependenciesLeft) Error() string {
	return fmt.Sprintf("conversation has open dependencies [conversation id: %d]", err.ConversationID)
}

// ErrUnknownDependencyType represents an error where an unknown dependency type was passed
type ErrUnknownDependencyType struct {
	Type DependencyType
}

// IsErrUnknownDependencyType checks if an error is ErrUnknownDependencyType
func IsErrUnknownDependencyType(err error) bool {
	_, ok := err.(ErrUnknownDependencyType)
	return ok
}

func (err ErrUnknownDependencyType) Error() string {
	return fmt.Sprintf("unknown dependency type [type: %d]", err.Type)
}

func (err ErrUnknownDependencyType) Unwrap() error {
	return util.ErrInvalidArgument
}

// ConversationDependency represents an conversation dependency
type ConversationDependency struct {
	ID             int64              `xorm:"pk autoincr"`
	UserID         int64              `xorm:"NOT NULL"`
	ConversationID int64              `xorm:"UNIQUE(conversation_dependency) NOT NULL"`
	DependencyID   int64              `xorm:"UNIQUE(conversation_dependency) NOT NULL"`
	CreatedUnix    timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix    timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ConversationDependency))
}

// DependencyType Defines Dependency Type Constants
type DependencyType int

// Define Dependency Types
const (
	DependencyTypeBlockedBy DependencyType = iota
	DependencyTypeBlocking
)

// CreateConversationDependency creates a new dependency for an conversation
func CreateConversationDependency(ctx context.Context, user *user_model.User, conversation, dep *Conversation) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	// Check if it already exists
	exists, err := conversationDepExists(ctx, conversation.ID, dep.ID)
	if err != nil {
		return err
	}
	if exists {
		return ErrDependencyExists{conversation.ID, dep.ID}
	}
	// And if it would be circular
	circular, err := conversationDepExists(ctx, dep.ID, conversation.ID)
	if err != nil {
		return err
	}
	if circular {
		return ErrCircularDependency{conversation.ID, dep.ID}
	}

	if err := db.Insert(ctx, &ConversationDependency{
		UserID:         user.ID,
		ConversationID: conversation.ID,
		DependencyID:   dep.ID,
	}); err != nil {
		return err
	}

	// Add comment referencing the new dependency
	if err = createConversationDependencyComment(ctx, user, conversation, dep, true); err != nil {
		return err
	}

	return committer.Commit()
}

// RemoveConversationDependency removes a dependency from an conversation
func RemoveConversationDependency(ctx context.Context, user *user_model.User, conversation, dep *Conversation, depType DependencyType) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	var conversationDepToDelete ConversationDependency

	switch depType {
	case DependencyTypeBlockedBy:
		conversationDepToDelete = ConversationDependency{ConversationID: conversation.ID, DependencyID: dep.ID}
	case DependencyTypeBlocking:
		conversationDepToDelete = ConversationDependency{ConversationID: dep.ID, DependencyID: conversation.ID}
	default:
		return ErrUnknownDependencyType{depType}
	}

	affected, err := db.GetEngine(ctx).Delete(&conversationDepToDelete)
	if err != nil {
		return err
	}

	// If we deleted nothing, the dependency did not exist
	if affected <= 0 {
		return ErrDependencyNotExists{conversation.ID, dep.ID}
	}

	// Add comment referencing the removed dependency
	if err = createConversationDependencyComment(ctx, user, conversation, dep, false); err != nil {
		return err
	}
	return committer.Commit()
}

// Check if the dependency already exists
func conversationDepExists(ctx context.Context, conversationID, depID int64) (bool, error) {
	return db.GetEngine(ctx).Where("(conversation_id = ? AND dependency_id = ?)", conversationID, depID).Exist(&ConversationDependency{})
}

// ConversationNoDependenciesLeft checks if conversation can be closed
func ConversationNoDependenciesLeft(ctx context.Context, conversation *Conversation) (bool, error) {
	exists, err := db.GetEngine(ctx).
		Table("conversation_dependency").
		Select("conversation.*").
		Join("INNER", "conversation", "conversation.id = conversation_dependency.dependency_id").
		Where("conversation_dependency.conversation_id = ?", conversation.ID).
		And("conversation.is_closed = ?", "0").
		Exist(&Conversation{})

	return !exists, err
}
