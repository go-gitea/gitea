package statmiddleware

import (
	"strconv"
	"time"

	"github.com/google/uuid"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/messagequeue"
	"code.gitea.io/gitea/modules/setting"
)

// Constants for topics, types, and field names.
const (
	gitClone    = "git_clone"
	webDownload = "web_download"
)

type msgDetail struct {
	RepoId        string `json:"repo_id"`
	RepoName      string `json:"repo_name"`
	RepoOwner     string `json:"repo_owner"`
	DoerEmail     string `json:"doer_email"`
	RepoUserEmail string `json:"repo_user_email"`

	RequestIP    string `json:"request_IP"`
	RequestID    string `json:"request_ID"`
	RequestPath  string `json:"path"`
	RequestProto string `json:"request_proto"`
	RequestAgent string `json:"http_user_agent"`
}

type msgNormal struct {
	Type      string      `json:"type"`
	User      string      `json:"user"`
	Desc      string      `json:"desc"`
	Details   interface{} `json:"details"`
	CreatedAt int64       `json:"created_at"`
}

// Contexter is an interface to abstract the common functionality needed from the context.
type Contexter interface {
	GetDoerName() string
	GetDoerEmail() string
	GetRepoID() int64
	GetRepoName() string
	GetRepoOwnerName() string
	GetRepoOwnerEmail() string
	GetRequestRemoteAddr() string
	GetRequestUserAgent() string
	GetRequestProto() string
	GetRepoLink() string
}

// AdaptAPIContext adapts *context.APIContext to Contexter interface.
func AdaptAPIContext(apiCtx *context.APIContext) Contexter {
	return &apiContextAdapter{apiCtx}
}

// AdaptGitContext adapts *context.Context to Contexter interface.
func AdaptGitContext(gitCtx *context.Context) Contexter {
	return &gitContextAdapter{gitCtx}
}

// Implementations for adapting specific context types to the Contexter interface
type apiContextAdapter struct {
	*context.APIContext
}

func (a *apiContextAdapter) GetDoerName() string {
	if a.Doer == nil {
		return "anno"
	}
	return a.Doer.Name
}

func (a *apiContextAdapter) GetDoerEmail() string {
	if a.Doer == nil {
		return "anno"
	}
	return a.Doer.Email
}

// Other required methods for apiContextAdapter...

type gitContextAdapter struct {
	*context.Context
}

func (g *gitContextAdapter) GetDoerName() string {
	if g.Doer == nil {
		return "anno"
	}
	return g.Doer.Name
}

func (g *gitContextAdapter) GetDoerEmail() string {
	if g.Doer == nil {
		return "anno"
	}
	return g.Doer.Email
}

// Other required methods for gitContextAdapter...

// prepareMessage constructs the message to be sent.
func prepareMessage(ctx Contexter, messageType, description string) msgNormal {
	detail := msgDetail{
		RepoId:        strconv.FormatInt(ctx.GetRepoID(), 10),
		RepoName:      ctx.GetRepoName(),
		RepoOwner:     ctx.GetRepoOwnerName(),
		DoerEmail:     ctx.GetDoerEmail(),
		RepoUserEmail: ctx.GetRepoOwnerEmail(),

		RequestID:    uuid.New().String(),
		RequestIP:    ctx.GetRequestRemoteAddr(),
		RequestPath:  ctx.GetRepoLink(),
		RequestAgent: ctx.GetRequestUserAgent(),
		RequestProto: ctx.GetRequestProto(),
	}

	return msgNormal{
		Type:      messageType,
		User:      ctx.GetDoerName(),
		Desc:      description,
		Details:   detail,
		CreatedAt: time.Now().Unix(),
	}
}

// Implement the remaining required methods for apiContextAdapter
func (a *apiContextAdapter) GetRepoID() int64 {
	return a.Repo.Repository.ID
}

func (a *apiContextAdapter) GetRepoName() string {
	return a.Repo.Repository.Name
}

func (a *apiContextAdapter) GetRepoOwnerName() string {
	return a.Repo.Repository.OwnerName
}

func (a *apiContextAdapter) GetRepoOwnerEmail() string {
	return a.Repo.Repository.Owner.Email
}

func (a *apiContextAdapter) GetRequestRemoteAddr() string {
	return a.Req.RemoteAddr
}

func (a *apiContextAdapter) GetRequestUserAgent() string {
	return a.Req.UserAgent()
}

func (a *apiContextAdapter) GetRequestProto() string {
	return a.Req.Proto
}

func (a *apiContextAdapter) GetRepoLink() string {
	return a.Repo.RepoLink
}

// Implement the remaining required methods for gitContextAdapter
// Note: Assuming the Context struct has similar fields as APIContext, adjust if necessary.
func (g *gitContextAdapter) GetRepoID() int64 {
	return g.Repo.Repository.ID
}

func (g *gitContextAdapter) GetRepoName() string {
	return g.Repo.Repository.Name
}

func (g *gitContextAdapter) GetRepoOwnerName() string {
	return g.Repo.Repository.OwnerName
}

func (g *gitContextAdapter) GetRepoOwnerEmail() string {
	return g.Repo.Repository.Owner.Email
}

func (g *gitContextAdapter) GetRequestRemoteAddr() string {
	return g.Req.RemoteAddr
}

func (g *gitContextAdapter) GetRequestUserAgent() string {
	return g.Req.UserAgent()
}

func (g *gitContextAdapter) GetRequestProto() string {
	return g.Req.Proto
}

func (g *gitContextAdapter) GetRepoLink() string {
	return g.Repo.RepoLink
}

// Middleware functions using the adapted contexts and the unified prepareMessage function
func GitCloneMessageMiddleware(ctx *context.Context) {

	adaptedCtx := AdaptGitContext(ctx)
	msg := prepareMessage(adaptedCtx, gitClone, "this message means someone cloned the repository")

	if err := messagequeue.Publish(setting.MQ.TopicName, &msg, nil); err != nil {
		log.Info("internal error of kafka sender: %s", err.Error())
	}
}

func WebDownloadMiddleware(ctx *context.APIContext) {
	adaptedCtx := AdaptAPIContext(ctx)
	msg := prepareMessage(adaptedCtx, webDownload, "this message means someone downloaded a file from the website")

	if err := messagequeue.Publish(setting.MQ.TopicName, &msg, nil); err != nil {
		log.Info("internal error of kafka sender: %s", err.Error())
	}
}
