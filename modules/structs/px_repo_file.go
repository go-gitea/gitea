package structs

import "time"

// CommitContentsResponse contains information about a repo's entry's (dir, file, symlink, submodule) metadata and content and commit
type CommitContentsResponse struct {
	URL         *string `json:"url"`
	GitURL      *string `json:"git_url"`
	HTMLURL     *string `json:"html_url"`
	DownloadURL *string `json:"download_url"`
	IsNonText   *bool   `json:"is_non_text"`

	Name              string    `json:"name"`
	Path              string    `json:"path"`
	SHA               string    `json:"sha"`
	LastCommitSHA     string    `json:"last_commit_sha"`
	LastCommitMessage string    `json:"last_commit_message"`
	LastCommitCreate  time.Time `json:"last_commit_create"`

	// `type` will be `file`, `dir`, `symlink`, or `submodule`
	Type  string `json:"type"`
	Size  int64  `json:"size"`
	IsLFS bool   `json:"is_lfs"`

	// `encoding` is populated when `type` is `file`, otherwise null
	Encoding *string `json:"encoding"`

	// `content` is populated when `type` is `file`, otherwise null
	Content *string `json:"content"`

	// `target` is populated when `type` is `symlink`, otherwise null
	Target *string `json:"target"`

	// `submodule_git_url` is populated when `type` is `submodule`, otherwise null
	SubmoduleGitURL *string            `json:"submodule_git_url"`
	Links           *FileLinksResponse `json:"_links"`
}
