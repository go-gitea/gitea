// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

const matrixPayloadSizeLimit = 1024 * 64

// MatrixMeta contains the Matrix metadata
type MatrixMeta struct {
	HomeserverURL string `json:"homeserver_url"`
	Room          string `json:"room_id"`
	MessageType   int    `json:"message_type"`
}

var messageTypeText = map[int]string{
	1: "m.notice",
	2: "m.text",
}

// GetMatrixHook returns Matrix metadata
func GetMatrixHook(w *webhook_model.Webhook) *MatrixMeta {
	s := &MatrixMeta{}
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetMatrixHook(%d): %v", w.ID, err)
	}
	return s
}

var _ PayloadConvertor = &MatrixPayload{}

// MatrixPayload contains payload for a Matrix room
type MatrixPayload struct {
	Body          string               `json:"body"`
	MsgType       string               `json:"msgtype"`
	Format        string               `json:"format"`
	FormattedBody string               `json:"formatted_body"`
	Commits       []*api.PayloadCommit `json:"io.gitea.commits,omitempty"`
}

// JSONPayload Marshals the MatrixPayload to json
func (m *MatrixPayload) JSONPayload() ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

// MatrixLinkFormatter creates a link compatible with Matrix
func MatrixLinkFormatter(url, text string) string {
	return fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(url), html.EscapeString(text))
}

// MatrixLinkToRef Matrix-formatter link to a repo ref
func MatrixLinkToRef(repoURL, ref string) string {
	refName := git.RefName(ref).ShortName()
	switch {
	case strings.HasPrefix(ref, git.BranchPrefix):
		return MatrixLinkFormatter(repoURL+"/src/branch/"+util.PathEscapeSegments(refName), refName)
	case strings.HasPrefix(ref, git.TagPrefix):
		return MatrixLinkFormatter(repoURL+"/src/tag/"+util.PathEscapeSegments(refName), refName)
	default:
		return MatrixLinkFormatter(repoURL+"/src/commit/"+util.PathEscapeSegments(refName), refName)
	}
}

// Create implements PayloadConvertor Create method
func (m *MatrixPayload) Create(p *api.CreatePayload) (api.Payloader, error) {
	repoLink := MatrixLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	refLink := MatrixLinkToRef(p.Repo.HTMLURL, p.Ref)
	text := fmt.Sprintf("[%s:%s] %s created by %s", repoLink, refLink, p.RefType, p.Sender.UserName)

	return getMatrixPayload(text, nil, m.MsgType), nil
}

// Delete composes Matrix payload for delete a branch or tag.
func (m *MatrixPayload) Delete(p *api.DeletePayload) (api.Payloader, error) {
	refName := git.RefName(p.Ref).ShortName()
	repoLink := MatrixLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	text := fmt.Sprintf("[%s:%s] %s deleted by %s", repoLink, refName, p.RefType, p.Sender.UserName)

	return getMatrixPayload(text, nil, m.MsgType), nil
}

// Fork composes Matrix payload for forked by a repository.
func (m *MatrixPayload) Fork(p *api.ForkPayload) (api.Payloader, error) {
	baseLink := MatrixLinkFormatter(p.Forkee.HTMLURL, p.Forkee.FullName)
	forkLink := MatrixLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	text := fmt.Sprintf("%s is forked to %s", baseLink, forkLink)

	return getMatrixPayload(text, nil, m.MsgType), nil
}

// Issue implements PayloadConvertor Issue method
func (m *MatrixPayload) Issue(p *api.IssuePayload) (api.Payloader, error) {
	text, _, _, _ := getIssuesPayloadInfo(p, MatrixLinkFormatter, true)

	return getMatrixPayload(text, nil, m.MsgType), nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (m *MatrixPayload) IssueComment(p *api.IssueCommentPayload) (api.Payloader, error) {
	text, _, _ := getIssueCommentPayloadInfo(p, MatrixLinkFormatter, true)

	return getMatrixPayload(text, nil, m.MsgType), nil
}

// Wiki implements PayloadConvertor Wiki method
func (m *MatrixPayload) Wiki(p *api.WikiPayload) (api.Payloader, error) {
	text, _, _ := getWikiPayloadInfo(p, MatrixLinkFormatter, true)

	return getMatrixPayload(text, nil, m.MsgType), nil
}

// Release implements PayloadConvertor Release method
func (m *MatrixPayload) Release(p *api.ReleasePayload) (api.Payloader, error) {
	text, _ := getReleasePayloadInfo(p, MatrixLinkFormatter, true)

	return getMatrixPayload(text, nil, m.MsgType), nil
}

// Push implements PayloadConvertor Push method
func (m *MatrixPayload) Push(p *api.PushPayload) (api.Payloader, error) {
	var commitDesc string

	if p.TotalCommits == 1 {
		commitDesc = "1 commit"
	} else {
		commitDesc = fmt.Sprintf("%d commits", p.TotalCommits)
	}

	repoLink := MatrixLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	branchLink := MatrixLinkToRef(p.Repo.HTMLURL, p.Ref)
	text := fmt.Sprintf("[%s] %s pushed %s to %s:<br>", repoLink, p.Pusher.UserName, commitDesc, branchLink)

	// for each commit, generate a new line text
	for i, commit := range p.Commits {
		text += fmt.Sprintf("%s: %s - %s", MatrixLinkFormatter(commit.URL, commit.ID[:7]), commit.Message, commit.Author.Name)
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			text += "<br>"
		}

	}

	return getMatrixPayload(text, p.Commits, m.MsgType), nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (m *MatrixPayload) PullRequest(p *api.PullRequestPayload) (api.Payloader, error) {
	text, _, _, _ := getPullRequestPayloadInfo(p, MatrixLinkFormatter, true)

	return getMatrixPayload(text, nil, m.MsgType), nil
}

// Review implements PayloadConvertor Review method
func (m *MatrixPayload) Review(p *api.PullRequestPayload, event webhook_module.HookEventType) (api.Payloader, error) {
	senderLink := MatrixLinkFormatter(setting.AppURL+url.PathEscape(p.Sender.UserName), p.Sender.UserName)
	title := fmt.Sprintf("#%d %s", p.Index, p.PullRequest.Title)
	titleLink := MatrixLinkFormatter(p.PullRequest.HTMLURL, title)
	repoLink := MatrixLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	var text string

	switch p.Action {
	case api.HookIssueReviewed:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return nil, err
		}

		text = fmt.Sprintf("[%s] Pull request review %s: %s by %s", repoLink, action, titleLink, senderLink)
	}

	return getMatrixPayload(text, nil, m.MsgType), nil
}

// Repository implements PayloadConvertor Repository method
func (m *MatrixPayload) Repository(p *api.RepositoryPayload) (api.Payloader, error) {
	senderLink := MatrixLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	repoLink := MatrixLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	var text string

	switch p.Action {
	case api.HookRepoCreated:
		text = fmt.Sprintf("[%s] Repository created by %s", repoLink, senderLink)
	case api.HookRepoDeleted:
		text = fmt.Sprintf("[%s] Repository deleted by %s", repoLink, senderLink)
	}

	return getMatrixPayload(text, nil, m.MsgType), nil
}

func (m *MatrixPayload) Package(p *api.PackagePayload) (api.Payloader, error) {
	senderLink := MatrixLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	packageLink := MatrixLinkFormatter(p.Package.HTMLURL, p.Package.Name)
	var text string

	switch p.Action {
	case api.HookPackageCreated:
		text = fmt.Sprintf("[%s] Package published by %s", packageLink, senderLink)
	case api.HookPackageDeleted:
		text = fmt.Sprintf("[%s] Package deleted by %s", packageLink, senderLink)
	}

	return getMatrixPayload(text, nil, m.MsgType), nil
}

// GetMatrixPayload converts a Matrix webhook into a MatrixPayload
func GetMatrixPayload(p api.Payloader, event webhook_module.HookEventType, meta string) (api.Payloader, error) {
	s := new(MatrixPayload)

	matrix := &MatrixMeta{}
	if err := json.Unmarshal([]byte(meta), &matrix); err != nil {
		return s, errors.New("GetMatrixPayload meta json:" + err.Error())
	}

	s.MsgType = messageTypeText[matrix.MessageType]

	return convertPayloader(s, p, event)
}

func getMatrixPayload(text string, commits []*api.PayloadCommit, msgType string) *MatrixPayload {
	p := MatrixPayload{}
	p.FormattedBody = text
	p.Body = getMessageBody(text)
	p.Format = "org.matrix.custom.html"
	p.MsgType = msgType
	p.Commits = commits
	return &p
}

var urlRegex = regexp.MustCompile(`<a [^>]*?href="([^">]*?)">(.*?)</a>`)

func getMessageBody(htmlText string) string {
	htmlText = urlRegex.ReplaceAllString(htmlText, "[$2]($1)")
	htmlText = strings.ReplaceAll(htmlText, "<br>", "\n")
	return htmlText
}

// getMatrixTxnID computes the transaction ID to ensure idempotency
func getMatrixTxnID(payload []byte) (string, error) {
	if len(payload) >= matrixPayloadSizeLimit {
		return "", fmt.Errorf("getMatrixTxnID: payload size %d > %d", len(payload), matrixPayloadSizeLimit)
	}

	h := sha1.New()
	_, err := h.Write(payload)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
