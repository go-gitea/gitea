// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

func init() {
	RegisterWebhookRequester(webhook_module.MATRIX, newMatrixRequest)
}

func newMatrixRequest(_ context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	meta := &MatrixMeta{}
	if err := json.Unmarshal([]byte(w.Meta), meta); err != nil {
		return nil, nil, fmt.Errorf("GetMatrixPayload meta json: %w", err)
	}
	var pc payloadConvertor[MatrixPayload] = matrixConvertor{
		MsgType: messageTypeText[meta.MessageType],
	}
	payload, err := newPayload(pc, []byte(t.PayloadContent), t.EventType)
	if err != nil {
		return nil, nil, err
	}

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, nil, err
	}

	txnID, err := getMatrixTxnID(body)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequest(http.MethodPut, w.URL+"/"+txnID, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return req, body, addDefaultHeaders(req, []byte(w.Secret), w, t, body) // likely useless, but has always been sent historially
}

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

// MatrixPayload contains payload for a Matrix room
type MatrixPayload struct {
	Body          string               `json:"body"`
	MsgType       string               `json:"msgtype"`
	Format        string               `json:"format"`
	FormattedBody string               `json:"formatted_body"`
	Commits       []*api.PayloadCommit `json:"io.gitea.commits,omitempty"`
}

type matrixConvertor struct {
	MsgType string
}

func (m matrixConvertor) newPayload(text string, commits ...*api.PayloadCommit) (MatrixPayload, error) {
	return MatrixPayload{
		Body:          getMessageBody(text),
		MsgType:       m.MsgType,
		Format:        "org.matrix.custom.html",
		FormattedBody: text,
		Commits:       commits,
	}, nil
}

// Create implements payloadConvertor Create method
func (m matrixConvertor) Create(p *api.CreatePayload) (MatrixPayload, error) {
	repoLink := htmlLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	refLink := MatrixLinkToRef(p.Repo.HTMLURL, p.Ref)
	text := fmt.Sprintf("[%s:%s] %s created by %s", repoLink, refLink, p.RefType, p.Sender.UserName)

	return m.newPayload(text)
}

// Delete composes Matrix payload for delete a branch or tag.
func (m matrixConvertor) Delete(p *api.DeletePayload) (MatrixPayload, error) {
	refName := git.RefName(p.Ref).ShortName()
	repoLink := htmlLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	text := fmt.Sprintf("[%s:%s] %s deleted by %s", repoLink, refName, p.RefType, p.Sender.UserName)

	return m.newPayload(text)
}

// Fork composes Matrix payload for forked by a repository.
func (m matrixConvertor) Fork(p *api.ForkPayload) (MatrixPayload, error) {
	baseLink := htmlLinkFormatter(p.Forkee.HTMLURL, p.Forkee.FullName)
	forkLink := htmlLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	text := fmt.Sprintf("%s is forked to %s", baseLink, forkLink)

	return m.newPayload(text)
}

// Issue implements payloadConvertor Issue method
func (m matrixConvertor) Issue(p *api.IssuePayload) (MatrixPayload, error) {
	text, _, _, _ := getIssuesPayloadInfo(p, htmlLinkFormatter, true)

	return m.newPayload(text)
}

// IssueComment implements payloadConvertor IssueComment method
func (m matrixConvertor) IssueComment(p *api.IssueCommentPayload) (MatrixPayload, error) {
	text, _, _ := getIssueCommentPayloadInfo(p, htmlLinkFormatter, true)

	return m.newPayload(text)
}

// Wiki implements payloadConvertor Wiki method
func (m matrixConvertor) Wiki(p *api.WikiPayload) (MatrixPayload, error) {
	text, _, _ := getWikiPayloadInfo(p, htmlLinkFormatter, true)

	return m.newPayload(text)
}

// Release implements payloadConvertor Release method
func (m matrixConvertor) Release(p *api.ReleasePayload) (MatrixPayload, error) {
	text, _ := getReleasePayloadInfo(p, htmlLinkFormatter, true)

	return m.newPayload(text)
}

// Push implements payloadConvertor Push method
func (m matrixConvertor) Push(p *api.PushPayload) (MatrixPayload, error) {
	var commitDesc string

	if p.TotalCommits == 1 {
		commitDesc = "1 commit"
	} else {
		commitDesc = fmt.Sprintf("%d commits", p.TotalCommits)
	}

	repoLink := htmlLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	branchLink := MatrixLinkToRef(p.Repo.HTMLURL, p.Ref)
	text := fmt.Sprintf("[%s] %s pushed %s to %s:<br>", repoLink, p.Pusher.UserName, commitDesc, branchLink)

	// for each commit, generate a new line text
	for i, commit := range p.Commits {
		text += fmt.Sprintf("%s: %s - %s", htmlLinkFormatter(commit.URL, commit.ID[:7]), commit.Message, commit.Author.Name)
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			text += "<br>"
		}
	}

	return m.newPayload(text, p.Commits...)
}

// PullRequest implements payloadConvertor PullRequest method
func (m matrixConvertor) PullRequest(p *api.PullRequestPayload) (MatrixPayload, error) {
	text, _, _, _ := getPullRequestPayloadInfo(p, htmlLinkFormatter, true)

	return m.newPayload(text)
}

// Review implements payloadConvertor Review method
func (m matrixConvertor) Review(p *api.PullRequestPayload, event webhook_module.HookEventType) (MatrixPayload, error) {
	senderLink := htmlLinkFormatter(setting.AppURL+url.PathEscape(p.Sender.UserName), p.Sender.UserName)
	title := fmt.Sprintf("#%d %s", p.Index, p.PullRequest.Title)
	titleLink := htmlLinkFormatter(p.PullRequest.HTMLURL, title)
	repoLink := htmlLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	var text string

	switch p.Action {
	case api.HookIssueReviewed:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return MatrixPayload{}, err
		}

		text = fmt.Sprintf("[%s] Pull request review %s: %s by %s", repoLink, action, titleLink, senderLink)
	}

	return m.newPayload(text)
}

// Repository implements payloadConvertor Repository method
func (m matrixConvertor) Repository(p *api.RepositoryPayload) (MatrixPayload, error) {
	senderLink := htmlLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	repoLink := htmlLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	var text string

	switch p.Action {
	case api.HookRepoCreated:
		text = fmt.Sprintf("[%s] Repository created by %s", repoLink, senderLink)
	case api.HookRepoDeleted:
		text = fmt.Sprintf("[%s] Repository deleted by %s", repoLink, senderLink)
	}
	return m.newPayload(text)
}

func (m matrixConvertor) Package(p *api.PackagePayload) (MatrixPayload, error) {
	senderLink := htmlLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	packageLink := htmlLinkFormatter(p.Package.HTMLURL, p.Package.Name)
	var text string

	switch p.Action {
	case api.HookPackageCreated:
		text = fmt.Sprintf("[%s] Package published by %s", packageLink, senderLink)
	case api.HookPackageDeleted:
		text = fmt.Sprintf("[%s] Package deleted by %s", packageLink, senderLink)
	}

	return m.newPayload(text)
}

func (m matrixConvertor) Status(p *api.CommitStatusPayload) (MatrixPayload, error) {
	refLink := htmlLinkFormatter(p.TargetURL, fmt.Sprintf("%s [%s]", p.Context, base.ShortSha(p.SHA)))
	text := fmt.Sprintf("Commit Status changed: %s - %s", refLink, p.Description)

	return m.newPayload(text)
}

func (m matrixConvertor) WorkflowJob(p *api.WorkflowJobPayload) (MatrixPayload, error) {
	text, _ := getWorkflowJobPayloadInfo(p, htmlLinkFormatter, true)

	return m.newPayload(text)
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

// MatrixLinkToRef Matrix-formatter link to a repo ref
func MatrixLinkToRef(repoURL, ref string) string {
	refName := git.RefName(ref).ShortName()
	switch {
	case strings.HasPrefix(ref, git.BranchPrefix):
		return htmlLinkFormatter(repoURL+"/src/branch/"+util.PathEscapeSegments(refName), refName)
	case strings.HasPrefix(ref, git.TagPrefix):
		return htmlLinkFormatter(repoURL+"/src/tag/"+util.PathEscapeSegments(refName), refName)
	default:
		return htmlLinkFormatter(repoURL+"/src/commit/"+util.PathEscapeSegments(refName), refName)
	}
}
