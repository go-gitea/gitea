// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	base "code.gitea.io/gitea/modules/migration"
)

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
type GithubUser struct {
	URL       string
	AvatarURL string
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
	CreatedAt time.Time
}

func (g *GithubUser) ID() int64 {
	u, _ := url.Parse(g.AvatarURL)
	fields := strings.Split(u.Path, "/")
	i, _ := strconv.ParseInt(fields[len(fields)-1], 10, 64)
	return i
}

func (g *GithubUser) Email() string {
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
    "issue_comment": "https://github.com/go-xorm/xorm/issues/115#issuecomment-42628488",
    "user": "https://github.com/mintzhao",
    "asset_name": "QQ20140509-1.2x.png",
    "asset_content_type": "image/png",
    "asset_url": "tarball://root/attachments/63a167ce-d721-11e3-91b6-74b83dc345bb/QQ20140509-1.2x.png",
    "created_at": "2014-05-09T02:38:54Z"
  },
*/
type githubAttachment struct {
	IssueComment     string
	User             string
	AssetName        string
	AssetContentType string
	AssetURL         string
	CreatedAt        time.Time
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
	SubjectType string
	CreatedAt   time.Time
}

type githubLabel string

func (l githubLabel) GetName() string {
	fields := strings.Split(string(l), "/labels/")
	return fields[len(fields)-1]
}

// GithubExportedDataRestorer implements an Downloader from the exported data of Github
type GithubExportedDataRestorer struct {
	base.NullDownloader
	ctx                context.Context
	tmpDir             string
	githubDataFilePath string
	repoOwner          string
	repoName           string
	labels             []*base.Label
	users              map[string]*GithubUser
}

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

	var restorer = &GithubExportedDataRestorer{
		ctx:                ctx,
		githubDataFilePath: githubDataFilePath,
		tmpDir:             tmpDir,
		repoOwner:          owner,
		repoName:           repoName,
		users:              make(map[string]*GithubUser),
	}
	if err := restorer.getUsers(); err != nil {
		return nil, err
	}

	return restorer, nil
}

// SetContext set context
func (r *GithubExportedDataRestorer) SetContext(ctx context.Context) {
	r.ctx = ctx
}

// GetRepoInfo returns a repository information
func (r *GithubExportedDataRestorer) GetRepoInfo() (*base.Repository, error) {
	type Label struct {
		URL         string
		Name        string `json:"name"`
		Color       string
		Description string
		CreatedAt   time.Time
	}
	type GithubRepo struct {
		Name          string `json:"name"`
		URL           string
		Owner         string
		Description   string
		Private       bool
		Labels        []Label `json:"labels"`
		CreatedAt     time.Time
		DefaultBranch string
	}

	var githubRepositories []GithubRepo
	p := filepath.Join(r.tmpDir, "repositories_000001.json")
	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(bs, &githubRepositories); err != nil {
		return nil, err
	}
	if len(githubRepositories) <= 0 {
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
	var topics = struct {
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
		return &[]GithubUser{}
	}, func(content interface{}) error {
		mss := content.(*[]GithubUser)
		for _, ms := range *mss {
			r.users[ms.URL] = &ms
		}
		return nil
	})
}

// GetMilestones returns milestones
func (r *GithubExportedDataRestorer) GetMilestones() ([]*base.Milestone, error) {
	type milestone struct {
		Title       string
		State       string
		Description string
		DueOn       time.Time
		CreatedAt   time.Time
		UpdatedAt   time.Time
		ClosedAt    time.Time
	}

	var milestones = make([]*base.Milestone, 0, 10)
	if err := r.readJSONFiles("milestones", func() interface{} {
		return &[]milestone{}
	}, func(content interface{}) error {
		mss := content.(*[]milestone)
		for _, milestone := range *mss {
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

// GetReleases returns releases
func (r *GithubExportedDataRestorer) GetReleases() ([]*base.Release, error) {
	type release struct {
		Name          string
		TagName       string
		Body          string
		State         string
		Prerelease    bool
		ReleaseAssets []struct {
		}
		TargetCommitish string
		CreatedAt       time.Time
		PublishedAt     time.Time
	}

	var releases = make([]*base.Release, 0, 30)
	if err := r.readJSONFiles("releases", func() interface{} {
		return &[]release{}
	}, func(content interface{}) error {
		rss := content.(*[]release)
		for _, rel := range *rss {
			// TODO
			/*for _, asset := range rel.ReleaseAssets {
				if asset.DownloadURL != nil {
					*asset.DownloadURL = "file://" + filepath.Join(r.baseDir, *asset.DownloadURL)
				}
			}*/

			releases = append(releases, &base.Release{
				TagName:         rel.TagName,
				TargetCommitish: rel.TargetCommitish,
				Name:            rel.Name,
				Body:            rel.Body,
				Draft:           rel.State == "draft",
				Prerelease:      rel.Prerelease,
				//PublisherID     : rel.
				//PublisherName   string `yaml:"publisher_name"`
				//PublisherEmail  string `yaml:"publisher_email"`
				Assets:    []*base.ReleaseAsset{},
				Created:   rel.CreatedAt,
				Published: rel.PublishedAt,
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
	Lables    []githubLabel
	Reactions []githubReaction
	ClosedAt  *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (g *githubIssue) Index() int64 {
	fields := strings.Split(g.URL, "/")
	i, _ := strconv.ParseInt(fields[len(fields)-1], 10, 64)
	return i
}

func (r *GithubExportedDataRestorer) getLabels(ls []githubLabel) []*base.Label {
	var res = make([]*base.Label, 0, len(ls))
	for _, l := range ls {
		for _, ll := range r.labels {
			if string(l) == ll.Name {
				res = append(res, ll)
				break
			}
		}
	}
	return res
}

func (r *GithubExportedDataRestorer) getReactions(ls []githubReaction) []*base.Reaction {
	var res = make([]*base.Reaction, 0, len(ls))
	for _, l := range ls {
		user := r.users[l.User]
		res = append(res, &base.Reaction{
			UserID:   user.ID(),
			UserName: user.Login,
			Content:  l.Content,
		})
	}
	return res
}

// GetIssues returns issues according start and limit
func (r *GithubExportedDataRestorer) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {
	var issues = make([]*base.Issue, 0, 50)
	if err := r.readJSONFiles("issues", func() interface{} {
		return &[]githubIssue{}
	}, func(content interface{}) error {
		rss := content.(*[]githubIssue)
		for _, issue := range *rss {
			user := r.users[issue.User]

			var state = "open"
			if issue.ClosedAt != nil {
				state = "closed"
			}

			issues = append(issues, &base.Issue{
				Number:      issue.Index(),
				Title:       issue.Title,
				Content:     issue.Body,
				PosterID:    user.ID(),
				PosterName:  user.Login,
				PosterEmail: user.Email(),
				Labels:      r.getLabels(issue.Lables),
				Reactions:   r.getReactions(issue.Reactions),
				Milestone:   issue.Milestone,
				Assignees:   issue.Assignees,
				//Ref: issue.
				State:   state,
				Context: base.BasicIssueContext(issue.Index()),
				Closed:  issue.ClosedAt,
				Created: issue.CreatedAt,
				Updated: issue.UpdatedAt,
			})
		}
		return nil
	}); err != nil {
		return nil, false, err
	}

	return issues, true, nil
}

// GetComments returns comments according issueNumber
func (r *GithubExportedDataRestorer) GetComments(opts base.GetCommentOptions) ([]*base.Comment, bool, error) {
	type Comment struct {
		Issue       string
		PullRequest string
		User        string
		Body        string
		Reactions   []githubReaction
		CreatedAt   time.Time
	}

	var comments = make([]*base.Comment, 0, 10)
	if err := r.readJSONFiles("issue_comments", func() interface{} {
		return &[]Comment{}
	}, func(content interface{}) error {
		rss := content.(*[]Comment)
		for _, c := range *rss {
			fields := strings.Split(c.Issue, "/")
			idx, _ := strconv.ParseInt(fields[len(fields)-1], 10, 64)
			u := r.users[c.User]

			comments = append(comments, &base.Comment{
				IssueIndex:  idx,
				PosterID:    u.ID(),
				PosterName:  u.Login,
				PosterEmail: "",
				Created:     c.CreatedAt,
				//Updated:,
				Content:   c.Body,
				Reactions: r.getReactions(c.Reactions),
			})
		}
		return nil
	}); err != nil {
		return nil, false, err
	}
	return comments, true, nil
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
	Lables               []githubLabel
	Reactions            []githubReaction
	ReviewRequests       []struct{}
	CloseIssueReferences []struct{}
	WorkInProgress       bool
	MergedAt             *time.Time
	ClosedAt             *time.Time
	CreatedAt            time.Time
}

func (g *githubPullRequest) Index() int64 {
	fields := strings.Split(g.URL, "/")
	i, _ := strconv.ParseInt(fields[len(fields)-1], 10, 64)
	return i
}

// GetPullRequests returns pull requests according page and perPage
func (r *GithubExportedDataRestorer) GetPullRequests(page, perPage int) ([]*base.PullRequest, bool, error) {
	var pulls = make([]*base.PullRequest, 0, 50)
	if err := r.readJSONFiles("pull_requests", func() interface{} {
		return &[]githubPullRequest{}
	}, func(content interface{}) error {
		prs := content.(*[]githubPullRequest)
		for _, pr := range *prs {
			user := r.users[pr.User]
			var state = "open"
			if pr.MergedAt != nil {
				state = "merged"
			} else if pr.ClosedAt != nil {
				state = "closed"
			}
			pulls = append(pulls, &base.PullRequest{
				Number:      pr.Index(),
				Title:       pr.Title,
				Content:     pr.Body,
				Milestone:   pr.Milestone,
				State:       state,
				PosterID:    user.ID(),
				PosterName:  user.Login,
				PosterEmail: user.Email(),
				Context:     base.BasicIssueContext(pr.Index()),
				Reactions:   r.getReactions(pr.Reactions),
				Created:     pr.CreatedAt,
				//Updated:     pr.,
				Closed: pr.ClosedAt,
				Labels: r.getLabels(pr.Lables),
				//PatchURL       : pr.
				Merged:     pr.MergedAt != nil,
				MergedTime: pr.MergedAt,
				//MergeCommitSHA : pr.Merge
				Head: base.PullRequestBranch{
					Ref: pr.Head.Ref,
					SHA: pr.Head.Sha,
					// TODO:
				},
				Base: base.PullRequestBranch{
					Ref: pr.Base.Ref,
					SHA: pr.Base.Sha,
					// TODO:
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
	PullRequest string
	User        string
	Body        string
	HeadSha     string
	State       int
	Reactions   []githubReaction
	CreatedAt   time.Time
	SubmittedAt *time.Time
}

func (p *pullrequestReview) Index() int64 {
	fields := strings.Split(p.PullRequest, "/")
	idx, _ := strconv.ParseInt(fields[len(fields)-1], 10, 64)
	return idx
}

func (p *pullrequestReview) GetState() string {
	return fmt.Sprintf("%d", p.State)
}

// GetReviews returns pull requests review
func (r *GithubExportedDataRestorer) GetReviews(context base.IssueContext) ([]*base.Review, error) {
	var reviews = make([]*base.Review, 0, 10)
	if err := r.readJSONFiles("pull_request_reviews", func() interface{} {
		return &[]pullrequestReview{}
	}, func(content interface{}) error {
		prReviews := content.(*[]pullrequestReview)
		for _, review := range *prReviews {
			user := r.users[review.User]
			reviews = append(reviews, &base.Review{
				IssueIndex:   review.Index(),
				ReviewerID:   user.ID(),
				ReviewerName: user.Login,
				CommitID:     review.HeadSha,
				Content:      review.Body,
				CreatedAt:    review.CreatedAt,
				State:        review.GetState(),
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return reviews, nil
}
