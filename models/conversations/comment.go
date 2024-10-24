package conversations

// This comment.go was refactored from issues/comment.go to make it context-agnostic to improve reusability.

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"

	"html/template"

	"xorm.io/builder"
)

// ErrCommentNotExist represents a "CommentNotExist" kind of error.
type ErrCommentNotExist struct {
	ID             int64
	ConversationID int64
}

// IsErrCommentNotExist checks if an error is a ErrCommentNotExist.
func IsErrCommentNotExist(err error) bool {
	_, ok := err.(ErrCommentNotExist)
	return ok
}

func (err ErrCommentNotExist) Error() string {
	return fmt.Sprintf("comment does not exist [id: %d, conversation_id: %d]", err.ID, err.ConversationID)
}

func (err ErrCommentNotExist) Unwrap() error {
	return util.ErrNotExist
}

var ErrCommentAlreadyChanged = util.NewInvalidArgumentErrorf("the comment is already changed")

// CommentType defines whether a comment is just a simple comment, an action (like close) or a reference.
type CommentType int

// CommentTypeUndefined is used to search for comments of any type
const CommentTypeUndefined CommentType = -1

const (
	CommentTypeComment CommentType = iota // 0 Plain comment, can be associated with a commit (CommitID > 0) and a line (LineNum > 0)

	CommentTypeLock   // 1 Lock an conversation, giving only collaborators access
	CommentTypeUnlock // 2 Unlocks a previously locked conversation

	CommentTypeAddDependency
	CommentTypeRemoveDependency
)

var commentStrings = []string{
	"comment",
	"lock",
	"unlock",
}

func (t CommentType) String() string {
	return commentStrings[t]
}

func AsCommentType(typeName string) CommentType {
	for index, name := range commentStrings {
		if typeName == name {
			return CommentType(index)
		}
	}
	return CommentTypeUndefined
}

func (t CommentType) HasContentSupport() bool {
	switch t {
	case CommentTypeComment:
		return true
	}
	return false
}

func (t CommentType) HasAttachmentSupport() bool {
	switch t {
	case CommentTypeComment:
		return true
	}
	return false
}

func (t CommentType) HasMailReplySupport() bool {
	switch t {
	case CommentTypeComment:
		return true
	}
	return false
}

// CommentMetaData stores metadata for a comment, these data will not be changed once inserted into database
type CommentMetaData struct {
	ProjectColumnID    int64  `json:"project_column_id,omitempty"`
	ProjectColumnTitle string `json:"project_column_title,omitempty"`
	ProjectTitle       string `json:"project_title,omitempty"`
}

// Comment represents a comment in commit and conversation page.
// Comment struct should not contain any pointers unrelated to Conversation unless absolutely necessary.
// To have pointers outside of conversation, create another comment type (e.g. ConversationComment) and use a converter to load it in.
// The database data for the comments however, for all comment types, are defined here.
type Comment struct {
	ID   int64       `xorm:"pk autoincr"`
	Type CommentType `xorm:"INDEX"`

	PosterID int64            `xorm:"INDEX"`
	Poster   *user_model.User `xorm:"-"`

	OriginalAuthor   string
	OriginalAuthorID int64

	Attachments []*repo_model.Attachment `xorm:"-"`
	Reactions   ReactionList             `xorm:"-"`

	Content        string `xorm:"LONGTEXT"`
	ContentVersion int    `xorm:"NOT NULL DEFAULT 0"`

	ConversationID int64 `xorm:"INDEX"`
	Conversation   *Conversation

	DependentConversationID int64 `xorm:"INDEX"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

	RenderedContent template.HTML  `xorm:"-"`
	ShowRole        RoleDescriptor `xorm:"-"`
}

func init() {
	db.RegisterModel(new(Comment))
}

// LoadPoster loads comment poster
func (c *Comment) LoadPoster(ctx context.Context) (err error) {
	if c.Poster != nil {
		return nil
	}

	c.Poster, err = user_model.GetPossibleUserByID(ctx, c.PosterID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			c.PosterID = user_model.GhostUserID
			c.Poster = user_model.NewGhostUser()
		} else {
			log.Error("getUserByID[%d]: %v", c.ID, err)
		}
	}
	return err
}

// Creates conversation dependency comment
func createConversationDependencyComment(ctx context.Context, doer *user_model.User, conversation, dependentConversation *Conversation, add bool) (err error) {
	cType := CommentTypeAddDependency
	if !add {
		cType = CommentTypeRemoveDependency
	}
	if err = conversation.LoadRepo(ctx); err != nil {
		return err
	}

	// Make two comments, one in each conversation
	opts := &CreateCommentOptions{
		Type:                    cType,
		Doer:                    doer,
		Repo:                    conversation.Repo,
		Conversation:            conversation,
		DependentConversationID: dependentConversation.ID,
	}
	if _, err = CreateComment(ctx, opts); err != nil {
		return err
	}

	opts = &CreateCommentOptions{
		Type:                    cType,
		Doer:                    doer,
		Repo:                    conversation.Repo,
		Conversation:            dependentConversation,
		DependentConversationID: conversation.ID,
	}
	_, err = CreateComment(ctx, opts)
	return err
}

// LoadReactions loads comment reactions
func (c *Comment) LoadReactions(ctx context.Context, repo *repo_model.Repository) (err error) {
	if c.Reactions != nil {
		return nil
	}
	c.Reactions, _, err = FindReactions(ctx, FindReactionsOptions{
		ConversationID: c.ConversationID,
		CommentID:      c.ID,
	})
	if err != nil {
		return err
	}
	// Load reaction user data
	if _, err := c.Reactions.LoadUsers(ctx, repo); err != nil {
		return err
	}
	return nil
}

// AfterDelete is invoked from XORM after the object is deleted.
func (c *Comment) AfterDelete(ctx context.Context) {
	if c.ID <= 0 {
		return
	}

	_, err := repo_model.DeleteAttachmentsByComment(ctx, c.ID, true)
	if err != nil {
		log.Info("Could not delete files for comment %d on conversation #%d: %s", c.ID, c.ConversationID, err)
	}
}

// RoleInRepo presents the user's participation in the repo
type RoleInRepo string

// RoleDescriptor defines comment "role" tags
type RoleDescriptor struct {
	IsPoster   bool
	RoleInRepo RoleInRepo
}

// Enumerate all the role tags.
const (
	RoleRepoOwner                RoleInRepo = "owner"
	RoleRepoMember               RoleInRepo = "member"
	RoleRepoCollaborator         RoleInRepo = "collaborator"
	RoleRepoFirstTimeContributor RoleInRepo = "first_time_contributor"
	RoleRepoContributor          RoleInRepo = "contributor"
)

// LocaleString returns the locale string name of the role
func (r RoleInRepo) LocaleString(lang translation.Locale) string {
	return lang.TrString("repo.conversations.role." + string(r))
}

// LocaleHelper returns the locale tooltip of the role
func (r RoleInRepo) LocaleHelper(lang translation.Locale) string {
	return lang.TrString("repo.conversations.role." + string(r) + "_helper")
}

// CreateCommentOptions defines options for creating comment
type CreateCommentOptions struct {
	Type                    CommentType
	Doer                    *user_model.User
	Repo                    *repo_model.Repository
	Attachments             []string // UUIDs of attachments
	ConversationID          int64
	Conversation            *Conversation
	Content                 string
	DependentConversationID int64
}

// CreateComment creates comment with context
func CreateComment(ctx context.Context, opts *CreateCommentOptions) (_ *Comment, err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	e := db.GetEngine(ctx)

	comment := &Comment{
		Type:           opts.Type,
		PosterID:       opts.Doer.ID,
		Poster:         opts.Doer,
		ConversationID: opts.ConversationID,
	}
	if _, err = e.Insert(comment); err != nil {
		return nil, err
	}

	if err = opts.Repo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	if err = committer.Commit(); err != nil {
		return nil, err
	}
	return comment, nil
}

// GetCommentByID returns the comment by given ID.
func GetCommentByID(ctx context.Context, id int64) (*Comment, error) {
	c := new(Comment)
	has, err := db.GetEngine(ctx).ID(id).Get(c)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrCommentNotExist{id, 0}
	}
	return c, nil
}

// FindCommentsOptions describes the conditions to Find comments
type FindCommentsOptions struct {
	db.ListOptions
	RepoID          int64
	ConversationID  int64
	ReviewID        int64
	Since           int64
	Before          int64
	Line            int64
	TreePath        string
	Type            CommentType
	ConversationIDs []int64
	Invalidated     optional.Option[bool]
	IsPull          optional.Option[bool]
}

// ToConds implements FindOptions interface
func (opts FindCommentsOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"conversation.repo_id": opts.RepoID})
	}
	if opts.ConversationID > 0 {
		cond = cond.And(builder.Eq{"comment.conversation_id": opts.ConversationID})
	} else if len(opts.ConversationIDs) > 0 {
		cond = cond.And(builder.In("comment.conversation_id", opts.ConversationIDs))
	}
	if opts.ReviewID > 0 {
		cond = cond.And(builder.Eq{"comment.review_id": opts.ReviewID})
	}
	if opts.Since > 0 {
		cond = cond.And(builder.Gte{"comment.updated_unix": opts.Since})
	}
	if opts.Before > 0 {
		cond = cond.And(builder.Lte{"comment.updated_unix": opts.Before})
	}
	if opts.Type != CommentTypeUndefined {
		cond = cond.And(builder.Eq{"comment.type": opts.Type})
	}
	if opts.Line != 0 {
		cond = cond.And(builder.Eq{"comment.line": opts.Line})
	}
	if len(opts.TreePath) > 0 {
		cond = cond.And(builder.Eq{"comment.tree_path": opts.TreePath})
	}
	if opts.Invalidated.Has() {
		cond = cond.And(builder.Eq{"comment.invalidated": opts.Invalidated.Value()})
	}
	if opts.IsPull.Has() {
		cond = cond.And(builder.Eq{"conversation.is_pull": opts.IsPull.Value()})
	}
	return cond
}

// FindComments returns all comments according options
func FindComments(ctx context.Context, opts *FindCommentsOptions) (CommentList, error) {
	comments := make([]*Comment, 0, 10)
	sess := db.GetEngine(ctx).Where(opts.ToConds())
	if opts.RepoID > 0 || opts.IsPull.Has() {
		sess.Join("INNER", "conversation", "conversation.id = comment.conversation_id")
	}

	if opts.Page != 0 {
		sess = db.SetSessionPagination(sess, opts)
	}

	// WARNING: If you change this order you will need to fix createCodeComment

	return comments, sess.
		Asc("comment.created_unix").
		Asc("comment.id").
		Find(&comments)
}

// CountComments count all comments according options by ignoring pagination
func CountComments(ctx context.Context, opts *FindCommentsOptions) (int64, error) {
	sess := db.GetEngine(ctx).Where(opts.ToConds())
	if opts.RepoID > 0 {
		sess.Join("INNER", "conversation", "conversation.id = comment.conversation_id")
	}
	return sess.Count(&Comment{})
}

// UpdateCommentInvalidate updates comment invalidated column
func UpdateCommentInvalidate(ctx context.Context, c *Comment) error {
	_, err := db.GetEngine(ctx).ID(c.ID).Cols("invalidated").Update(c)
	return err
}

// UpdateComment updates information of comment.
func UpdateComment(ctx context.Context, c *Comment, contentVersion int, doer *user_model.User) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	c.ContentVersion = contentVersion + 1

	affected, err := sess.ID(c.ID).AllCols().Where("content_version = ?", contentVersion).Update(c)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrCommentAlreadyChanged
	}
	if err := committer.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// DeleteComment deletes the comment
func DeleteComment(ctx context.Context, comment *Comment) error {
	e := db.GetEngine(ctx)
	if _, err := e.ID(comment.ID).NoAutoCondition().Delete(comment); err != nil {
		return err
	}

	if _, err := db.DeleteByBean(ctx, &ConversationContentHistory{
		CommentID: comment.ID,
	}); err != nil {
		return err
	}

	if _, err := e.Table("action").
		Where("comment_id = ?", comment.ID).
		Update(map[string]any{
			"is_deleted": true,
		}); err != nil {
		return err
	}

	return DeleteReaction(ctx, &ReactionOptions{CommentID: comment.ID})
}

// UpdateCommentsMigrationsByType updates comments' migrations information via given git service type and original id and poster id
func UpdateCommentsMigrationsByType(ctx context.Context, tp structs.GitServiceType, originalAuthorID string, posterID int64) error {
	_, err := db.GetEngine(ctx).Table("comment").
		Join("INNER", "conversation", "conversation.id = comment.conversation_id").
		Join("INNER", "repository", "conversation.repo_id = repository.id").
		Where("repository.original_service_type = ?", tp).
		And("comment.original_author_id = ?", originalAuthorID).
		Update(map[string]any{
			"poster_id":          posterID,
			"original_author":    "",
			"original_author_id": 0,
		})
	return err
}

func UpdateAttachments(ctx context.Context, opts *CreateCommentOptions, comment *Comment) error {
	attachments, err := repo_model.GetAttachmentsByUUIDs(ctx, opts.Attachments)
	if err != nil {
		return fmt.Errorf("getAttachmentsByUUIDs [uuids: %v]: %w", opts.Attachments, err)
	}
	for i := range attachments {
		attachments[i].ConversationID = comment.ConversationID
		attachments[i].CommentID = comment.ID
		// No assign value could be 0, so ignore AllCols().
		if _, err = db.GetEngine(ctx).ID(attachments[i].ID).Update(attachments[i]); err != nil {
			return fmt.Errorf("update attachment [%d]: %w", attachments[i].ID, err)
		}
	}
	comment.Attachments = attachments
	return nil
}

// LoadConversation loads the conversation reference for the comment
func (c *Comment) LoadConversation(ctx context.Context) (err error) {
	if c.Conversation != nil {
		return nil
	}
	c.Conversation, err = GetConversationByID(ctx, c.ConversationID)
	return err
}

// LoadAttachments loads attachments (it never returns error, the error during `GetAttachmentsByCommentIDCtx` is ignored)
func (c *Comment) LoadAttachments(ctx context.Context) error {
	if len(c.Attachments) > 0 {
		return nil
	}

	var err error
	c.Attachments, err = repo_model.GetAttachmentsByCommentID(ctx, c.ID)
	if err != nil {
		log.Error("getAttachmentsByCommentID[%d]: %v", c.ID, err)
	}
	return nil
}

// UpdateAttachments update attachments by UUIDs for the comment
func (c *Comment) UpdateAttachments(ctx context.Context, uuids []string) error {
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
		attachments[i].ConversationID = c.ConversationID
		attachments[i].CommentID = c.ID
		if err := repo_model.UpdateAttachment(ctx, attachments[i]); err != nil {
			return fmt.Errorf("update attachment [id: %d]: %w", attachments[i].ID, err)
		}
	}
	return committer.Commit()
}

// HashTag returns unique hash tag for issue.
func (comment *Comment) HashTag() string {
	return fmt.Sprintf("comment-%d", comment.ID)
}

func (c *Comment) hashLink(ctx context.Context) string {
	return "#" + c.HashTag()
}

// HTMLURL formats a URL-string to the conversation-comment
func (c *Comment) HTMLURL(ctx context.Context) string {
	err := c.LoadConversation(ctx)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("LoadConversation(%d): %v", c.ConversationID, err)
		return ""
	}
	err = c.Conversation.LoadRepo(ctx)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("loadRepo(%d): %v", c.Conversation.RepoID, err)
		return ""
	}
	return c.Conversation.HTMLURL() + c.hashLink(ctx)
}

// APIURL formats a API-string to the conversation-comment
func (c *Comment) APIURL(ctx context.Context) string {
	err := c.LoadConversation(ctx)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("LoadConversation(%d): %v", c.ConversationID, err)
		return ""
	}
	err = c.Conversation.LoadRepo(ctx)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("loadRepo(%d): %v", c.Conversation.RepoID, err)
		return ""
	}

	return fmt.Sprintf("%s/conversations/comments/%d", c.Conversation.Repo.APIURL(), c.ID)
}

// HasOriginalAuthor returns if a comment was migrated and has an original author.
func (c *Comment) HasOriginalAuthor() bool {
	return c.OriginalAuthor != "" && c.OriginalAuthorID != 0
}
