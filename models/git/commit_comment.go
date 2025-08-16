// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"slices"
	"strconv"
	"sync"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
)

type (
	CommitCommentList []*CommitComment
	ReactionMap       map[string][]string
	AttachmentMap     map[string]*AttachmentOptions
)

type CommitComment struct {
	ID               int64            `xorm:"pk autoincr"`
	PosterID         int64            `xorm:"INDEX"`
	Poster           *user_model.User `xorm:"-"`
	Line             int64
	FileName         string
	CommitSHA        string                 `xorm:"VARCHAR(64)"`
	Attachments      string                 `xorm:"JSON TEXT"`
	Reactions        string                 `xorm:"JSON TEXT"`
	Comment          string                 `xorm:"LONGTEXT"`
	RenderedComment  template.HTML          `xorm:"-"`
	ContentVersion   int                    `xorm:"NOT NULL DEFAULT 0"`
	RefRepoID        int64                  `xorm:"index"`
	Repo             *repo_model.Repository `xorm:"-"`
	ReplyToCommentID int64                  `xorm:"index"`
	CreatedUnix      timeutil.TimeStamp     `xorm:"INDEX created"`
	UpdatedUnix      timeutil.TimeStamp     `xorm:"INDEX updated"`
}

type CreateCommitCommentOptions struct {
	Doer *user_model.User
	Repo *repo_model.Repository

	CommitID         int64
	CommitSHA        string
	LineNum          int64
	Reactions        string
	Comment          string
	FileName         string
	Attachments      AttachmentMap
	RefRepoID        int64
	ReplyToCommentID int64
}

type AttachmentOptions struct {
	FileName   string
	Size       int64
	UploaderID int64
}

type FindCommitCommentOptions struct {
	db.ListOptions
	RepoID    int64
	CommitSHA string
	Since     int64
	Before    int64
	Line      int64
	FileName  string
}

var (
	attachmentsMap = make(AttachmentMap)
	mutex          sync.Mutex
)

func init() {
	db.RegisterModel(new(CommitComment))
}

func AddAttachment(uuid string, opts AttachmentOptions) {
	mutex.Lock()
	attachmentsMap[uuid] = &opts
	mutex.Unlock()
}

func RemoveAttachment(uuid string) {
	mutex.Lock()
	if attachmentsMap[uuid] != nil {
		delete(attachmentsMap, uuid)
	}
	mutex.Unlock()
}

func GetAttachments() AttachmentMap {
	return attachmentsMap
}

func ClearAttachments() {
	mutex.Lock()
	attachmentsMap = make(AttachmentMap)
	mutex.Unlock()
}

// HashTag returns unique hash tag for CommitComment.
func (commitComment *CommitComment) HashTag() string {
	return fmt.Sprintf("commitComment-%d", commitComment.ID)
}

func (commitComment *CommitComment) LoadRepo(ctx context.Context) (err error) {
	if commitComment.Repo == nil && commitComment.RefRepoID != 0 {
		commitComment.Repo, err = repo_model.GetRepositoryByID(ctx, commitComment.RefRepoID)
		if err != nil {
			return fmt.Errorf("getRepositoryByID [%d]: %w", commitComment.RefRepoID, err)
		}
	}
	return nil
}

// LoadPoster loads poster
func (commitComment *CommitComment) LoadPoster(ctx context.Context) (err error) {
	if commitComment.Poster == nil && commitComment.PosterID != 0 {
		commitComment.Poster, err = user_model.GetPossibleUserByID(ctx, commitComment.PosterID)
		if err != nil {
			commitComment.PosterID = user_model.GhostUserID
			commitComment.Poster = user_model.NewGhostUser()
			if !user_model.IsErrUserNotExist(err) {
				return fmt.Errorf("getUserByID.(poster) [%d]: %w", commitComment.PosterID, err)
			}
			return nil
		}
	}
	return err
}

func (commitComment *CommitComment) UnsignedLine() uint64 {
	if commitComment.Line < 0 {
		return uint64(commitComment.Line * -1)
	}
	return uint64(commitComment.Line)
}

func (commitComment *CommitComment) TreePath() string {
	return commitComment.FileName
}

// DiffSide returns "previous" if Comment.Line is a LOC of the previous changes and "proposed" if it is a LOC of the proposed changes.
func (commitComment *CommitComment) DiffSide() string {
	if commitComment.Line < 0 {
		return "previous"
	}
	return "proposed"
}

func (commitComment *CommitComment) GroupReactionsByType() (ReactionMap, error) {
	reactions := make(ReactionMap)

	err := json.Unmarshal([]byte(commitComment.Reactions), &reactions)
	if err != nil {
		return nil, errors.New("GroupReactionsByType")
	}
	return reactions, nil
}

func (commitComment *CommitComment) GroupAttachmentsByUUID() (AttachmentMap, error) {
	attachmentMap := make(AttachmentMap)
	err := json.Unmarshal([]byte(commitComment.Attachments), &attachmentMap)
	if err != nil {
		return nil, err
	}
	return attachmentMap, nil
}

// HasUser check if user has reacted
func (commitComment *CommitComment) HasUser(reaction string, userID int64) bool {
	if userID == 0 {
		return false
	}
	reactions, err := commitComment.GroupReactionsByType()
	if err != nil {
		return false
	}
	list := reactions[reaction]
	hasUser := false
	for _, userid := range list {
		id, _ := strconv.ParseInt(userid, 10, 64)
		if id == userID {
			hasUser = true
			return hasUser
		}
	}
	return hasUser
}

// GetFirstUsers returns first reacted user display names separated by comma
func (commitComment *CommitComment) GetFirstUsers(ctx context.Context, reaction string) string {
	var buffer bytes.Buffer
	rem := setting.UI.ReactionMaxUserNum
	reactions, err := commitComment.GroupReactionsByType()
	if err != nil {
		return ""
	}
	list := reactions[reaction]
	for _, userid := range list {
		if buffer.Len() > 0 {
			buffer.WriteString(", ")
		}
		id, _ := strconv.ParseInt(userid, 10, 64)
		user, _ := user_model.GetUserByID(ctx, id)

		buffer.WriteString(user.Name)
		if rem--; rem == 0 {
			break
		}
	}
	return buffer.String()
}

// GetMoreUserCount returns count of not shown users in reaction tooltip
func (commitComment *CommitComment) GetMoreUserCount(reaction string) int {
	if reaction == "" {
		return 0
	}
	reactions, err := commitComment.GroupReactionsByType()
	if err != nil {
		return 0
	}
	list := reactions[reaction]

	if len(list) <= setting.UI.ReactionMaxUserNum {
		return 0
	}
	return len(list) - setting.UI.ReactionMaxUserNum
}

func GetCommitCommentByID(ctx context.Context, repoID, ID int64) (*CommitComment, error) {
	return db.WithTx2(ctx, func(ctx context.Context) (*CommitComment, error) {
		commitComment := &CommitComment{
			RefRepoID: repoID,
			ID:        ID,
		}
		has, err := db.GetEngine(ctx).Get(commitComment)
		if err != nil {
			return nil, err
		} else if !has {
			return nil, err
		}
		err = commitComment.LoadRepo(ctx)
		if err != nil {
			return nil, err
		}
		err = commitComment.LoadPoster(ctx)
		if err != nil {
			return nil, err
		}
		return commitComment, err
	})
}

func GetCommitCommentBySHA(ctx context.Context, repoID int64, commitSHA string) (*CommitComment, error) {
	return db.WithTx2(ctx, func(ctx context.Context) (*CommitComment, error) {
		commitComment := &CommitComment{
			RefRepoID: repoID,
			CommitSHA: commitSHA,
		}
		has, err := db.GetEngine(ctx).Get(commitComment)
		if err != nil {
			return nil, err
		} else if !has {
			return nil, err
		}
		err = commitComment.LoadRepo(ctx)
		if err != nil {
			return nil, err
		}
		err = commitComment.LoadPoster(ctx)
		if err != nil {
			return nil, err
		}
		return commitComment, err
	})
}

// CreateCommitComment creates comment with context
func CreateCommitComment(ctx context.Context, opts *CreateCommitCommentOptions) (_ *CommitComment, err error) {
	return db.WithTx2(ctx, func(ctx context.Context) (*CommitComment, error) {
		reactions := make(ReactionMap)
		jsonBytes, err := json.Marshal(reactions)
		if err != nil {
			return nil, err
		}
		jsonString := string(jsonBytes)
		attachmentsJSONBytes, err := json.Marshal(opts.Attachments)
		if err != nil {
			return nil, err
		}
		attachmentsJSON := string(attachmentsJSONBytes)
		commitComment := &CommitComment{
			PosterID:         opts.Doer.ID,
			Poster:           opts.Doer,
			CommitSHA:        opts.CommitSHA,
			FileName:         opts.FileName,
			Line:             opts.LineNum,
			Comment:          opts.Comment,
			Reactions:        jsonString,
			Attachments:      attachmentsJSON,
			RefRepoID:        opts.RefRepoID,
			ReplyToCommentID: opts.ReplyToCommentID,
		}
		if err = db.Insert(ctx, commitComment); err != nil {
			return nil, err
		}
		return commitComment, err
	})
}

func UpdateCommitComment(ctx context.Context, attachmentMap *AttachmentMap, commitComment *CommitComment) (err error) {
	return db.WithTx(ctx, func(ctx context.Context) error {
		attachmentsJSONBytes, err := json.Marshal(attachmentMap)
		if err != nil {
			return err
		}
		attachmentsJSON := string(attachmentsJSONBytes)
		_, err = db.GetEngine(ctx).ID(commitComment.ID).Where("commit_sha = ?", commitComment.CommitSHA).Update(&CommitComment{
			PosterID:       commitComment.PosterID,
			Poster:         commitComment.Poster,
			CommitSHA:      commitComment.CommitSHA,
			FileName:       commitComment.FileName,
			Line:           commitComment.Line,
			Comment:        commitComment.Comment,
			ContentVersion: commitComment.ContentVersion,
			Attachments:    attachmentsJSON,
			RefRepoID:      commitComment.RefRepoID,
		})
		return err
	})
}

// DeleteComment deletes the comment
func DeleteCommitComment(ctx context.Context, commitComment *CommitComment) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		_, err := db.GetEngine(ctx).ID(commitComment.ID).NoAutoCondition().Delete(commitComment)
		return err
	})
}

func FindCommitCommentsByCommit(ctx context.Context, opts *FindCommitCommentOptions, commitComment *CommitComment) (CommitCommentList, error) {
	return db.WithTx2(ctx, func(ctx context.Context) (CommitCommentList, error) {
		var commitCommentList CommitCommentList
		sess := db.GetEngine(ctx).Where(opts.ToConds())

		if opts.CommitSHA == "" {
			return nil, nil
		}

		if opts.Page > 0 {
			sess = db.SetSessionPagination(sess, opts)
		}

		err := sess.Table(&CommitComment{}).Where(opts.ToConds()).Find(&commitCommentList)
		if err != nil {
			return nil, err
		}
		err = commitComment.LoadRepo(ctx)
		if err != nil {
			return nil, err
		}
		err = commitComment.LoadPoster(ctx)
		if err != nil {
			return nil, err
		}
		for _, cd := range commitCommentList {
			var err error
			rctx := renderhelper.NewRenderContextRepoComment(ctx, commitComment.Repo, renderhelper.RepoCommentOptions{
				FootnoteContextID: strconv.FormatInt(commitComment.ID, 10),
			})

			if cd.RenderedComment, err = markdown.RenderString(rctx, cd.Comment); err != nil {
				return nil, err
			}
			cd.Repo = commitComment.Repo
			cd.Poster = commitComment.Poster
		}
		return commitCommentList, nil
	})
}

func FindCommitCommentsByLine(ctx context.Context, opts *FindCommitCommentOptions, commitComment *CommitComment) (CommitCommentList, error) {
	return db.WithTx2(ctx, func(ctx context.Context) (CommitCommentList, error) {
		var commitCommentList CommitCommentList

		err := db.GetEngine(ctx).Table(&CommitComment{}).Where("commit_sha=? AND line=? ", opts.CommitSHA, opts.Line).Find(&commitCommentList)
		if err != nil {
			return nil, err
		}
		err = commitComment.LoadRepo(ctx)
		if err != nil {
			return nil, err
		}
		err = commitComment.LoadPoster(ctx)
		if err != nil {
			return nil, err
		}
		for _, cd := range commitCommentList {
			var err error
			rctx := renderhelper.NewRenderContextRepoComment(ctx, commitComment.Repo, renderhelper.RepoCommentOptions{
				FootnoteContextID: strconv.FormatInt(commitComment.ID, 10),
			})

			if cd.RenderedComment, err = markdown.RenderString(rctx, cd.Comment); err != nil {
				return nil, err
			}
			cd.Repo = commitComment.Repo
			cd.Poster = commitComment.Poster
		}
		return commitCommentList, nil
	})
}

func CreateCommitCommentReaction(ctx context.Context, reaction string, userID int64, commitComment *CommitComment) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		reactions := make(ReactionMap)

		err := json.Unmarshal([]byte(commitComment.Reactions), &reactions)
		if err != nil {
			return err
		}

		reactions[reaction] = append(reactions[reaction], strconv.FormatInt(userID, 10))

		jsonBytes, err := json.Marshal(reactions)
		if err != nil {
			return err
		}

		jsonString := string(jsonBytes)

		commitComment.Reactions = jsonString
		_, err = db.GetEngine(ctx).ID(commitComment.ID).Where("commit_sha = ?", commitComment.CommitSHA).Update(commitComment)
		return err
	})
}

func DeleteCommentReaction(ctx context.Context, reaction string, userID int64, commitComment *CommitComment) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		reactions := make(ReactionMap)
		err := json.Unmarshal([]byte(commitComment.Reactions), &reactions)
		if err != nil {
			return err
		}

		list := reactions[reaction]
		userid := strconv.FormatInt(userID, 10)
		idx := slices.Index(list, userid)
		reactions[reaction] = slices.Delete(list, idx, idx+1)

		for reactionType, userIDs := range reactions {
			if len(userIDs) == 0 {
				delete(reactions, reactionType) // Delete the key with an empty slice
			}
		}

		jsonBytes, err := json.Marshal(reactions)
		if err != nil {
			return err
		}

		jsonString := string(jsonBytes)

		commitComment.Reactions = jsonString
		_, err = db.GetEngine(ctx).ID(commitComment.ID).Where("commit_sha = ?", commitComment.CommitSHA).Update(commitComment)
		return err
	})
}
