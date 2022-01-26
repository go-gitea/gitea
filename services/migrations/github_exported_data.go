// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"

	"github.com/hashicorp/go-version"
)

/*
	{"version":"1.0.1","github_sha":"8de0984858fd99a8dcd2d756cf0f128b9161e3b5"}
*/
type githubSchema struct {
	Version   string
	GithubSha string `json:"github_sha"`
}

/*
{
    "type": "user",
    "url": "https://github.com/sunvim",
    "avatar_url": "https://avatars.githubusercontent.com/u/859692?v=4",
    "login": "sunvim",
    "name": "mobus",
    "bio": "code happy ",
    "company": "Ankr",
    "website": null,
    "location": "Shanghai",
    "emails": [
      {
        "address": "sv0220@163.com",
        "primary": true,
        "verified": true
      }
    ],
    "billing_plan": null,
    "created_at": "2011-06-19T11:25:35Z"
  },
*/
type githubUser struct {
	URL       string
	AvatarURL string `json:"avatar_url"`
	Login     string
	Name      string
	Bio       string
	Company   string
	Website   string
	Location  string
	Emails    []struct {
		Address  string
		Primary  bool
		Verified bool
	}
	CreatedAt time.Time `json:"created_at"`
}

func getURLLastField(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		log.Error("parse %s failed: %v", s, err)
		return ""
	}
	fields := strings.Split(u.Path, "/")
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

func parseGitHubResID(s string) int64 {
	u, err := url.Parse(s)
	if err != nil {
		log.Error("parse %s failed: %v", s, err)
		return 0
	}
	fields := strings.Split(u.Path, "/")
	if len(fields) == 0 {
		return 0
	}
	i, _ := strconv.ParseInt(fields[len(fields)-1], 10, 64)
	return i
}

func (g *githubUser) ID() int64 {
	return parseGitHubResID(g.AvatarURL)
}

func (g *githubUser) Email() string {
	if len(g.Emails) < 1 {
		return ""
	}

	for _, e := range g.Emails {
		if e.Primary {
			return e.Address
		}
	}
	return ""
}

/*{
    "type": "attachment",
    "url": "https://user-images.githubusercontent.com/1595118/2923824-63a167ce-d721-11e3-91b6-74b83dc345bb.png",
		"issue": "https://github.com/go-xorm/xorm/issues/205",
    "issue_comment": "https://github.com/go-xorm/xorm/issues/115#issuecomment-42628488",
    "user": "https://github.com/mintzhao",
    "asset_name": "QQ20140509-1.2x.png",
    "asset_content_type": "image/png",
    "asset_url": "tarball://root/attachments/63a167ce-d721-11e3-91b6-74b83dc345bb/QQ20140509-1.2x.png",
    "created_at": "2014-05-09T02:38:54Z"
  },
*/
type githubAttachment struct {
	Issue            string
	IssueComment     string `json:"issue_comment"`
	User             string
	AssetName        string    `json:"asset_name"`
	AssetContentType string    `json:"asset_content_type"`
	AssetURL         string    `json:"asset_url"`
	CreatedAt        time.Time `json:"created_at"`
}

func (g *githubAttachment) GetUserID() int64 {
	return parseGitHubResID(g.User)
}

func (g *githubAttachment) IsIssue() bool {
	return len(g.Issue) > 0
}

func (g *githubAttachment) IssueID() int64 {
	if g.IsIssue() {
		return parseGitHubResID(g.Issue)
	}
	return parseGitHubResID(g.IssueComment)
}

func (r *GithubExportedDataRestorer) convertAttachments(ls []githubAttachment) []*base.Asset {
	res := make([]*base.Asset, 0, len(ls))
	for _, l := range ls {
		fPath := strings.TrimPrefix(l.AssetURL, "tarball://root")
		info, err := os.Stat(fPath)
		var size int
		if err == nil {
			size = int(info.Size())
		}
		assetURL := "file://" + fPath
		res = append(res, &base.Asset{
			Name:        l.AssetName,
			ContentType: &l.AssetContentType,
			Size:        &size,
			Created:     l.CreatedAt,
			DownloadURL: &assetURL,
		})
	}
	return res
}

/*
{
        "user": "https://github.com/mrsdizzie",
        "content": "+1",
        "subject_type": "Issue",
        "created_at": "2019-11-13T04:22:13.000+08:00"
      }*/
type githubReaction struct {
	User        string
	Content     string
	SubjectType string    `json:"subject_type"`
	CreatedAt   time.Time `json:"created_at"`
}

type githubLabel string

func (l githubLabel) GetName() string {
	fields := strings.Split(string(l), "/labels/")
	if len(fields) == 0 {
		return ""
	}
	s, err := url.PathUnescape(fields[len(fields)-1])
	if err != nil {
		log.Error("url.PathUnescape %s failed: %v", fields[len(fields)-1], err)
		return fields[len(fields)-1]
	}
	return s
}

// GithubExportedDataRestorer implements an Downloader from the exported data of Github
type GithubExportedDataRestorer struct {
	base.NullDownloader
	ctx                context.Context
	tmpDir             string
	githubDataFilePath string
	repoOwner          string
	repoName           string
	baseURL            string
	regMatchIssue      *regexp.Regexp
	regMatchCommit     *regexp.Regexp
	labels             []*base.Label
	users              map[string]githubUser
	issueAttachments   map[string][]githubAttachment
	commentAttachments map[string][]githubAttachment
	milestones         map[string]githubMilestone
	attachmentLoaded   bool
}

var _ base.Downloader = &GithubExportedDataRestorer{}

func decompressFile(targzFile, targetDir string) error {
	f, err := os.Open(targzFile)
	if err != nil {
		return err
	}
	defer f.Close()
	uncompressedStream, err := gzip.NewReader(f)
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(uncompressedStream)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(filepath.Join(targetDir, header.Name), os.ModePerm); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(filepath.Join(targetDir, header.Name))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		default:
			return fmt.Errorf("decompressFile: uknown type: %d in %s",
				header.Typeflag,
				header.Name)
		}
	}
	return nil
}

// NewGithubExportedDataRestorer creates a repository restorer which could restore repository from a github exported data
func NewGithubExportedDataRestorer(ctx context.Context, githubDataFilePath, owner, repoName string) (*GithubExportedDataRestorer, error) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "github_exported_data")
	if err != nil {
		return nil, err
	}
	// uncompress the file
	if err := decompressFile(githubDataFilePath, tmpDir); err != nil {
		return nil, err
	}

	restorer := &GithubExportedDataRestorer{
		ctx:                ctx,
		githubDataFilePath: githubDataFilePath,
		tmpDir:             tmpDir,
		repoOwner:          owner,
		repoName:           repoName,
		users:              make(map[string]githubUser),
		milestones:         make(map[string]githubMilestone),
		issueAttachments:   make(map[string][]githubAttachment),
		commentAttachments: make(map[string][]githubAttachment),
	}
	if err := restorer.readSchema(); err != nil {
		return nil, err
	}
	if err := restorer.getUsers(); err != nil {
		return nil, err
	}

	return restorer, nil
}

// CleanUp clean the downloader temporary resources
func (r *GithubExportedDataRestorer) CleanUp() {
	if r.tmpDir != "" {
		_ = os.RemoveAll(r.tmpDir)
	}
}

// replaceComment replace #id to new form
// i.e.
// 1) https://github.com/userstyles-world/userstyles.world/commit/b70d545a1cbb5c92ca20f442f59de5d955600408 -> b70d545a1cbb5c92ca20f442f59de5d955600408
// 2) https://github.com/go-gitea/gitea/issue/1 -> #1
// 3) https://github.com/go-gitea/gitea/pull/2 -> #2
func (r *GithubExportedDataRestorer) replaceGithubLinks(content string) string {
	c := r.regMatchIssue.ReplaceAllString(content, "#$2")
	c = r.regMatchCommit.ReplaceAllString(c, "$1")
	return c
}

// SupportGetRepoComments return true if it can get all comments once
func (r *GithubExportedDataRestorer) SupportGetRepoComments() bool {
	return true
}

// SupportGetRepoComments return true if it can get all comments once
func (r *GithubExportedDataRestorer) SupportGetRepoReviews() bool {
	return true
}

// SetContext set context
func (r *GithubExportedDataRestorer) SetContext(ctx context.Context) {
	r.ctx = ctx
}

func (r *GithubExportedDataRestorer) readSchema() error {
	bs, err := os.ReadFile(filepath.Join(r.tmpDir, "schema.json"))
	if err != nil {
		return err
	}
	var schema githubSchema
	if err := json.Unmarshal(bs, &schema); err != nil {
		return err
	}

	v, err := version.NewSemver(schema.Version)
	if err != nil {
		return fmt.Errorf("archive version %s is not semver", schema.Version)
	}
	s := v.Segments()
	if s[0] != 1 {
		return fmt.Errorf("archive version is %s, but expected 1.x.x", schema.Version)
	}
	return nil
}

// GetRepoInfo returns a repository information
func (r *GithubExportedDataRestorer) GetRepoInfo() (*base.Repository, error) {
	type Label struct {
		URL         string
		Name        string
		Color       string
		Description string
		CreatedAt   time.Time `json:"created_at"`
	}
	type GithubRepo struct {
		Name          string
		URL           string
		Owner         string
		Description   string
		Private       bool
		Labels        []Label
		CreatedAt     time.Time `json:"created_at"`
		DefaultBranch string    `json:"default_branch"`
	}

	p := filepath.Join(r.tmpDir, "repositories_000001.json")
	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	var githubRepositories []GithubRepo
	if err := json.Unmarshal(bs, &githubRepositories); err != nil {
		return nil, err
	}
	if len(githubRepositories) == 0 {
		return nil, errors.New("no repository found in the json file: repositories_000001.json")
	} else if len(githubRepositories) > 1 {
		return nil, errors.New("only one repository is supported")
	}

	opts := githubRepositories[0]
	fields := strings.Split(opts.Owner, "/")
	owner := fields[len(fields)-1]

	for _, label := range opts.Labels {
		r.labels = append(r.labels, &base.Label{
			Name:        label.Name,
			Color:       label.Color,
			Description: label.Description,
		})
	}
	r.baseURL = opts.URL
	r.regMatchIssue, err = regexp.Compile(r.baseURL + "/(issue|pull)/([0-9]+)")
	if err != nil {
		return nil, err
	}
	r.regMatchCommit, err = regexp.Compile(r.baseURL + "/commit/([a-z0-9]{7, 40})")
	if err != nil {
		return nil, err
	}

	return &base.Repository{
		Owner:         r.repoOwner,
		Name:          r.repoName,
		IsPrivate:     opts.Private,
		Description:   opts.Description,
		OriginalURL:   opts.URL,
		CloneURL:      filepath.Join(r.tmpDir, "repositories", owner, opts.Name+".git"),
		DefaultBranch: opts.DefaultBranch,
	}, nil
}

// GetTopics return github topics
func (r *GithubExportedDataRestorer) GetTopics() ([]string, error) {
	topics := struct {
		Topics []string `yaml:"topics"`
	}{}

	// FIXME: No topic information provided

	return topics.Topics, nil
}

func (r *GithubExportedDataRestorer) readJSONFiles(filePrefix string, makeF func() interface{}, f func(content interface{}) error) error {
	for i := 1; ; i++ {
		p := filepath.Join(r.tmpDir, fmt.Sprintf(filePrefix+"_%06d.json", i))
		_, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		bs, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		content := makeF()
		if err := json.Unmarshal(bs, content); err != nil {
			return err
		}

		if err := f(content); err != nil {
			return err
		}
	}
}

func (r *GithubExportedDataRestorer) getUsers() error {
	return r.readJSONFiles("users", func() interface{} {
		return &[]githubUser{}
	}, func(content interface{}) error {
		mss := content.(*[]githubUser)
		for _, ms := range *mss {
			r.users[ms.URL] = ms
		}
		return nil
	})
}

func (r *GithubExportedDataRestorer) getAttachments() error {
	if r.attachmentLoaded {
		return nil
	}

	return r.readJSONFiles("attachments", func() interface{} {
		r.attachmentLoaded = true
		return &[]githubAttachment{}
	}, func(content interface{}) error {
		mss := content.(*[]githubAttachment)
		for _, ms := range *mss {
			if ms.IsIssue() {
				r.issueAttachments[ms.Issue] = append(r.issueAttachments[ms.Issue], ms)
			} else {
				r.commentAttachments[ms.IssueComment] = append(r.commentAttachments[ms.IssueComment], ms)
			}
		}
		return nil
	})
}

/* {
   "type": "milestone",
   "url": "https://github.com/go-gitea/test_repo/milestones/1",
   "repository": "https://github.com/go-gitea/test_repo",
   "user": "https://github.com/mrsdizzie",
   "title": "1.0.0",
   "description": "Milestone 1.0.0",
   "state": "closed",
   "due_on": "2019-11-11T00:00:00Z",
   "created_at": "2019-11-12T19:37:08Z",
   "updated_at": "2019-11-12T21:56:17Z",
   "closed_at": "2019-11-12T19:45:49Z"
 },*/
type githubMilestone struct {
	URL         string
	User        string
	Title       string
	State       string
	Description string
	DueOn       time.Time `json:"due_on"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ClosedAt    time.Time `json:"closed_at"`
}

// GetMilestones returns milestones
func (r *GithubExportedDataRestorer) GetMilestones() ([]*base.Milestone, error) {
	milestones := make([]*base.Milestone, 0, 10)
	if err := r.readJSONFiles("milestones", func() interface{} {
		return &[]githubMilestone{}
	}, func(content interface{}) error {
		mss := content.(*[]githubMilestone)
		for _, milestone := range *mss {
			r.milestones[milestone.URL] = milestone
			milestones = append(milestones, &base.Milestone{
				Title:       milestone.Title,
				Description: milestone.Description,
				Deadline:    &milestone.DueOn,
				Created:     milestone.ClosedAt,
				Updated:     &milestone.UpdatedAt,
				Closed:      &milestone.ClosedAt,
				State:       milestone.State,
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return milestones, nil
}

/*
{
    "type": "release",
    "url": "https://github.com/go-xorm/xorm/releases/tag/v0.3.1",
    "repository": "https://github.com/go-xorm/xorm",
    "user": "https://github.com/lunny",
    "name": "",
    "tag_name": "v0.3.1",
    "body": "- Features:\n  - Support MSSQL DB via ODBC driver ([github.com/lunny/godbc](https://github.com/lunny/godbc));\n  - Composite Key, using multiple pk xorm tag \n  - Added Row() API as alternative to Iterate() API for traversing result set, provide similar usages to sql.Rows type\n  - ORM struct allowed declaration of pointer builtin type as members to allow null DB fields \n  - Before and After Event processors\n- Improvements:\n  - Allowed int/int32/int64/uint/uint32/uint64/string as Primary Key type\n  - Performance improvement for Get()/Find()/Iterate()\n",
    "state": "published",
    "pending_tag": "v0.3.1",
    "prerelease": false,
    "target_commitish": "master",
    "release_assets": [

    ],
    "published_at": "2014-01-02T09:51:34Z",
    "created_at": "2014-01-02T09:48:57Z"
  },
*/
type githubRelease struct {
	User            string
	Name            string
	TagName         string `json:"tag_name"`
	Body            string
	State           string
	Prerelease      bool
	ReleaseAssets   []githubAttachment `json:"release_assets"`
	TargetCommitish string             `json:"target_commitish"`
	CreatedAt       time.Time          `json:"created_at"`
	PublishedAt     time.Time          `json:"published_at"`
}

func (r *GithubExportedDataRestorer) getUserInfo(u string) (int64, string, string) {
	user, ok := r.users[u]
	if !ok {
		return 0, getURLLastField(u), ""
	}
	return user.ID(), user.Login, user.Email()
}

// GetReleases returns releases
func (r *GithubExportedDataRestorer) GetReleases() ([]*base.Release, error) {
	releases := make([]*base.Release, 0, 30)
	if err := r.readJSONFiles("releases", func() interface{} {
		return &[]githubRelease{}
	}, func(content interface{}) error {
		rss := content.(*[]githubRelease)
		for _, rel := range *rss {
			id, login, email := r.getUserInfo(rel.User)
			releases = append(releases, &base.Release{
				TagName:         rel.TagName,
				TargetCommitish: rel.TargetCommitish,
				Name:            rel.Name,
				Body:            rel.Body,
				Draft:           rel.State == "draft",
				Prerelease:      rel.Prerelease,
				PublisherID:     id,
				PublisherName:   login,
				PublisherEmail:  email,
				Assets:          r.convertAttachments(rel.ReleaseAssets),
				Created:         rel.CreatedAt,
				Published:       rel.PublishedAt,
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return releases, nil
}

// GetLabels returns labels
func (r *GithubExportedDataRestorer) GetLabels() ([]*base.Label, error) {
	return r.labels, nil
}

/*
		{
	    "type": "issue",
	    "url": "https://github.com/go-xorm/xorm/issues/1",
	    "repository": "https://github.com/go-xorm/xorm",
	    "user": "https://github.com/zakzou",
	    "title": "建表功能已经强大了，不过希望添加上自定义mysql engine和charset",
	    "body": "如题\n",
	    "assignee": "https://github.com/lunny",
	    "assignees": [
	      "https://github.com/lunny"
	    ],
	    "milestone": null,
	    "labels": [
				"https://github.com/go-gitea/test_repo/labels/bug",
				"https://github.com/go-gitea/test_repo/labels/good%20first%20issue"
    	],
	    "reactions": [

	    ],
	    "closed_at": "2013-08-08T05:26:00Z",
	    "created_at": "2013-07-04T08:08:39Z",
	    "updated_at": "2013-08-08T05:26:00Z"
	  },*/
type githubIssue struct {
	URL       string
	User      string
	Title     string
	Body      string
	Assignee  string
	Assignees []string
	Milestone string
	Labels    []githubLabel
	Reactions []githubReaction
	ClosedAt  *time.Time `json:"closed_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (g *githubIssue) Index() int64 {
	fields := strings.Split(g.URL, "/")
	i, _ := strconv.ParseInt(fields[len(fields)-1], 10, 64)
	return i
}

func (r *GithubExportedDataRestorer) getLabels(ls []githubLabel) []*base.Label {
	res := make([]*base.Label, 0, len(ls))
	for _, l := range ls {
		for _, ll := range r.labels {
			if l.GetName() == ll.Name {
				res = append(res, ll)
				break
			}
		}
	}
	return res
}

func (r *GithubExportedDataRestorer) getReactions(ls []githubReaction) []*base.Reaction {
	res := make([]*base.Reaction, 0, len(ls))
	for _, l := range ls {
		content := l.Content
		switch content {
		case "thinking_face":
			content = "confused"
		case "tada":
			content = "hooray"
		}

		id, login, _ := r.getUserInfo(l.User)
		if id == 0 {
			log.Warn("Cannot get user %s information, userid will be set 0", l.User)
		} else {
			res = append(res, &base.Reaction{
				UserID:   id,
				UserName: login,
				Content:  content,
			})
		}
	}
	return res
}

// GetIssues returns issues according start and limit
func (r *GithubExportedDataRestorer) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {
	if err := r.getAttachments(); err != nil {
		return nil, false, err
	}

	issues := make([]*base.Issue, 0, 50)
	if err := r.readJSONFiles("issues", func() interface{} {
		return &[]githubIssue{}
	}, func(content interface{}) error {
		rss := content.(*[]githubIssue)
		for _, issue := range *rss {
			id, login, email := r.getUserInfo(issue.User)
			var milestone string
			if issue.Milestone != "" {
				milestone = r.milestones[issue.Milestone].Title
			}

			state := "open"
			if issue.ClosedAt != nil {
				state = "closed"
			}

			issue.Body = strings.ReplaceAll(issue.Body, "\u0000", "")

			issues = append(issues, &base.Issue{
				Number:      issue.Index(),
				Title:       issue.Title,
				Content:     issue.Body,
				PosterID:    id,
				PosterName:  login,
				PosterEmail: email,
				Labels:      r.getLabels(issue.Labels),
				Reactions:   r.getReactions(issue.Reactions),
				Assets:      r.convertAttachments(r.issueAttachments[issue.URL]),
				Milestone:   milestone,
				Assignees:   issue.Assignees,
				State:       state,
				Context:     base.BasicIssueContext(issue.Index()),
				Closed:      issue.ClosedAt,
				Created:     issue.CreatedAt,
				Updated:     issue.UpdatedAt,
			})
		}
		return nil
	}); err != nil {
		return nil, false, err
	}

	return issues, true, nil
}

type githubComment struct {
	Issue       string
	PullRequest string `json:"pull_request"`
	User        string
	Body        string
	Reactions   []githubReaction
	CreatedAt   time.Time `json:"created_at"`
}

func getIssueIndex(issue, pullRequest string) int64 {
	var c string
	if issue != "" {
		c = issue
	} else {
		c = pullRequest
	}

	fields := strings.Split(c, "/")
	idx, _ := strconv.ParseInt(fields[len(fields)-1], 10, 64)
	return idx
}

func (g *githubComment) GetIssueIndex() int64 {
	return getIssueIndex(g.Issue, g.PullRequest)
}

/*
{
    "type": "issue_event",
    "url": "https://github.com/go-xorm/xorm/issues/1#event-55275262",
    "issue": "https://github.com/go-xorm/xorm/issues/1",
    "actor": "https://github.com/lunny",
    "event": "assigned",
    "created_at": "2013-07-04T14:27:53Z"
  },
  {
    "type": "issue_event",
    "url": "https://github.com/go-xorm/xorm/pull/2#event-56230828",
    "pull_request": "https://github.com/go-xorm/xorm/pull/2",
    "actor": "https://github.com/lunny",
    "event": "referenced",
    "commit_id": "1be80583b0fa18e7b478fa12e129c95e9a06a62f",
    "commit_repository": "https://github.com/go-xorm/xorm",
    "created_at": "2013-07-12T02:10:52Z"

    "label": "https://github.com/go-xorm/xorm/labels/wip",
		"label_name": "New Feature",
    "label_color": "5319e7",
    "label_text_color": "fff",
		"milestone_title": "v0.4",
		"title_was": "自动读写分离",
    "title_is": "Automatical Read/Write seperatelly.",
  },*/
type githubIssueEvent struct {
	URL              string
	Issue            string
	PullRequest      string `json:"pull_request"`
	Actor            string
	Event            string
	CommitID         string `json:"commit_id"`
	Ref              string
	BeforeCommitOID  string    `json:"before_commit_oid"`
	AfterCommitOID   string    `json:"after_commit_oid"`
	CommitRepoistory string    `json:"commit_repository"`
	CreatedAt        time.Time `json:"created_at"`
	Label            string
	LabelName        string `json:"label_name"`
	LabelColor       string `json:"label_color"`
	LabelTextColor   string `json:"label_text_color"`
	MilestoneTitle   string `json:"milestone_title"`
	Subject          string
	TitleWas         string `json:"title_was"`
	TitleIs          string `json:"title_is"`
}

// CommentContent returns comment content
func (g *githubIssueEvent) CommentContent() map[string]interface{} {
	switch g.Event {
	case "closed":
		return map[string]interface{}{}
	case "head_ref_force_pushed":
		return map[string]interface{}{}
	case "moved_columns_in_project":
		return map[string]interface{}{}
	case "referenced":
		tp := "commit_ref"
		if g.Issue != "" {
			tp = "issue_ref"
		} else if g.PullRequest != "" {
			tp = "pull_ref"
		}
		return map[string]interface{}{
			"type":     tp,
			"CommitID": g.CommitID,
		}
	case "merged":
		return map[string]interface{}{}
	case "mentioned":
		return map[string]interface{}{}
	case "subscribed":
		return map[string]interface{}{}
	case "head_ref_deleted":
		return map[string]interface{}{}
	case "head_ref_restored":
		return map[string]interface{}{}
	case "milestoned":
		return map[string]interface{}{
			"type":           "add",
			"MilestoneTitle": g.MilestoneTitle,
		}
	case "demilestoned":
		return map[string]interface{}{
			"type":           "remove",
			"MilestoneTitle": g.MilestoneTitle,
		}
	case "labeled":
		return map[string]interface{}{
			"type":           "add",
			"Label":          g.Label,
			"LabelName":      g.LabelName,
			"LabelColor":     g.LabelColor,
			"LabelTextColor": g.LabelTextColor,
		}
	case "renamed":
		return map[string]interface{}{
			"OldTitle": g.TitleWas,
			"NewTitle": g.TitleIs,
		}
	case "ready_for_review":
		return map[string]interface{}{}
	case "reopened":
		return map[string]interface{}{}
	case "unlabeled":
		return map[string]interface{}{
			"type":           "remove",
			"Label":          g.Label,
			"LabelName":      g.LabelName,
			"LabelColor":     g.LabelColor,
			"LabelTextColor": g.LabelTextColor,
		}
	case "assigned":
		return map[string]interface{}{
			"Actor":   g.Actor,
			"Subject": g.Subject,
		}
	case "added_to_project":
		return map[string]interface{}{}
	default:
		return map[string]interface{}{}
	}
}

// CommentStr returns comment type string
func (g *githubIssueEvent) CommentStr() string {
	switch g.Event {
	case "closed":
		return "close"
	case "head_ref_force_pushed":
		return "pull_push"
	case "referenced":
		return "commit_ref"
	case "moved_columns_in_project":
		return "unknown"
	case "convert_to_draft":
		return "unknown"
	case "ready_for_review":
		return "unknown"
	case "merged":
		return "merge_pull"
	case "mentioned":
		return "unknown" // ignore
	case "subscribed":
		return "unknown" // ignore
	case "head_ref_deleted":
		return "delete_branch"
	case "head_ref_restored":
		return "unknown"
	case "added_to_project":
		return "unknown"
	case "milestoned":
		return "milestone"
	case "demilestoned":
		return "milestone"
	case "labeled":
		return "label"
	case "renamed":
		return "change_title"
	case "reopened":
		return "reopen"
	case "unlabeled":
		return "label"
	case "assigned":
		return "assignees"
	case "pinned":
		return "pinned"
	case "unpinned":
		return "unpinned"
	default:
		return "comment"
	}
}

func (g *githubIssueEvent) GetIssueIndex() int64 {
	return getIssueIndex(g.Issue, g.PullRequest)
}

func (r *GithubExportedDataRestorer) getIssueEvents() ([]*base.Comment, error) {
	comments := make([]*base.Comment, 0, 10)
	if err := r.readJSONFiles("issue_events", func() interface{} {
		return &[]githubIssueEvent{}
	}, func(content interface{}) error {
		rss := content.(*[]githubIssueEvent)
		for _, c := range *rss {
			id, login, email := r.getUserInfo(c.Actor)
			v := c.CommentContent()
			bs, err := json.Marshal(v)
			if err != nil {
				return err
			}

			comments = append(comments, &base.Comment{
				Type:        c.CommentStr(),
				IssueIndex:  c.GetIssueIndex(),
				PosterID:    id,
				PosterName:  login,
				PosterEmail: email,
				Created:     c.CreatedAt,
				Updated:     c.CreatedAt, // FIXME:
				Content:     string(bs),
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return comments, nil
}

// GetComments returns comments according issueNumber
func (r *GithubExportedDataRestorer) GetComments(opts base.GetCommentOptions) ([]*base.Comment, bool, error) {
	comments := make([]*base.Comment, 0, 10)
	if err := r.readJSONFiles("issue_comments", func() interface{} {
		return &[]githubComment{}
	}, func(content interface{}) error {
		rss := content.(*[]githubComment)
		for _, c := range *rss {
			id, login, email := r.getUserInfo(c.User)
			comments = append(comments, &base.Comment{
				IssueIndex:  c.GetIssueIndex(),
				PosterID:    id,
				PosterName:  login,
				PosterEmail: email,
				Created:     c.CreatedAt,
				Updated:     c.CreatedAt, // FIXME:
				Content:     r.replaceGithubLinks(c.Body),
				Reactions:   r.getReactions(c.Reactions),
			})
		}
		return nil
	}); err != nil {
		return nil, false, err
	}

	comments2, err := r.getIssueEvents()
	if err != nil {
		return nil, false, err
	}

	return append(comments, comments2...), true, nil
}

/*
		{
	    "type": "pull_request",
	    "url": "https://github.com/go-xorm/xorm/pull/2",
	    "user": "https://github.com/airylinus",
	    "repository": "https://github.com/go-xorm/xorm",
	    "title": "修正文档中代码示例中的笔误",
	    "body": "1. 修正变量名错误\n2. 修改查询示例的查询，让代码更易懂\n",
	    "base": {
	      "ref": "master",
	      "sha": "a9eb28a00e4b93817906eac5c8af2a566e8c73af",
	      "user": "https://github.com/go-xorm",
	      "repo": "https://github.com/go-xorm/xorm"
	    },
	    "head": {
	      "ref": "master",
	      "sha": "c18e4e8d174cd7619333f7645bd9dccd4cbf5168",
	      "user": "https://github.com/airylinus",
	      "repo": null
	    },
	    "assignee": null,
	    "assignees": [

	    ],
	    "milestone": null,
	    "labels": [

	    ],
	    "reactions": [

	    ],
	    "review_requests": [

	    ],
	    "close_issue_references": [

	    ],
	    "work_in_progress": false,
	    "merged_at": "2013-07-12T02:10:52Z",
	    "closed_at": "2013-07-12T02:10:52Z",
	    "created_at": "2013-07-12T02:04:44Z"
	  },
*/
type githubPullRequest struct {
	URL   string
	User  string
	Title string
	Body  string
	Base  struct {
		Ref  string
		Sha  string
		User string
		Repo string
	}
	Head struct {
		Ref  string
		Sha  string
		User string
		Repo string
	}
	Assignee             string
	Assignees            []string
	Milestone            string
	Labels               []githubLabel
	Reactions            []githubReaction
	ReviewRequests       []struct{} `json:"review_requests"`
	CloseIssueReferences []struct{} `json:"close_issue_references"`
	WorkInProgress       bool       `json:"work_in_progress"`
	MergedAt             *time.Time `json:"merged_at"`
	ClosedAt             *time.Time `json:"closed_at"`
	CreatedAt            time.Time  `json:"created_at"`
}

func (g *githubPullRequest) Index() int64 {
	fields := strings.Split(g.URL, "/")
	i, _ := strconv.ParseInt(fields[len(fields)-1], 10, 64)
	return i
}

// GetPullRequests returns pull requests according page and perPage
func (r *GithubExportedDataRestorer) GetPullRequests(page, perPage int) ([]*base.PullRequest, bool, error) {
	pulls := make([]*base.PullRequest, 0, 50)
	if err := r.readJSONFiles("pull_requests", func() interface{} {
		return &[]githubPullRequest{}
	}, func(content interface{}) error {
		prs := content.(*[]githubPullRequest)
		for _, pr := range *prs {
			id, login, email := r.getUserInfo(pr.User)
			state := "open"
			if pr.MergedAt != nil || pr.ClosedAt != nil {
				state = "closed"
			}
			isFork := !(pr.Head.User == pr.Base.User && pr.Head.Repo == pr.Base.Repo)
			head := base.PullRequestBranch{
				Ref: pr.Head.Ref,
				SHA: pr.Head.Sha,
			}
			if isFork {
				if pr.Head.User != "" {
					fields := strings.Split(pr.Head.User, "/")
					if pr.Head.Ref == "" {
						pr.Head.Ref = fmt.Sprintf("%d", pr.Index())
					}
					head.Ref = fmt.Sprintf("%s/%s", fields[len(fields)-1], pr.Head.Ref)
				} else {
					head.Ref = fmt.Sprintf("pr/%d", pr.Index())
				}
			}
			var milestone string
			if pr.Milestone != "" {
				milestone = r.milestones[pr.Milestone].Title
			}
			pulls = append(pulls, &base.PullRequest{
				Number:      pr.Index(),
				Title:       pr.Title,
				Content:     pr.Body,
				Milestone:   milestone,
				State:       state,
				PosterID:    id,
				PosterName:  login,
				PosterEmail: email,
				Context:     base.BasicIssueContext(pr.Index()),
				Reactions:   r.getReactions(pr.Reactions),
				Created:     pr.CreatedAt,
				Closed:      pr.ClosedAt,
				Labels:      r.getLabels(pr.Labels),
				Merged:      pr.MergedAt != nil,
				MergedTime:  pr.MergedAt,
				Head:        head,
				Base: base.PullRequestBranch{
					Ref: pr.Base.Ref,
					SHA: pr.Base.Sha,
				},
				Assignees: pr.Assignees,
			})
		}
		return nil
	}); err != nil {
		return nil, false, err
	}

	return pulls, true, nil
}

/*
		{
	    "type": "pull_request_review",
	    "url": "https://github.com/go-gitea/test_repo/pull/3/files#pullrequestreview-315859956",
	    "pull_request": "https://github.com/go-gitea/test_repo/pull/3",
	    "user": "https://github.com/jolheiser",
	    "body": "",
	    "head_sha": "076160cf0b039f13e5eff19619932d181269414b",
	    "formatter": "markdown",
	    "state": 40,
	    "reactions": [

	    ],
	    "created_at": "2019-11-12T21:35:24Z",
	    "submitted_at": "2019-11-12T21:35:24Z"
	  },
*/
type pullrequestReview struct {
	URL         string
	PullRequest string `json:"pull_request"`
	User        string
	Body        string
	HeadSha     string `json:"head_sha"`
	State       int
	Reactions   []githubReaction
	CreatedAt   time.Time  `json:"created_at"`
	SubmittedAt *time.Time `json:"submitted_at"`
}

func (p *pullrequestReview) Index() int64 {
	fields := strings.Split(p.PullRequest, "/")
	idx, _ := strconv.ParseInt(fields[len(fields)-1], 10, 64)
	return idx
}

// GetState return PENDING, APPROVED, REQUEST_CHANGES, or COMMENT
func (p *pullrequestReview) GetState() string {
	switch p.State {
	case 1:
		return base.ReviewStateCommented
	case 30:
		return base.ReviewStateChangesRequested
	case 40:
		return base.ReviewStateApproved
	}
	return fmt.Sprintf("%d", p.State)
}

/*
{
    "type": "pull_request_review_thread",
    "url": "https://github.com/go-xorm/xorm/pull/1445/files#pullrequestreviewthread-203253693",
    "pull_request": "https://github.com/go-xorm/xorm/pull/1445",
    "pull_request_review": "https://github.com/go-xorm/xorm/pull/1445/files#pullrequestreview-295977501",
    "diff_hunk": "@@ -245,12 +245,17 @@ func (session *Session) Sync2(beans ...interface{}) error {\n \t\tif err != nil {\n \t\t\treturn err\n \t\t}\n-\t\ttbName := engine.TableName(bean)\n-\t\ttbNameWithSchema := engine.TableName(tbName, true)\n+\t\tvar tbName string\n+\t\tif len(session.statement.AltTableName) > 0 {\n+\t\t\ttbName = session.statement.AltTableName\n+\t\t} else {\n+\t\t\ttbName = engine.TableName(bean)\n+\t\t}\n+\t\ttbNameWithSchema := engine.tbNameWithSchema(tbName)\n \n \t\tvar oriTable *core.Table\n \t\tfor _, tb := range tables {\n-\t\t\tif strings.EqualFold(tb.Name, tbName) {\n+\t\t\tif strings.EqualFold(engine.tbNameWithSchema(tb.Name), engine.tbNameWithSchema(tbName)) {",
    "path": "session_schema.go",
    "position": 17,
    "original_position": 17,
    "commit_id": "f6b642c82aab95178a4551a1ff65dc2a631a08cf",
    "original_commit_id": "f6b642c82aab95178a4551a1ff65dc2a631a08cf",
    "start_line": null,
    "line": 258,
    "start_side": null,
    "side": "right",
    "original_start_line": null,
    "original_line": 258,
    "created_at": "2019-10-02T01:40:41Z",
    "resolved_at": null,
    "resolver": null
  },*/
type pullrequestReviewThread struct {
	URL               string
	PullRequest       string `json:"pull_request"`
	PullRequestReview string `json:"pull_request_review"`
	DiffHunk          string `json:"diff_hunk"`
	Path              string
	Position          int64
	OriginalPosition  int64  `json:"original_position"`
	CommitID          string `json:"commit_id"`
	OriginalCommitID  string `json:"original_commit_id"`
	Line              int64
	Side              string
	OriginalLine      int64      `json:"original_line"`
	CreatedAt         time.Time  `json:"created_at"`
	ResolvedAt        *time.Time `json:"resolved_at"`
	Resolver          string
}

func (p *pullrequestReviewThread) Index() int64 {
	fields := strings.Split(p.PullRequest, "/")
	idx, _ := strconv.ParseInt(fields[len(fields)-1], 10, 64)
	return idx
}

/*{
  "type": "pull_request_review_comment",
  "url": "https://github.com/go-gitea/test_repo/pull/4/files#r363017488",
  "pull_request": "https://github.com/go-gitea/test_repo/pull/4",
  "pull_request_review": "https://github.com/go-gitea/test_repo/pull/4/files#pullrequestreview-338338740",
  "pull_request_review_thread": "https://github.com/go-gitea/test_repo/pull/4/files#pullrequestreviewthread-224172719",
  "user": "https://github.com/lunny",
  "body": "This is a good pull request.",
  "formatter": "markdown",
  "diff_hunk": "@@ -1,2 +1,4 @@\n # test_repo\n Test repository for testing migration from github to gitea\n+",
  "path": "README.md",
  "position": 3,
  "original_position": 3,
  "commit_id": "2be9101c543658591222acbee3eb799edfc3853d",
  "original_commit_id": "2be9101c543658591222acbee3eb799edfc3853d",
  "state": 1,
  "in_reply_to": null,
  "reactions": [

  ],
  "created_at": "2020-01-04T05:33:06Z"
},*/
type pullrequestReviewComment struct {
	PullRequest             string `json:"pull_request"`
	PullRequestReview       string `json:"pull_request_review"`
	PullRequestReviewThread string `json:"pull_request_review_thread"`
	User                    string
	Body                    string
	DiffHunk                string `json:"diff_hunk"`
	Path                    string
	Position                int
	OriginalPosition        int    `json:"original_position"`
	CommitID                string `json:"commit_id"`
	OriginalCommitID        string `json:"original_commit_id"`
	State                   int
	Reactions               []githubReaction
	CreatedAt               time.Time `json:"created_at"`
}

func (r *GithubExportedDataRestorer) getReviewComments(thread *pullrequestReviewThread, comments []pullrequestReviewComment) []*base.ReviewComment {
	res := make([]*base.ReviewComment, 0, 10)
	for _, c := range comments {
		id, login, email := r.getUserInfo(c.User)
		position := int(thread.Position)
		if thread.Side == "right" {
			position = int(thread.OriginalPosition)
		}
		// Line will be parse from diffhunk, so we ignore it here
		res = append(res, &base.ReviewComment{
			Content:     c.Body,
			TreePath:    c.Path,
			DiffHunk:    c.DiffHunk,
			Position:    position,
			CommitID:    c.OriginalCommitID,
			PosterID:    id,
			PosterName:  login,
			PosterEmail: email,
			Reactions:   r.getReactions(c.Reactions),
			CreatedAt:   c.CreatedAt,
		})
	}
	return res
}

// GetReviews returns pull requests review
func (r *GithubExportedDataRestorer) GetReviews(opts base.GetReviewOptions) ([]*base.Review, bool, error) {
	comments := make(map[string][]pullrequestReviewComment)
	if err := r.readJSONFiles("pull_request_review_comments", func() interface{} {
		return &[]pullrequestReviewComment{}
	}, func(content interface{}) error {
		cs := *content.(*[]pullrequestReviewComment)
		for _, c := range cs {
			comments[c.PullRequestReviewThread] = append(comments[c.PullRequestReviewThread], c)
		}
		return nil
	}); err != nil {
		return nil, true, err
	}

	reviews := make(map[string]*base.Review, 10)
	if err := r.readJSONFiles("pull_request_reviews", func() interface{} {
		return &[]pullrequestReview{}
	}, func(content interface{}) error {
		prReviews := content.(*[]pullrequestReview)
		for _, review := range *prReviews {
			id, login, email := r.getUserInfo(review.User)
			baseReview := &base.Review{
				IssueIndex:    review.Index(),
				ReviewerID:    id,
				ReviewerName:  login,
				ReviewerEmail: email,
				CommitID:      review.HeadSha,
				Content:       review.Body,
				CreatedAt:     review.CreatedAt,
				State:         review.GetState(),
			}
			reviews[review.URL] = baseReview
		}
		return nil
	}); err != nil {
		return nil, true, err
	}

	if err := r.readJSONFiles("pull_request_review_threads", func() interface{} {
		return &[]pullrequestReviewThread{}
	}, func(content interface{}) error {
		cs := *content.(*[]pullrequestReviewThread)
		for _, review := range cs {
			reviewComments := comments[review.URL]
			if len(reviewComments) == 0 {
				continue
			}
			rr, ok := reviews[review.PullRequestReview]
			if !ok {
				id, login, email := r.getUserInfo(reviewComments[0].User)
				rr = &base.Review{
					IssueIndex:    review.Index(),
					ReviewerID:    id,
					ReviewerName:  login,
					ReviewerEmail: email,
					CommitID:      review.CommitID,
					CreatedAt:     review.CreatedAt,
					State:         base.ReviewStateCommented,
				}
				reviews[review.URL] = rr
			}

			rr.Comments = r.getReviewComments(&review, reviewComments)
			rr.ResolvedAt = review.ResolvedAt
			if resolver, ok := r.users[review.Resolver]; ok {
				rr.ResolverID = resolver.ID()
				rr.ResolverName = resolver.Login
				rr.ResolverEmail = resolver.Email()
			}
		}
		return nil
	}); err != nil {
		return nil, true, err
	}

	rs := make([]*base.Review, 0, len(reviews))
	for _, review := range reviews {
		rs = append(rs, review)
	}

	return rs, true, nil
}
