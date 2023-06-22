package contributors

import (
	"context"
	"fmt"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	util "code.gitea.io/gitea/modules/util"
)

// CreateWeeks converts list of sundays to list of *api.WeekData
func CreateWeeks(sundays []int64) []*api.WeekData {
	var weeks []*api.WeekData
	for _, week := range sundays {
		weeks = append(weeks, &api.WeekData{
			Week:      week,
			Additions: 0,
			Deletions: 0,
			Commits:   0,
		},
		)
	}
	return weeks
}

// GetContributorStats returns contributors stats for git commits for given revision or default branch
func GetContributorStats(ctx context.Context, repo *repo_model.Repository, revision string) (map[string]*api.ContributorData, error) {
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo.RepoPath())
	if err != nil {
		return nil, fmt.Errorf("OpenRepository: %w", err)
	}
	defer closer.Close()

	if len(revision) == 0 {
		revision = repo.DefaultBranch
	}
	extended_commit_stats, err := gitRepo.ExtendedCommitStats(revision)
	if err != nil {
		return nil, fmt.Errorf("ExtendedCommitStats: %w", err)
	}

	layout := "2006-01-02"
	initial_commit_date := extended_commit_stats[0].Author.Date

	starting_sunday, _ := util.FindLastSundayBeforeDate(initial_commit_date)
	ending_sunday, _ := util.FindFirstSundayAfterDate(time.Now().Format(layout))

	sundays, _ := util.ListSundaysBetween(starting_sunday, ending_sunday)

	unknownUserAvatarLink := user_model.NewGhostUser().AvatarLink(ctx)
	contributors_commit_stats := make(map[string]*api.ContributorData)
	contributors_commit_stats["Total"] = &api.ContributorData{
		Name:       "Total",
		AvatarLink: unknownUserAvatarLink,
		Weeks:      CreateWeeks(sundays),
	}
	total, _ := contributors_commit_stats["Total"]

	for _, v := range extended_commit_stats {
		if len(v.Author.Email) == 0 {
			continue
		}
		if _, ok := contributors_commit_stats[v.Author.Email]; !ok {
			u, err := user_model.GetUserByEmail(ctx, v.Author.Email)
			if u == nil || user_model.IsErrUserNotExist(err) {
				contributors_commit_stats[v.Author.Email] = &api.ContributorData{
					Name:       v.Author.Name,
					AvatarLink: unknownUserAvatarLink,
					Weeks:      CreateWeeks(sundays),
				}
			} else {
				contributors_commit_stats[v.Author.Email] = &api.ContributorData{
					Name:       u.DisplayName(),
					Login:      u.LowerName,
					AvatarLink: u.AvatarLink(ctx),
					HomeLink:   u.HomeLink(),
					Weeks:      CreateWeeks(sundays),
				}
			}
		}
		// Update user statistics
		user, _ := contributors_commit_stats[v.Author.Email]
		starting_of_week, _ := util.FindLastSundayBeforeDate(v.Author.Date)

		val, _ := time.Parse(layout, starting_of_week)
		starting_sunday_p, _ := time.Parse(layout, starting_sunday)
		idx := int(val.Sub(starting_sunday_p).Hours()/24) / 7
		user.Weeks[idx].Additions += v.Stats.Additions
		user.Weeks[idx].Deletions += v.Stats.Deletions
		user.Weeks[idx].Commits++
		user.TotalCommits++

		// Update overall statistics
		total.Weeks[idx].Additions += v.Stats.Additions
		total.Weeks[idx].Deletions += v.Stats.Deletions
		total.Weeks[idx].Commits++
		total.TotalCommits++
	}

	return contributors_commit_stats, nil
}
