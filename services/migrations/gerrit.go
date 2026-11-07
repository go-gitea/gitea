// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	giturl "gitea.dev/modules/git/url"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/structs"
)

var (
	_ base.Downloader        = &GerritDownloader{}
	_ base.DownloaderFactory = &GerritDownloaderFactory{}
)

const (
	gerritMaxPageSize     = 100
	gerritMaxResponseSize = 64 * 1024 * 1024
	gerritMaxPathDepth    = 8
	gerritXSSIPrefix      = ")]}'"
)

func init() {
	RegisterDownloaderFactory(&GerritDownloaderFactory{})
}

// GerritDownloaderFactory defines a downloader factory for Gerrit Code Review
type GerritDownloaderFactory struct{}

// New returns a Downloader related to this factory according MigrateOptions
func (f *GerritDownloaderFactory) New(_ctx context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	location, err := parseGerritCloneAddr(opts.CloneAddr)
	if err != nil {
		return nil, err
	}
	username, password := opts.AuthUsername, opts.AuthPassword
	if location.username != "" && username == "" {
		username, password = location.username, cmp.Or(password, location.password)
	}
	log.Trace("Create Gerrit downloader. Host: %s Path: %s", location.host, location.path)
	return NewGerritDownloader(location, username, password), nil
}

// GitServiceType returns the type of git service
func (f *GerritDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.GerritService
}

// gerritLocation is the parsed HTTP(S) clone URL of a Gerrit project
type gerritLocation struct {
	scheme   string
	host     string
	path     []string // path segments, the "/a/" authentication prefix included as given
	username string
	password string
}

func (l *gerritLocation) String() string {
	return (&url.URL{Scheme: l.scheme, Host: l.host, Path: "/" + strings.Join(l.path, "/")}).String()
}

func parseGerritCloneAddr(rawURL string) (*gerritLocation, error) {
	u, err := giturl.ParseGitURL(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, errors.New("invalid Gerrit clone URL")
	}
	// the address of the REST API can only be derived from an HTTP(S) clone URL, not from SSH or a local path
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q for Gerrit, use the HTTP(S) clone URL of the project", u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, errors.New("the Gerrit clone URL has no host")
	}

	location := &gerritLocation{scheme: u.Scheme, host: u.Host}
	if u.User != nil {
		location.username = u.User.Username()
		location.password, _ = u.User.Password()
	}
	trimmed := strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
	if trimmed == "" {
		return nil, fmt.Errorf("the Gerrit clone URL %q has no project name", location)
	}
	location.path = strings.Split(trimmed, "/")
	if len(location.path) > gerritMaxPathDepth {
		return nil, fmt.Errorf("the Gerrit clone URL %q has too many path segments", location)
	}
	for _, seg := range location.path {
		if seg == "" || seg == "." || seg == ".." {
			return nil, fmt.Errorf("the Gerrit clone URL %q has an invalid path", location)
		}
	}
	return location, nil
}

// gerritCandidate is a way to split a clone URL path into the root of the Gerrit web application and a project name
type gerritCandidate struct {
	root    string
	project string
}

// candidates lists the possible splits of the path, as the length of the project name is not part of the URL.
// A segment "a" right after the root is the prefix of authenticated requests, not a part of the project.
func (l *gerritLocation) candidates() []gerritCandidate {
	var result []gerritCandidate
	for i := range l.path {
		root := (&url.URL{Scheme: l.scheme, Host: l.host, Path: "/" + strings.Join(l.path[:i], "/")}).String()
		root = strings.TrimSuffix(root, "/")
		rest := l.path[i:]
		if rest[0] == "a" && len(rest) > 1 {
			result = append(result, gerritCandidate{root: root, project: strings.Join(rest[1:], "/")})
		}
		result = append(result, gerritCandidate{root: root, project: strings.Join(rest, "/")})
	}
	return result
}

// GerritDownloader implements a Downloader for the REST API of Gerrit Code Review.
// A Gerrit change is a pull request, its patch sets are fetched through refs/changes with the repository.
// Gerrit has no issues, wiki, labels, milestones or releases.
type GerritDownloader struct {
	base.NullDownloader
	client   *http.Client
	location *gerritLocation
	username string
	password string

	root        string // URL of the Gerrit web application, without the authentication prefix
	project     string
	description string

	changeNumbers []int64                   // all changes of the project, ascending
	activities    map[int64]*gerritActivity // review data by change number, of the current page
}

// NewGerritDownloader creates a Gerrit downloader
func NewGerritDownloader(location *gerritLocation, username, password string) *GerritDownloader {
	return &GerritDownloader{
		client:     newMigrationHTTPClient(location.String(), basicAuthorization(username, password)),
		activities: map[int64]*gerritActivity{},
		location:   location,
		username:   username,
		password:   password,
	}
}

// isAuthenticated tells whether requests carry credentials, Gerrit's HTTP authentication needs a username
func (d *GerritDownloader) isAuthenticated() bool {
	return d.username != ""
}

// String implements Stringer
func (d *GerritDownloader) String() string {
	return fmt.Sprintf("migration from gerrit %s", d.location)
}

// LogString returns a log friendly description
func (d *GerritDownloader) LogString() string {
	if d == nil {
		return "<GerritDownloader nil>"
	}
	return fmt.Sprintf("<GerritDownloader %s>", d.location)
}

// FormatCloneURL adds authentication into the remote URL
func (d *GerritDownloader) FormatCloneURL(_ base.MigrateOptions, remoteAddr string) (string, error) {
	u, err := url.Parse(remoteAddr)
	if err != nil {
		return "", err
	}
	if d.isAuthenticated() {
		u.User = url.UserPassword(d.username, d.password)
	}
	return u.String(), nil
}

type gerritTime struct {
	time.Time
}

func (t *gerritTime) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	if text == "" {
		return nil
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", text, time.UTC)
	if err != nil {
		return fmt.Errorf("invalid Gerrit timestamp %q", text)
	}
	t.Time = parsed
	return nil
}

type gerritAccount struct {
	ID       int64  `json:"_account_id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

func (a gerritAccount) displayName() string {
	return cmp.Or(a.Name, a.Username, a.Email, "gerrit-user-"+strconv.FormatInt(a.ID, 10))
}

type gerritRevision struct {
	Number int    `json:"_number"`
	Ref    string `json:"ref"`
	Commit struct {
		Parents []struct {
			Commit string `json:"commit"`
		} `json:"parents"`
		Message string `json:"message"`
	} `json:"commit"`
}

type gerritMessage struct {
	Author         gerritAccount `json:"author"`
	Date           gerritTime    `json:"date"`
	Message        string        `json:"message"`
	Tag            string        `json:"tag"`
	RevisionNumber int           `json:"_revision_number"`
}

// gerritChange is the downloader context of a pull request
type gerritChange struct {
	Branch          string                    `json:"branch"`
	ChangeID        string                    `json:"change_id"`
	Subject         string                    `json:"subject"`
	Status          string                    `json:"status"`
	Created         gerritTime                `json:"created"`
	Updated         gerritTime                `json:"updated"`
	Submitted       gerritTime                `json:"submitted"`
	Owner           gerritAccount             `json:"owner"`
	Number          int64                     `json:"_number"`
	WorkInProgress  bool                      `json:"work_in_progress"`
	CurrentRevision string                    `json:"current_revision"`
	Revisions       map[string]gerritRevision `json:"revisions"`
	Messages        []gerritMessage           `json:"messages"`
}

func (c *gerritChange) revision(patchSet int) string {
	for sha, revision := range c.Revisions {
		if revision.Number == patchSet {
			return sha
		}
	}
	return ""
}

type gerritCommentInfo struct {
	PatchSet int           `json:"patch_set"`
	Line     int           `json:"line"`
	Message  string        `json:"message"`
	Updated  gerritTime    `json:"updated"`
	Author   gerritAccount `json:"author"`
	Side     string        `json:"side"`
	Parent   int           `json:"parent"`
	Range    *struct {
		StartLine int `json:"start_line"`
		EndLine   int `json:"end_line"`
	} `json:"range"`
}

type gerritStatusError struct {
	Status int
	URL    string
}

func (e *gerritStatusError) isAuthError() bool {
	return e.Status == http.StatusUnauthorized || e.Status == http.StatusForbidden
}

func (e *gerritStatusError) Error() string {
	msg := fmt.Sprintf("gerrit API %s returned status %d", e.URL, e.Status)
	if e.isAuthError() {
		msg += ", check the credentials and the permission to read the project"
	}
	return msg
}

type gerritDecodeError struct {
	URL string
	Err error
}

func (e *gerritDecodeError) Error() string {
	return fmt.Sprintf("gerrit API %s returned an invalid response: %v", e.URL, e.Err)
}

func (e *gerritDecodeError) Unwrap() error { return e.Err }

func (d *GerritDownloader) apiRoot() string {
	if d.isAuthenticated() {
		return d.root + "/a"
	}
	return d.root
}

func (d *GerritDownloader) projectPath() string {
	return "/projects/" + url.PathEscape(d.project)
}

func (d *GerritDownloader) changePath(number int64) string {
	return "/changes/" + url.PathEscape(d.project) + "~" + strconv.FormatInt(number, 10)
}

func (d *GerritDownloader) cloneURL() string {
	segments := strings.Split(d.project, "/")
	for i := range segments {
		segments[i] = url.PathEscape(segments[i])
	}
	return d.apiRoot() + "/" + strings.Join(segments, "/")
}

func (d *GerritDownloader) changeURL(number int64) string {
	return d.root + "/c/" + d.project + "/+/" + strconv.FormatInt(number, 10)
}

// callAPI sends a GET request to the REST API of a resolved root
func (d *GerritDownloader) callAPI(ctx context.Context, root, path string, query url.Values, result any) error {
	endpoint := root + path
	apiURL := endpoint
	if len(query) > 0 {
		apiURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &gerritStatusError{Status: resp.StatusCode, URL: endpoint}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, gerritMaxResponseSize+1))
	if err != nil {
		return err
	}
	if len(data) > gerritMaxResponseSize {
		return errors.New("gerrit API response is too large")
	}
	data = bytes.TrimPrefix(bytes.TrimSpace(data), []byte(gerritXSSIPrefix))
	if err := json.Unmarshal(data, result); err != nil {
		return &gerritDecodeError{URL: endpoint, Err: err}
	}
	return nil
}

func (d *GerritDownloader) get(ctx context.Context, path string, query url.Values, result any) error {
	return d.callAPI(ctx, d.apiRoot(), path, query, result)
}

// resolve finds out which part of the clone URL path is the project by asking the server
func (d *GerritDownloader) resolve(ctx context.Context) error {
	for _, candidate := range d.location.candidates() {
		var info struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		apiRoot := candidate.root
		if d.isAuthenticated() {
			apiRoot += "/a"
		}
		err := d.callAPI(ctx, apiRoot, "/projects/"+url.PathEscape(candidate.project), nil, &info)
		if statusErr, ok := errors.AsType[*gerritStatusError](err); ok && statusErr.Status == http.StatusNotFound {
			continue
		}
		if _, ok := errors.AsType[*gerritDecodeError](err); ok {
			continue // another web application, such as an HTML page, answers on this path
		}
		if err != nil {
			return err
		}
		if info.Name != candidate.project {
			continue
		}
		d.root, d.project, d.description = candidate.root, candidate.project, info.Description
		return nil
	}
	return fmt.Errorf("no Gerrit project found for %s, check the clone URL and the permission to read the project", d.location)
}

// GetRepoInfo resolves the project, the migration calls it before any other method
func (d *GerritDownloader) GetRepoInfo(ctx context.Context) (*base.Repository, error) {
	if err := d.resolve(ctx); err != nil {
		return nil, err
	}

	// an empty project has no HEAD to resolve and the API answers 404
	var head string
	if err := d.get(ctx, d.projectPath()+"/HEAD", nil, &head); err != nil {
		if statusErr, ok := errors.AsType[*gerritStatusError](err); !ok || statusErr.Status != http.StatusNotFound {
			return nil, err
		}
	}

	return &base.Repository{
		Name:          d.project[strings.LastIndex(d.project, "/")+1:],
		Description:   d.description,
		CloneURL:      d.cloneURL(),
		OriginalURL:   d.root + "/admin/repos/" + url.PathEscape(d.project),
		DefaultBranch: strings.TrimPrefix(head, "refs/heads/"),
	}, nil
}

func (d *GerritDownloader) loadChangeNumbers(ctx context.Context) error {
	d.changeNumbers = nil
	query := url.Values{"q": {"project:" + strconv.Quote(d.project)}, "n": {strconv.Itoa(gerritMaxPageSize)}}
	for start := 0; ; {
		query.Set("S", strconv.Itoa(start))
		var page []struct {
			Number      int64 `json:"_number"`
			MoreChanges bool  `json:"_more_changes"`
		}
		if err := d.get(ctx, "/changes/", query, &page); err != nil {
			return err
		}
		for _, change := range page {
			d.changeNumbers = append(d.changeNumbers, change.Number)
		}
		start += len(page)
		if len(page) == 0 || !page[len(page)-1].MoreChanges {
			break
		}
	}
	// changes updated while paging move to the front of the list and can be returned twice
	slices.Sort(d.changeNumbers)
	d.changeNumbers = slices.Compact(d.changeNumbers)
	return nil
}

// GetPullRequests returns a page of changes, oldest first
func (d *GerritDownloader) GetPullRequests(ctx context.Context, page, perPage int) ([]*base.PullRequest, bool, error) {
	d.activities = map[int64]*gerritActivity{}
	page = max(page, 1)
	if perPage <= 0 {
		perPage = gerritMaxPageSize
	}
	if page == 1 {
		if err := d.loadChangeNumbers(ctx); err != nil {
			return nil, false, err
		}
	}

	start := min((page-1)*perPage, len(d.changeNumbers))
	end := min(start+perPage, len(d.changeNumbers))
	prs := make([]*base.PullRequest, 0, end-start)
	for _, number := range d.changeNumbers[start:end] {
		pr, err := d.getPullRequest(ctx, number)
		if err != nil {
			return nil, false, err
		}
		prs = append(prs, pr)
	}
	return prs, end >= len(d.changeNumbers), nil
}

func (d *GerritDownloader) getPullRequest(ctx context.Context, number int64) (*base.PullRequest, error) {
	change := &gerritChange{}
	query := url.Values{"o": {"ALL_REVISIONS", "CURRENT_REVISION", "ALL_COMMITS", "MESSAGES", "DETAILED_ACCOUNTS"}}
	if err := d.get(ctx, d.changePath(number), query, change); err != nil {
		return nil, err
	}

	current := change.Revisions[change.CurrentRevision]
	// the target branch tip moves on after a merge, the parent of the patch set is the base the reviewers saw
	baseSHA := ""
	if len(current.Commit.Parents) > 0 {
		baseSHA = current.Commit.Parents[0].Commit
	}
	pr := &base.PullRequest{
		FlowType:     base.PRFlowAGit,
		Number:       change.Number,
		ForeignIndex: change.Number,
		Title:        change.Subject,
		Content:      d.changeDescription(change, current.Commit.Message),
		PosterID:     change.Owner.ID,
		PosterName:   change.Owner.displayName(),
		PosterEmail:  change.Owner.Email,
		State:        "open",
		Created:      change.Created.Time,
		Updated:      change.Updated.Time,
		IsDraft:      change.WorkInProgress || change.Status == "DRAFT",
		Head:         base.PullRequestBranch{Ref: current.Ref, SHA: change.CurrentRevision},
		Base:         base.PullRequestBranch{Ref: change.Branch, SHA: baseSHA},
		Context:      change,
	}
	switch change.Status {
	case "MERGED":
		merged := cmp.Or(change.Submitted.Time, change.Updated.Time)
		pr.State, pr.Merged, pr.MergedTime, pr.Closed = "closed", true, &merged, &merged
		pr.MergeCommitSHA = change.CurrentRevision
	case "ABANDONED":
		closed := change.Updated.Time
		pr.State, pr.Closed = "closed", &closed
	}
	_ = CheckAndEnsureSafePR(pr, d.root, d)
	return pr, nil
}

func (d *GerritDownloader) changeDescription(change *gerritChange, message string) string {
	body := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(message), change.Subject))
	footer := fmt.Sprintf("Imported from Gerrit change %s\nChange-Id: %s\nBranch: %s\nStatus: %s",
		d.changeURL(change.Number), change.ChangeID, change.Branch, change.Status)
	if body == "" {
		return footer
	}
	return body + "\n\n---\n" + footer
}

// gerritActivity is what the messages and comments of a change are converted to
type gerritActivity struct {
	reviews  []*base.Review
	comments []*base.Comment
}

func (d *GerritDownloader) getActivity(ctx context.Context, foreignIndex int64, downloaderContext base.DownloaderContext) (*gerritActivity, error) {
	if activity, ok := d.activities[foreignIndex]; ok {
		return activity, nil
	}
	change, ok := downloaderContext.(*gerritChange)
	if !ok || change.Number != foreignIndex {
		return nil, fmt.Errorf("the downloader context of gerrit change %d is missing", foreignIndex)
	}
	var byPath map[string][]gerritCommentInfo
	if err := d.get(ctx, d.changePath(foreignIndex)+"/comments", nil, &byPath); err != nil {
		return nil, err
	}
	activity := buildGerritActivity(change, byPath)
	d.activities[foreignIndex] = activity
	return activity, nil
}

// GetComments returns the messages without a patch set and the inline comments that cannot be anchored
func (d *GerritDownloader) GetComments(ctx context.Context, commentable base.Commentable) ([]*base.Comment, bool, error) {
	activity, err := d.getActivity(ctx, commentable.GetForeignIndex(), commentable.GetContext())
	if err != nil {
		return nil, false, err
	}
	for _, comment := range activity.comments {
		comment.IssueIndex = commentable.GetLocalIndex()
	}
	return activity.comments, true, nil
}

// GetReviews returns a review for each message of a patch set, with the inline comments anchored to the current patch set
func (d *GerritDownloader) GetReviews(ctx context.Context, reviewable base.Reviewable) ([]*base.Review, error) {
	activity, err := d.getActivity(ctx, reviewable.GetForeignIndex(), reviewable.GetContext())
	if err != nil {
		return nil, err
	}
	for _, review := range activity.reviews {
		review.IssueIndex = reviewable.GetLocalIndex()
	}
	return activity.reviews, nil
}

type gerritVote struct {
	label string
	value int
}

// parseGerritVotes reads the header of a message of a patch set, "Patch Set 3: Code-Review+1 Verified-1"
func parseGerritVotes(message string) (patchSet int, votes []gerritVote) {
	header, _, _ := strings.Cut(message, "\n")
	rest, ok := strings.CutPrefix(header, "Patch Set ")
	if !ok {
		return 0, nil
	}
	number, labels, _ := strings.Cut(rest, ":")
	patchSet, err := strconv.Atoi(number)
	if err != nil {
		return 0, nil
	}
	for token := range strings.FieldsSeq(labels) {
		cut := strings.LastIndexAny(token, "+-")
		if cut <= 0 {
			continue
		}
		value, err := strconv.Atoi(token[cut:])
		if err != nil {
			continue
		}
		votes = append(votes, gerritVote{label: token[:cut], value: value})
	}
	return patchSet, votes
}

// gerritReviewState maps Code-Review votes, +1 only means that someone else must approve
func gerritReviewState(votes []gerritVote) string {
	state := base.ReviewStateCommented
	for _, vote := range votes {
		if vote.label != "Code-Review" {
			continue
		}
		switch {
		case vote.value >= 2:
			state = base.ReviewStateApproved
		case vote.value < 0:
			state = base.ReviewStateChangesRequested
		}
	}
	return state
}

type gerritReviewKey struct {
	accountID int64
	patchSet  int
	at        int64
}

func buildGerritActivity(change *gerritChange, byPath map[string][]gerritCommentInfo) *gerritActivity {
	activity := &gerritActivity{}
	reviewsByKey := map[gerritReviewKey]*base.Review{}

	for _, message := range change.Messages {
		at := cmp.Or(message.Date.Time, change.Updated.Time)
		headerPatchSet, votes := parseGerritVotes(message.Message)
		patchSet := cmp.Or(message.RevisionNumber, headerPatchSet)
		sha := change.revision(patchSet)
		if sha == "" || (strings.HasPrefix(message.Tag, "autogenerated:") && len(votes) == 0) {
			activity.comments = append(activity.comments, &base.Comment{
				PosterID:    message.Author.ID,
				PosterName:  message.Author.displayName(),
				PosterEmail: message.Author.Email,
				Created:     at,
				Updated:     at,
				Content:     message.Message,
			})
			continue
		}
		review := &base.Review{
			ReviewerID:   message.Author.ID,
			ReviewerName: message.Author.displayName(),
			CommitID:     sha,
			Content:      message.Message,
			CreatedAt:    at,
			State:        gerritReviewState(votes),
		}
		activity.reviews = append(activity.reviews, review)
		reviewsByKey[gerritReviewKey{message.Author.ID, patchSet, at.Unix()}] = review
	}

	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		for i := range byPath[path] {
			activity.addComment(change, path, &byPath[path][i], reviewsByKey)
		}
	}

	slices.SortStableFunc(activity.reviews, func(a, b *base.Review) int { return a.CreatedAt.Compare(b.CreatedAt) })
	slices.SortStableFunc(activity.comments, func(a, b *base.Comment) int { return a.Created.Compare(b.Created) })
	return activity
}

func (a *gerritActivity) addComment(change *gerritChange, path string, c *gerritCommentInfo, reviewsByKey map[gerritReviewKey]*base.Review) {
	at := cmp.Or(c.Updated.Time, change.Updated.Time)
	text := cmp.Or(c.Message, "(no text)")
	ranged := c.Range != nil && c.Range.StartLine > 0 && c.Range.StartLine < c.Range.EndLine

	// the diff of a pull request is the current patch set against its parent, Gerrit's magic files start with "/"
	if c.PatchSet == change.Revisions[change.CurrentRevision].Number && !strings.HasPrefix(path, "/") && c.Line > 0 && c.Parent == 0 {
		line := c.Line
		if c.Side == "PARENT" {
			line = -line
		}
		if ranged {
			text = fmt.Sprintf("Lines %d-%d:\n\n%s", c.Range.StartLine, c.Range.EndLine, text)
		}
		reviewComment := &base.ReviewComment{
			Content:   text,
			TreePath:  path,
			Line:      line,
			CommitID:  change.CurrentRevision,
			PosterID:  c.Author.ID,
			CreatedAt: at,
			UpdatedAt: at,
		}
		key := gerritReviewKey{c.Author.ID, c.PatchSet, at.Unix()}
		review, ok := reviewsByKey[key]
		if !ok {
			review = &base.Review{
				ReviewerID:   c.Author.ID,
				ReviewerName: c.Author.displayName(),
				CommitID:     change.CurrentRevision,
				CreatedAt:    at,
				State:        base.ReviewStateCommented,
			}
			reviewsByKey[key] = review
			a.reviews = append(a.reviews, review)
		}
		review.Comments = append(review.Comments, reviewComment)
		return
	}

	var where strings.Builder
	if path == "/PATCHSET_LEVEL" {
		where.WriteString("Patch set level comment")
	} else {
		fmt.Fprintf(&where, "Comment on `%s`", path)
		switch {
		case ranged:
			fmt.Fprintf(&where, " lines %d-%d", c.Range.StartLine, c.Range.EndLine)
		case c.Line > 0:
			fmt.Fprintf(&where, " line %d", c.Line)
		}
		if c.Side == "PARENT" || c.Parent > 0 {
			where.WriteString(" of the parent")
		}
	}
	if sha := change.revision(c.PatchSet); sha != "" {
		fmt.Fprintf(&where, " (patch set %d, revision %s)", c.PatchSet, sha)
	} else {
		fmt.Fprintf(&where, " (unknown patch set %d)", c.PatchSet)
	}
	a.comments = append(a.comments, &base.Comment{
		PosterID:    c.Author.ID,
		PosterName:  c.Author.displayName(),
		PosterEmail: c.Author.Email,
		Created:     at,
		Updated:     at,
		Content:     where.String() + ":\n\n" + text,
	})
}
