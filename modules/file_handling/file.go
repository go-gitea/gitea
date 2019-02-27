package file_handling

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/sdk/gitea"
	"net/url"
	"time"
)

func GetFileResponseFromCommit(repo *models.Repository, commit *git.Commit, treeName string) (*gitea.FileResponse, error) {
	fileContents, _ := GetFileContents(repo, treeName, commit.ID.String()) // of if fails, then will be nil
	fileCommitResponse, _ := GetFileCommitResponse(repo, commit)           // ok if fails, then will be nil
	verification := GetPayloadCommitVerification(commit)
	fileResponse := &gitea.FileResponse{
		Content:      fileContents,
		Commit:       fileCommitResponse,
		Verification: verification,
	}
	return fileResponse, nil
}

func GetFileCommitResponse(repo *models.Repository, commit *git.Commit) (*gitea.FileCommitResponse, error) {
	commitURL, _ := url.Parse(repo.APIURL() + "/git/commits/" + commit.ID.String())
	commitTreeURL, _ := url.Parse(repo.APIURL() + "/git/trees/" + commit.Tree.ID.String())
	parents := make([]gitea.CommitMeta, commit.ParentCount())
	for i := 0; i <= commit.ParentCount(); i++ {
		if parent, err := commit.Parent(i); err == nil && parent != nil {
			parentCommitURL, _ := url.Parse(repo.APIURL() + "/git/commits/" + parent.ID.String())
			parents[i] = gitea.CommitMeta{
				SHA: parent.ID.String(),
				URL: parentCommitURL.String(),
			}
		}
	}
	commitHtmlURL, _ := url.Parse(repo.HTMLURL() + "/commit/" + commit.ID.String())
	fileCommit := &gitea.FileCommitResponse{
		CommitMeta: &gitea.CommitMeta{
			SHA: commit.ID.String(),
			URL: commitURL.String(),
		},
		HTMLURL: commitHtmlURL.String(),
		Author: &gitea.CommitUser{
			Date:  commit.Author.When.UTC().Format(time.RFC3339),
			Name:  commit.Author.Name,
			Email: commit.Author.Email,
		},
		Committer: &gitea.CommitUser{
			Date:  commit.Committer.When.UTC().Format(time.RFC3339),
			Name:  commit.Committer.Name,
			Email: commit.Committer.Email,
		},
		Message: commit.Message(),
		Tree: &gitea.CommitMeta{
			URL: commitTreeURL.String(),
			SHA: commit.Tree.ID.String(),
		},
		Parents: &parents,
	}
	return fileCommit, nil
}
