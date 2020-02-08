// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"crypto/sha1"
	"fmt"
	"io"
	"sort"
	"testing"

	"code.gitea.io/gitea/modules/log"

	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
)

// IMPORTANT: THIS FILE IS ONLY A STEPPING STONE TO HELP TEST THE FEATURE
// DURING DEVELOPMENT. IT'S NOT INTENDED TO GO LIKE THIS IN THE FINAL
// VERSION OF THE PR.

type sumdata struct {
	Count int
	Type  int
	Mode  int
}

// UserRepoUnitTest FIXME: remove export
func UserRepoUnitTest(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	assert.NoError(t, UserRepoUnitTestDo(x))
}

// UserRepoUnitTestDo is a temporary function for testing during development
func UserRepoUnitTestDo(x *xorm.Engine) error {

	var err error

	if err = RebuildAllUserRepoUnits(x); err != nil {
		return fmt.Errorf("RebuildAllUserRepoUnits: %v", err)
	}

	sharepo, usercntrepo, repocntrepo, err := getUserRepoUnitsSha(x, "RebuildAllUserRepoUnits")
	if err != nil {
		return fmt.Errorf("getUserRepoUnitsSha: %v", err)
	}

	duser, drepo := int64(0), int64(0)

	dumpUserOrRepo(x, "RebuildAllUserRepoUnits", duser, drepo)

	if err = batchBuildByUsersTest(x); err != nil {
		return fmt.Errorf("batchBuildByUsersTest: %v", err)
	}

	shaother, usercntother, repocntother, err := getUserRepoUnitsSha(x, "batchBuildByUsersTest")
	if err != nil {
		return fmt.Errorf("getUserRepoUnitsSha: %v", err)
	}

	dumpUserOrRepo(x, "batchBuildByUsersTest", duser, drepo)

	if err = compareShas(sharepo, shaother, "BuildByUsers", usercntrepo, repocntrepo, usercntother, repocntother); err != nil {
		return err
	}

	if err = batchBuildByReposUsersTest(x); err != nil {
		return fmt.Errorf("batchBuildByReposUsersTest: %v", err)
	}

	shaother, usercntother, repocntother, err = getUserRepoUnitsSha(x, "batchBuildByReposUsersTest")
	if err != nil {
		return fmt.Errorf("getUserRepoUnitsSha: %v", err)
	}

	dumpUserOrRepo(x, "batchBuildByReposUsersTest", duser, drepo)

	if err = compareShas(sharepo, shaother, "BuildByRepoUsers", usercntrepo, repocntrepo, usercntother, repocntother); err != nil {
		return err
	}

	if err = batchRebuildByTeamsTest(x, sharepo, usercntrepo, repocntrepo); err != nil {
		return fmt.Errorf("batchRebuildByTeams: %v", err)
	}

	shaother, usercntother, repocntother, err = getUserRepoUnitsSha(x, "batchRebuildByTeamsTest")
	if err != nil {
		return fmt.Errorf("getUserRepoUnitsSha: %v", err)
	}

	dumpUserOrRepo(x, "batchRebuildByTeamsTest", duser, drepo)

	return compareShas(sharepo, shaother, "RebuildByTeams", usercntrepo, repocntrepo, usercntother, repocntother)
}

func compareShas(sharepo, shaother, othername string,
	usercntrepo, repocntrepo, usercntother, repocntother map[int64]*sumdata) error {
	if sharepo == shaother {
		return nil
	}

	users1 := orderMapKeys(usercntrepo)
	for _, id := range users1 {
		pr, okr := usercntrepo[id]
		po, oko := usercntother[id]
		if !okr {
			log.Info("User %d not in repo list", id)
		} else if !oko {
			log.Info("User %d not in %s list", id, othername)
		} else if pr.Count != po.Count ||
			pr.Type != po.Type ||
			pr.Mode != po.Mode {
			log.Info("User %d %s %d,%d,%d != repo %d,%d,%d", id, othername,
				po.Count, po.Type, po.Mode,
				pr.Count, pr.Type, pr.Mode)
		}
	}
	users2 := orderMapKeys(usercntother)
	for _, id := range users2 {
		_, okr := usercntrepo[id]
		_, oko := usercntother[id]
		if !okr {
			log.Info("User %d not in repo list", id)
		} else if !oko {
			log.Info("User %d not in %s list", id, othername)
		}
	}
	repos1 := orderMapKeys(repocntrepo)
	for _, id := range repos1 {
		pr, okr := repocntrepo[id]
		po, oko := repocntother[id]
		if !okr {
			log.Info("Repo %d not in repo list", id)
		} else if !oko {
			log.Info("Repo %d not in %s list", id, othername)
		} else if pr.Count != po.Count ||
			pr.Type != po.Type ||
			pr.Mode != po.Mode {
			log.Info("Repo %d %s %d,%d,%d != repo %d,%d,%d", id, othername,
				po.Count, po.Type, po.Mode,
				pr.Count, pr.Type, pr.Mode)
		}
	}
	repos2 := orderMapKeys(repocntother)
	for _, id := range repos2 {
		_, okr := repocntrepo[id]
		_, oko := repocntother[id]
		if !okr {
			log.Info("Repo %d not in repo list", id)
		} else if !oko {
			log.Info("Repo %d not in %s list", id, othername)
		}
	}

	return fmt.Errorf("build by repo and by %s don't yield the same results", othername)
}

func batchBuildByUsersTest(x *xorm.Engine) error {

	// Don't get too greedy on the batches
	const userBatchCount = 20

	if _, err := x.Exec("DELETE FROM user_repo_unit"); err != nil {
		return fmt.Errorf("addUserRepoUnit: DELETE old data: %v", err)
	}

	var maxid int64
	if _, err := x.Table("user").Select("MAX(id)").Get(&maxid); err != nil {
		return fmt.Errorf("addUserRepoUnit: get MAX(user_id): %v", err)
	}

	// Create access data for the first time
	for i := int64(1); i <= maxid; i += userBatchCount {
		if err := rangeBuildUserUnitsTest(x, i, userBatchCount); err != nil {
			return fmt.Errorf("rangeBuildUserUnitsTest(%d,%d): %v", i, userBatchCount, err)
		}
	}

	// Use a single transaction for the batch
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := RebuildAdminUnits(sess); err != nil {
		return fmt.Errorf("RebuildAdminUnits: %v", err)
	}

	if err := RebuildLoggedInUnits(sess); err != nil {
		return fmt.Errorf("RebuildLoggedInUnits: %v", err)
	}

	if err := RebuildAnonymousUnits(sess); err != nil {
		return fmt.Errorf("RebuildAnonymousUnits: %v", err)
	}

	return sess.Commit()
}

func batchBuildByReposUsersTest(x *xorm.Engine) error {

	if _, err := x.Exec("DELETE FROM user_repo_unit"); err != nil {
		return fmt.Errorf("batchBuildByReposUsersTest: DELETE old data: %v", err)
	}

	var maxuserid int64
	if _, err := x.Table("user").Select("MAX(id)").Get(&maxuserid); err != nil {
		return fmt.Errorf("batchBuildByReposUsersTest: get MAX(user_id): %v", err)
	}

	var maxrepoid int64
	if _, err := x.Table("repository").Select("MAX(id)").Get(&maxrepoid); err != nil {
		return fmt.Errorf("batchBuildByReposUsersTest: get MAX(repo_id): %v", err)
	}

	// Create access data for the first time
	for u := int64(1); u <= maxuserid; u++ {
		for r := int64(1); r <= maxrepoid; r++ {
			if err := batchBuildUserRepoUnitsTest(x, u, r); err != nil {
				return fmt.Errorf("batchBuildUserRepoUnitsTest(%d,%d): %v", u, r, err)
			}
		}
	}

	// Use a single transaction for the batch
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := RebuildAdminUnits(sess); err != nil {
		return fmt.Errorf("RebuildAdminUnits: %v", err)
	}

	if err := RebuildLoggedInUnits(sess); err != nil {
		return fmt.Errorf("RebuildLoggedInUnits: %v", err)
	}

	if err := RebuildAnonymousUnits(sess); err != nil {
		return fmt.Errorf("RebuildAnonymousUnits: %v", err)
	}

	return sess.Commit()
}

func batchRebuildByTeamsTest(x *xorm.Engine, sharepo string, usercntrepo, repocntrepo map[int64]*sumdata) error {

	var maxteamid int64
	if _, err := x.Table("team").Select("MAX(id)").Get(&maxteamid); err != nil {
		return fmt.Errorf("batchRebuildByTeamsTest: get MAX(team_id): %v", err)
	}

	// dumpUserOrRepo(x, "batchRebuildByTeamsTest(before)", -2, 0)

	for id := int64(1); id <= maxteamid; id++ {
		log.Info("Rebuilding team %d", id)
		if err := batchRebuildTeamTest(x, id); err != nil {
			return fmt.Errorf("batchRebuildTeamTest(%d): %v", id, err)
		}

		desc := fmt.Sprintf("RebuildTeam(%d)", id)
		shaother, usercntother, repocntother, err := getUserRepoUnitsSha(x, desc)
		if err != nil {
			return fmt.Errorf("getUserRepoUnitsSha: %v", err)
		}

		// dumpUserOrRepo(x, desc, -2, 0)

		if err = compareShas(sharepo, shaother, desc, usercntrepo, repocntrepo, usercntother, repocntother); err != nil {
			return err
		}
	}
	return nil
}

func rangeBuildUserUnitsTest(x *xorm.Engine, fromID int64, count int) error {
	// Use a single transaction for the batch
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	users := make([]*User, 0, count)
	if err := sess.Where("id BETWEEN ? AND ?", fromID, fromID+int64(count-1)).Find(&users); err != nil {
		return fmt.Errorf("Find repositories: %v", err)
	}

	// Some ID ranges might be empty
	if len(users) == 0 {
		return nil
	}

	for _, user := range users {
		if err := RebuildUserUnits(sess, user); err != nil {
			return fmt.Errorf("RebuildUserUnits(%d): %v", user.ID, err)
		}
	}

	return sess.Commit()
}

func batchBuildUserRepoUnitsTest(x *xorm.Engine, userID, repoID int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	user := &User{ID: userID}
	repo := &Repository{ID: repoID}

	if has, err := sess.Get(user); !has || err != nil {
		return err
	}

	if has, err := sess.Get(repo); !has || err != nil {
		return err
	}

	if err := RebuildUserRepoUnits(sess, user, repo); err != nil {
		return fmt.Errorf("RebuildUserRepoUnits(%d, %d): %v", user.ID, repo.ID, err)
	}

	return sess.Commit()
}

func batchRebuildTeamTest(x *xorm.Engine, teamID int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	team := &Team{ID: teamID}

	if has, err := sess.Get(team); !has || err != nil {
		return err
	}

	if err := RebuildTeamUnits(sess, team, false); err != nil {
		return fmt.Errorf("RebuildTeamUnits(%d): %v", team.ID, err)
	}

	return sess.Commit()
}

// getUserRepoUnitsSha this function is useful to check the generation
// of the user_repo_unit table by different means.
func getUserRepoUnitsSha(x *xorm.Engine, source string) (string, map[int64]*sumdata, map[int64]*sumdata, error) {
	type totdata struct {
		User  int64
		Repo  int64
		Count int
		Type  int
		Mode  int
	}
	data := make([]*UserRepoUnit, 0, 1024)
	usercnt := make(map[int64]*sumdata)
	repocnt := make(map[int64]*sumdata)
	if err := x.Table("user_repo_unit").
		OrderBy("user_id, repo_id, type, mode").
		Find(&data); err != nil {
		return "", nil, nil, fmt.Errorf("Find user_repo_unit: %v", err)
	}
	var (
		sum  totdata
		pair *sumdata
		ok   bool
	)

	h := sha1.New()
	for _, u := range data {
		_, _ = io.WriteString(h, fmt.Sprintf("%d,%d,%d,%d", u.UserID, u.RepoID, u.Type, u.Mode))
		sum.Count++
		sum.User += u.UserID
		sum.Repo += u.RepoID
		sum.Type += int(u.Type)
		sum.Mode += int(u.Mode)
		if pair, ok = usercnt[u.UserID]; !ok {
			pair = &sumdata{}
			usercnt[u.UserID] = pair
		}
		pair.Count++
		pair.Type += int(u.Type)
		pair.Mode += int(u.Mode)
		if pair, ok = repocnt[u.RepoID]; !ok {
			pair = &sumdata{}
			repocnt[u.RepoID] = pair
		}
		pair.Count++
		pair.Type += int(u.Type)
		pair.Mode += int(u.Mode)
	}
	sha := fmt.Sprintf("%x total:%d usersum: %d, reposum: %d, typesum: %d, modesum: %d",
		h.Sum(nil), sum.Count, sum.User, sum.Repo, sum.Type, sum.Mode)
	log.Info("SHA from %s: %s", source, sha)
	return sha, usercnt, repocnt, nil
}

func dumpUserOrRepo(x *xorm.Engine, str string, userID, repoID int64) {
	if userID == 0 && repoID == 0 {
		return
	}
	data := make([]*UserRepoUnit, 0, 32)
	sess := x.Table("user_repo_unit").Where("1 = 1")
	if userID != 0 {
		sess.And("user_id = ?", userID)
	}
	if repoID != 0 {
		sess.And("repo_id = ?", repoID)
	}
	if err := sess.OrderBy("user_id, repo_id, type, mode").
		Find(&data); err != nil {
		log.Error("dumpUserOrRepo: %v", err)
		return
	}
	for _, u := range data {
		log.Info(" --- %s: User %3d  Repo %3d  %d, %d", str, u.UserID, u.RepoID, u.Type, u.Mode)
	}
}

func orderMapKeys(m map[int64]*sumdata) []int64 {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
