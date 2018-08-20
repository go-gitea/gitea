// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"container/list"
	"fmt"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/util"
	"github.com/go-xorm/xorm"
)

func reformatAndAddReferenceComments(x *xorm.Engine) error {
	const batchSize = 100

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	var maxCommentID int64

	// create an repo lookup to map the repo's full name to the repository object
	var repos []*models.Repository
	repoLookup := make(map[string]*models.Repository)
	if err := x.Find(&repos); err != nil {
		return err
	}
	for _, repo := range repos {
		repoLookup[repo.FullName()] = repo
	}

	// create an issue updated lookup to save the current UpdateUnix values since UpdatedUnix gets updated as we migrate
	var issues []*models.Issue
	issueUpdatedLookup := make(map[int64]util.TimeStamp)
	if err := x.Find(&issues); err != nil {
		return err
	}
	for _, issue := range issues {
		issueUpdatedLookup[issue.ID] = issue.UpdatedUnix
	}

	// convert or remove old-style commit reference comments
	for refCommentStart := 0; ; refCommentStart += batchSize {
		refComments := make([]*models.Comment, 0, batchSize)
		if err := x.Where("comment.type = ?", models.CommentTypeCommitRef).Limit(batchSize, refCommentStart).Find(&refComments); err != nil {
			return err
		}
		if len(refComments) == 0 {
			break
		}

		for _, refComment := range refComments {
			if err := refComment.LoadReference(); err != nil {
				// if load failed, parse the content into a repo link and a commit SHA...
				if strings.HasPrefix(refComment.Content, `<a href="`) && strings.HasSuffix(refComment.Content, `</a>`) {
					content := strings.TrimSuffix(strings.TrimPrefix(refComment.Content, `<a href="`), `</a>`)
					contentParts := strings.SplitN(content, `">`, 2)
					if len(contentParts) == 2 {
						linkParts := strings.SplitN(contentParts[0], `/commit/`, 2)
						if len(linkParts) == 2 && len(linkParts[1]) == 40 {
							repoLinkParts := strings.Split(linkParts[0], "/")
							commitSha1 := linkParts[1]
							// ...then check if the repo/commit exist...
							if len(repoLinkParts) >= 2 {
								repoFullName := strings.Join(repoLinkParts[len(repoLinkParts)-2:], "/")
								refRepo := repoLookup[repoFullName]
								if refRepo != nil {
									gitRepo, err := git.OpenRepository(refRepo.RepoPath())
									if err == nil {
										refCommit, err := gitRepo.GetCommit(commitSha1)
										if err == nil {
											// ...and update the ref comment to the new-style
											refComment.Content = fmt.Sprintf("%d %s", refRepo.ID, refCommit.ID.String())
											refComment.CommitSHA = base.EncodeSha1(refComment.Content)

											if _, err := x.ID(refComment.ID).AllCols().Update(refComment); err == nil {
												// continue if successful in order to skip over the delete below
												continue
											}
										}
									}
								}
							}
						}
					}
				}

				// delete the comment if unable to convert it
				if _, err := x.Delete(refComment); err != nil {
					return err
				}
			}
		}
	}

	// for every issue and every comment, try to process the title+contents as ref only, no open/close action
	for issueStart := 0; ; issueStart += batchSize {
		issues = make([]*models.Issue, 0, batchSize)
		if err := x.Limit(batchSize, issueStart).Find(&issues); err != nil {
			return err
		}
		if len(issues) == 0 {
			break
		}

		if err := models.IssueList(issues).LoadAttributes(); err != nil {
			continue
		}

		for _, issue := range issues {
			if _, err := x.Table("comment").Select("coalesce(max(comment.id), 0)").Get(&maxCommentID); err != nil {
				return err
			}

			if err := models.UpdateIssuesComment(issue.Poster, issue.Repo, issue, nil, false); err != nil {
				continue
			}

			// try to handle the commit messages if this is a pull request
			for once := true; once && issue.IsPull; once = false {
				var (
					err         error
					pr          *models.PullRequest
					baseBranch  *models.Branch
					headBranch  *models.Branch
					baseCommit  *git.Commit
					headCommit  *git.Commit
					baseGitRepo *git.Repository
				)

				if pr, err = issue.GetPullRequest(); err != nil {
					continue
				}

				if err = pr.GetBaseRepo(); err != nil {
					continue
				}

				if err = pr.GetHeadRepo(); err != nil {
					continue
				}

				if baseBranch, err = pr.BaseRepo.GetBranch(pr.BaseBranch); err != nil {
					continue
				}
				if baseCommit, err = baseBranch.GetCommit(); err != nil {
					continue
				}
				if headBranch, err = pr.HeadRepo.GetBranch(pr.HeadBranch); err != nil {
					continue
				}
				if headCommit, err = headBranch.GetCommit(); err != nil {
					continue
				}
				if baseGitRepo, err = git.OpenRepository(pr.BaseRepo.RepoPath()); err != nil {
					continue
				}

				mergeBase, err := baseGitRepo.GetMergeBase(baseCommit.ID.String(), headCommit.ID.String())
				if err != nil {
					continue
				}

				l, err := baseGitRepo.CommitsBetweenIDs(headCommit.ID.String(), mergeBase)
				if err != nil {
					continue
				}

				commits := models.ListToPushCommits(l)
				if err := models.UpdateIssuesCommit(issue.Poster, pr.BaseRepo, commits.Commits, false); err != nil {
					continue
				}
			}

			if _, err := x.Exec("UPDATE `comment` SET `created_unix` = ?, `updated_unix` = ? WHERE `id` > ?", issueUpdatedLookup[issue.ID], issueUpdatedLookup[issue.ID], maxCommentID); err != nil {
				return err
			}

			// try to handle all comments on this issue
			for commentStart := 0; ; commentStart += batchSize {
				comments := make([]*models.Comment, 0, batchSize)
				if err := x.Limit(batchSize, commentStart).Find(&comments); err != nil {
					return err
				}
				if len(comments) == 0 {
					break
				}

				for _, comment := range comments {
					if comment.Type != models.CommentTypeComment && comment.Type != models.CommentTypeCode && comment.Type != models.CommentTypeReview {
						continue
					}

					if _, err := x.Table("comment").Select("coalesce(max(comment.id), 0)").Get(&maxCommentID); err != nil {
						return err
					}

					if err := comment.LoadIssue(); err != nil {
						continue
					}

					if err := models.UpdateIssuesComment(comment.Poster, issue.Repo, issue, comment, false); err != nil {
						continue
					}

					if _, err := x.Exec("UPDATE `comment` SET `created_unix` = ?, `updated_unix` = ? WHERE `id` > ?", comment.UpdatedUnix, comment.UpdatedUnix, maxCommentID); err != nil {
						return err
					}
				}
			}
		}
	}

	// for every repo, try to process the commits an all branches for issue references
	for _, repo := range repoLookup {
		if _, err := x.Table("comment").Select("coalesce(max(comment.id), 0)").Get(&maxCommentID); err != nil {
			return err
		}

		var (
			err           error
			gitRepo       *git.Repository
			branches      []*models.Branch
			defaultBranch *models.Branch
			defaultCommit *git.Commit
		)

		if gitRepo, err = git.OpenRepository(repo.RepoPath()); err != nil {
			continue
		}

		if defaultBranch, err = repo.GetBranch(repo.DefaultBranch); err != nil {
			continue
		}
		if defaultCommit, err = defaultBranch.GetCommit(); err != nil {
			continue
		}

		if branches, err = repo.GetBranches(); err != nil {
			continue
		}

		for _, branch := range branches {
			var branchCommit *git.Commit
			if branchCommit, err = branch.GetCommit(); err != nil {
				continue
			}

			var l *list.List
			if branch.Name == repo.DefaultBranch {
				l, err = branchCommit.CommitsBefore()
			} else {
				l, err = gitRepo.CommitsBetweenIDs(branchCommit.ID.String(), defaultCommit.ID.String())
			}
			if err != nil {
				continue
			}

			commits := models.ListToPushCommits(l)
			if err := models.UpdateIssuesCommit(repo.MustOwner(), repo, commits.Commits, false); err != nil {
				continue
			}
		}

		if _, err := x.Exec("UPDATE `comment` SET `created_unix` = ?, `updated_unix` = ? WHERE `id` > ?", repo.UpdatedUnix, repo.UpdatedUnix, maxCommentID); err != nil {
			return err
		}
	}

	return sess.Commit()
}
