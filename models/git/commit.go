// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"slices"
	"strconv"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/timeutil"
	"github.com/google/uuid"
)

type (
	CommitDataList []*CommitData
	ReactionMap    map[string][]string
	AttachmentMap  map[string]*AttachmentOptions
)

type CommitData struct {
	ID              int64            `xorm:"pk autoincr"`
	PosterID        int64            `xorm:"INDEX"`
	Poster          *user_model.User `xorm:"-"`
	Line            int64
	FileName        string
	CommitSHA       string                 `xorm:"VARCHAR(64)"`
	Attachments     string                 `xorm:"JSON TEXT"`
	Reactions       string                 `xorm:"JSON TEXT"`
	Comment         string                 `xorm:"LONGTEXT"`
	RenderedComment template.HTML          `xorm:"-"`
	ContentVersion  int                    `xorm:"NOT NULL DEFAULT 0"`
	RefRepoID       int64                  `xorm:"index"`
	Repo            *repo_model.Repository `xorm:"-"`
	CreatedUnix     timeutil.TimeStamp     `xorm:"INDEX created"`
	UpdatedUnix     timeutil.TimeStamp     `xorm:"INDEX updated"`
}

type CreateCommitDataOptions struct {
	Doer *user_model.User
	Repo *repo_model.Repository

	CommitID    int64
	CommitSHA   string
	LineNum     int64
	Reactions   string
	Comment     string
	FileName    string
	Attachments AttachmentMap
	RefRepoID   int64
}

type AttachmentOptions struct {
	FileName   string
	Size       int64
	UploaderID int64
}

type FindCommitDataOptions struct {
	db.ListOptions
	RepoID    int64
	CommitSHA string
	Since     int64
	Before    int64
	Line      int64
	FileName  string
}

func init() {
	db.RegisterModel(new(CommitData))
}

// HashTag returns unique hash tag for commitData.
func (commitData *CommitData) HashTag() string {
	return fmt.Sprintf("commitdata-%d", commitData.ID)
}

func (commitData *CommitData) LoadRepo(ctx context.Context) (err error) {
	if commitData.Repo == nil && commitData.RefRepoID != 0 {
		commitData.Repo, err = repo_model.GetRepositoryByID(ctx, commitData.RefRepoID)
		if err != nil {
			return fmt.Errorf("getRepositoryByID [%d]: %w", commitData.RefRepoID, err)
		}
	}
	return nil
}

// LoadPoster loads poster
func (commitData *CommitData) LoadPoster(ctx context.Context) (err error) {
	if commitData.Poster == nil && commitData.PosterID != 0 {
		commitData.Poster, err = user_model.GetPossibleUserByID(ctx, commitData.PosterID)
		if err != nil {
			commitData.PosterID = user_model.GhostUserID
			commitData.Poster = user_model.NewGhostUser()
			if !user_model.IsErrUserNotExist(err) {
				return fmt.Errorf("getUserByID.(poster) [%d]: %w", commitData.PosterID, err)
			}
			return nil
		}
	}
	return err
}

func (commitData *CommitData) UnsignedLine() uint64 {
	if commitData.Line < 0 {
		return uint64(commitData.Line * -1)
	}
	return uint64(commitData.Line)
}

func (commitData *CommitData) TreePath() string {
	return commitData.FileName
}

// DiffSide returns "previous" if Comment.Line is a LOC of the previous changes and "proposed" if it is a LOC of the proposed changes.
func (commitData *CommitData) DiffSide() string {
	if commitData.Line < 0 {
		return "previous"
	}
	return "proposed"
}

func (commitData *CommitData) GroupReactionsByType() (ReactionMap, error) {
	reactions := make(ReactionMap)

	err := json.Unmarshal([]byte(commitData.Reactions), &reactions)
	if err != nil {
		return nil, errors.New("GroupReactionsByType")
	}
	return reactions, nil
}

func (commitData *CommitData) GroupAttachmentsByUUID() (AttachmentMap, error) {
	attachmentMap := make(AttachmentMap)
	err := json.Unmarshal([]byte(commitData.Attachments), &attachmentMap)
	if err != nil {
		return nil, err
	}
	return attachmentMap, nil
}

// HasUser check if user has reacted
func (commitData *CommitData) HasUser(reaction string, userID int64) bool {
	if userID == 0 {
		return false
	}
	reactions, err := commitData.GroupReactionsByType()
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
func (commitData *CommitData) GetFirstUsers(ctx context.Context, reaction string) string {
	var buffer bytes.Buffer
	rem := setting.UI.ReactionMaxUserNum
	reactions, err := commitData.GroupReactionsByType()
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
func (commitData *CommitData) GetMoreUserCount(reaction string) int {
	if reaction == "" {
		return 0
	}
	reactions, err := commitData.GroupReactionsByType()
	if err != nil {
		return 0
	}
	list := reactions[reaction]

	if len(list) <= setting.UI.ReactionMaxUserNum {
		return 0
	}
	return len(list) - setting.UI.ReactionMaxUserNum
}

func GetCommitDataByID(ctx context.Context, repoID, ID int64) (*CommitData, error) {
	commitData := &CommitData{
		RefRepoID: repoID,
		ID:        ID,
	}
	has, err := db.GetEngine(ctx).Get(commitData)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, err
	}
	err = commitData.LoadRepo(ctx)
	if err != nil {
		return nil, err
	}
	err = commitData.LoadPoster(ctx)
	if err != nil {
		return nil, err
	}
	return commitData, err
}

func GetCommitDataBySHA(ctx context.Context, repoID int64, commitSHA string) (*CommitData, error) {
	commitData := &CommitData{
		RefRepoID: repoID,
		CommitSHA: commitSHA,
	}
	has, err := db.GetEngine(ctx).Get(commitData)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, err
	}
	err = commitData.LoadRepo(ctx)
	if err != nil {
		return nil, err
	}
	err = commitData.LoadPoster(ctx)
	if err != nil {
		return nil, err
	}
	return commitData, err
}

// CreateCommitComment creates comment with context
func CreateCommitData(ctx context.Context, opts *CreateCommitDataOptions) (_ *CommitData, err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	e := db.GetEngine(ctx)
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

	commit := &CommitData{
		PosterID:    opts.Doer.ID,
		Poster:      opts.Doer,
		CommitSHA:   opts.CommitSHA,
		FileName:    opts.FileName,
		Line:        opts.LineNum,
		Comment:     opts.Comment,
		Reactions:   jsonString,
		Attachments: attachmentsJSON,
		RefRepoID:   opts.RefRepoID,
	}
	if _, err = e.Insert(commit); err != nil {
		return nil, err
	}

	if err = committer.Commit(); err != nil {
		return nil, err
	}
	return commit, nil
}

func UpdateCommitData(ctx context.Context, attachmentMap *AttachmentMap, commitData *CommitData) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	attachmentsJSONBytes, err := json.Marshal(attachmentMap)
	if err != nil {
		return err
	}

	attachmentsJSON := string(attachmentsJSONBytes)

	commit := &CommitData{
		PosterID:       commitData.PosterID,
		Poster:         commitData.Poster,
		CommitSHA:      commitData.CommitSHA,
		FileName:       commitData.FileName,
		Line:           commitData.Line,
		Comment:        commitData.Comment,
		ContentVersion: commitData.ContentVersion,
		Attachments:    attachmentsJSON,
		RefRepoID:      commitData.RefRepoID,
	}

	sess := db.GetEngine(ctx)
	_, err = sess.ID(commitData.ID).Where("commit_sha = ?", commitData.CommitSHA).Update(commit)
	if err != nil {
		return err
	}
	err = committer.Commit()
	return err
}

// DeleteComment deletes the comment
func DeleteCommitComment(ctx context.Context, commitData *CommitData) error {
	e := db.GetEngine(ctx)
	if _, err := e.ID(commitData.ID).NoAutoCondition().Delete(commitData); err != nil {
		return err
	}
	return nil
}

func FindCommitCommentsByCommit(ctx context.Context, opts *FindCommitDataOptions, commitData *CommitData) (CommitDataList, error) {
	var commitDataList CommitDataList
	sess := db.GetEngine(ctx).Where(opts.ToConds())

	if opts.CommitSHA == "" {
		return nil, nil
	}

	if opts.Page > 0 {
		sess = db.SetSessionPagination(sess, opts)
	}

	err := sess.Table("commit_data").Where(opts.ToConds()).Find(&commitDataList)
	if err != nil {
		return nil, err
	}
	err = commitData.LoadRepo(ctx)
	if err != nil {
		return nil, err
	}
	err = commitData.LoadPoster(ctx)
	if err != nil {
		return nil, err
	}
	for _, cd := range commitDataList {
		var err error
		rctx := renderhelper.NewRenderContextRepoComment(ctx, commitData.Repo, renderhelper.RepoCommentOptions{
			FootnoteContextID: strconv.FormatInt(commitData.ID, 10),
		})

		if cd.RenderedComment, err = markdown.RenderString(rctx, cd.Comment); err != nil {
			return nil, err
		}
		cd.Repo = commitData.Repo
		cd.Poster = commitData.Poster
	}
	return commitDataList, nil
}

func FindCommitCommentsByLine(ctx context.Context, opts *FindCommitDataOptions, commitData *CommitData) (CommitDataList, error) {
	var commitDataList CommitDataList

	sess := db.GetEngine(ctx)

	err := sess.Table("commit_data").Where("commit_sha=? AND line=? ", opts.CommitSHA, opts.Line).Find(&commitDataList)
	if err != nil {
		return nil, err
	}
	err = commitData.LoadRepo(ctx)
	if err != nil {
		return nil, err
	}
	err = commitData.LoadPoster(ctx)
	if err != nil {
		return nil, err
	}
	for _, cd := range commitDataList {
		var err error
		rctx := renderhelper.NewRenderContextRepoComment(ctx, commitData.Repo, renderhelper.RepoCommentOptions{
			FootnoteContextID: strconv.FormatInt(commitData.ID, 10),
		})

		if cd.RenderedComment, err = markdown.RenderString(rctx, cd.Comment); err != nil {
			return nil, err
		}
		cd.Repo = commitData.Repo
		cd.Poster = commitData.Poster
	}
	return commitDataList, nil
}

func CreateCommitCommentReaction(ctx context.Context, reaction string, userID int64, commitData *CommitData) error {
	if !setting.UI.ReactionsLookup.Contains(reaction) {
		return nil
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}

	reactions := make(ReactionMap)

	err = json.Unmarshal([]byte(commitData.Reactions), &reactions)
	if err != nil {
		return err
	}

	reactions[reaction] = append(reactions[reaction], strconv.FormatInt(userID, 10))

	jsonBytes, err := json.Marshal(reactions)
	if err != nil {
		return err
	}

	jsonString := string(jsonBytes)

	commitData.Reactions = jsonString
	sess := db.GetEngine(ctx)
	_, err = sess.ID(commitData.ID).Where("commit_sha = ?", commitData.CommitSHA).Update(commitData)
	if err != nil {
		return err
	}

	err = committer.Commit()
	if err != nil {
		return err
	}

	return nil
}

func DeleteCommentReaction(ctx context.Context, reaction string, userID int64, commitData *CommitData) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}

	reactions := make(ReactionMap)

	err = json.Unmarshal([]byte(commitData.Reactions), &reactions)
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

	commitData.Reactions = jsonString
	sess := db.GetEngine(ctx)
	_, err = sess.ID(commitData.ID).Where("commit_sha = ?", commitData.CommitSHA).Update(commitData)
	if err != nil {
		return err
	}

	err = committer.Commit()
	if err != nil {
		return err
	}

	return nil
}

func SaveTemporaryAttachment(ctx context.Context, file io.Reader, opts *AttachmentOptions) (string, error) {
	attachmentUUID := uuid.New().String()
	_, err := storage.Attachments.Save(attachmentUUID, file, opts.Size)
	return attachmentUUID, err
}

func UploadCommitAttachment(ctx context.Context, file io.Reader, commitData *CommitData, opts *AttachmentOptions) error {
	attachmentUUID := uuid.New().String()

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}

	attachment := make(AttachmentMap)
	attachment[attachmentUUID] = opts

	jsonBytes, err := json.Marshal(attachment)
	if err != nil {
		return err
	}
	jsonString := string(jsonBytes)
	commitData.Attachments = jsonString

	sess := db.GetEngine(ctx)
	_, err = sess.ID(commitData.ID).Where("commit_sha = ?", commitData.CommitSHA).Update(commitData)
	if err != nil {
		return err
	}

	err = committer.Commit()
	if err != nil {
		return err
	}
	return err
}
