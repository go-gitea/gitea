package mirror

import (
	admin_model "code.gitea.io/gitea/models/admin"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// get git command running stdout and stderr
func getGitCommandStdoutStderr(ctx context.Context, m *repo_model.Mirror, gitArgs []string, newRepoPath string) (string, string, error) {
	stdoutBuilder := strings.Builder{}
	stderrBuilder := strings.Builder{}
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	remoteAddr, remoteErr := git.GetRemoteAddress(ctx, newRepoPath, m.GetRemoteName())
	if remoteErr != nil {
		log.Error("GetMirrorCanUpdate [repo: %-v]: GetRemoteAddress Error %v", m.Repo, remoteErr)
		return "", "", remoteErr
	}

	if err := git.NewCommand(ctx, gitArgs...).
		SetDescription(fmt.Sprintf("Mirror.getMirrorCanUpdate: %s", m.Repo.FullName())).
		RunWithContext(&git.RunContext{
			Timeout: timeout,
			Dir:     newRepoPath,
			Stdout:  &stdoutBuilder,
			Stderr:  &stderrBuilder,
		}); err != nil {
		stdout := stdoutBuilder.String()
		stderr := stderrBuilder.String()
		sanitizer := util.NewURLSanitizer(remoteAddr, true)
		stderrMessage := sanitizer.Replace(stderr)
		stdoutMessage := sanitizer.Replace(stdout)
		log.Error("CreateRepositoryNotice: %v", err)
		stderrMessage = sanitizer.Replace(stderr)
		stdoutMessage = sanitizer.Replace(stdout)
		desc := fmt.Sprintf("Failed to get mirror repository can update '%s': %s", newRepoPath, stderrMessage)
		log.Error("GetMirrorCanUpdate [repo: %-v]: failed to get mirror repository can update:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdoutMessage, stderrMessage, err)
		if err = admin_model.CreateRepositoryNotice(desc); err != nil {
			log.Error("GetMirrorCanUpdateNotice: %v", err)
		}
	}
	stdoutRepoCommitCount := stdoutBuilder.String()
	stderrRepoCommitCount := stdoutBuilder.String()
	stderrBuilder.Reset()
	stdoutBuilder.Reset()

	return stdoutRepoCommitCount, stderrRepoCommitCount, nil
}

// detect user can update the mirror
func detectCanUpdateMirror(ctx context.Context, m *repo_model.Mirror, gitArgs []string) (error, bool) {
	repoPath := m.Repo.RepoPath()
	newRepoPath := fmt.Sprintf("%s_update", repoPath)
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	//do copy directory recursive
	err := util.CopyDir(repoPath, newRepoPath)
	defer util.RemoveAll(newRepoPath)
	if err != nil {
		log.Error("GetMirrorCanUpdate [repo: %-v]: CopyDirectory Error %v", m.Repo, err)
		return err, false
	}
	remoteAddr, remoteErr := git.GetRemoteAddress(ctx, newRepoPath, m.GetRemoteName())
	if remoteErr != nil {
		log.Error("GetMirrorCanUpdate [repo: %-v]: GetRemoteAddress Error %v", m.Repo, remoteErr)
	}

	stdoutBuilder := strings.Builder{}
	stderrBuilder := strings.Builder{}
	if err := git.NewCommand(ctx, gitArgs...).
		SetDescription(fmt.Sprintf("Mirror.runSync: %s", m.Repo.FullName())).
		RunWithContext(&git.RunContext{
			Timeout: timeout,
			Dir:     newRepoPath,
			Stdout:  &stdoutBuilder,
			Stderr:  &stderrBuilder,
		}); err != nil {
		stdout := stdoutBuilder.String()
		stderr := stderrBuilder.String()

		// sanitize the output, since it may contain the remote address, which may
		// contain a password
		sanitizer := util.NewURLSanitizer(remoteAddr, true)
		stderrMessage := sanitizer.Replace(stderr)
		stdoutMessage := sanitizer.Replace(stdout)

		// Now check if the error is a resolve reference due to broken reference
		if strings.Contains(stderr, "unable to resolve reference") && strings.Contains(stderr, "reference broken") {
			log.Warn("SyncMirrors [repo: %-v]: failed to update mirror repository due to broken references:\nStdout: %s\nStderr: %s\nErr: %v\nAttempting Prune", m.Repo, stdoutMessage, stderrMessage, err)
			err = nil

			// Attempt prune
			pruneErr := pruneBrokenReferences(ctx, m, newRepoPath, timeout, &stdoutBuilder, &stderrBuilder, sanitizer, false)
			if pruneErr == nil {
				// Successful prune - reattempt mirror
				stderrBuilder.Reset()
				stdoutBuilder.Reset()
				if err = git.NewCommand(ctx, gitArgs...).
					SetDescription(fmt.Sprintf("Mirror.runSync: %s", m.Repo.FullName())).
					RunWithContext(&git.RunContext{
						Timeout: timeout,
						Dir:     newRepoPath,
						Stdout:  &stdoutBuilder,
						Stderr:  &stderrBuilder,
					}); err != nil {
					stdout := stdoutBuilder.String()
					stderr := stderrBuilder.String()

					// sanitize the output, since it may contain the remote address, which may
					// contain a password
					stderrMessage = sanitizer.Replace(stderr)
					stdoutMessage = sanitizer.Replace(stdout)
				}
			}
		}

		// If there is still an error (or there always was an error)
		if err != nil {
			log.Error("SyncMirrors [repo: %-v]: failed to update mirror repository:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdoutMessage, stderrMessage, err)
			desc := fmt.Sprintf("Failed to update mirror repository '%s': %s", newRepoPath, stderrMessage)
			if err = admin_model.CreateRepositoryNotice(desc); err != nil {
				log.Error("CreateRepositoryNotice: %v", err)
			}
			return nil, false
		}
	}
	/*
		gitea安全镜像仓库实现逻辑及思路

		思路逻辑 拉取最新的项目代码，对比当前镜像的代码count数目

		譬如当前镜像的count数目为1135
		最新的代码count数目为1140
		那么比对1135后的commit
		git log -1 --skip=5 --format="%H" 那么这是未更新前的代码的commit id 对比当前镜像仓库未更新前的最后一条commit id


		实现思路，未更新本地镜像前记录下当前镜像项目已经提交的commit数目，另外拷贝一个文件夹进行代码更新，更新后获取提交过的commit数目，
		如果本地镜像和更新后的commit 数目一致且代码最后一条commit id一致，那么可以进行安全进更新（重复进行更新，相当于只是为了记录运行更新日志）
		如果本地镜像和更新后的commit数目不一致且后续的commit数目大于当前的项目commit数目，且skip后的项目commit id和当前的最后commit id一致，那么进行代码更新
		如果本地镜像和更新后的commit数目小于已更新的代码数目，需进行人工干预更新，基本上已经判定作者提交了force push或者作者删库跑路或者已经商业化了

		git rev-list HEAD --count 获取已经提交的历史记录数
	*/
	gitCommitCountArgs := []string{"rev-list", "HEAD", "--count"}
	stdoutNewRepoCommitCount, _, err := getGitCommandStdoutStderr(ctx, m, gitCommitCountArgs, newRepoPath)
	if err != nil {
		return err, false
	}
	stdoutNewRepoCommitCount = strings.TrimSpace(stdoutNewRepoCommitCount)
	stdoutRepoCommitCount, _, err := getGitCommandStdoutStderr(ctx, m, gitCommitCountArgs, repoPath)
	if err != nil {
		return err, false
	}
	stdoutRepoCommitCount = strings.TrimSpace(stdoutRepoCommitCount)
	var repoCommitCount, newRepoCommitCount int64
	if i, err := strconv.ParseInt(stdoutRepoCommitCount, 10, 64); err == nil {
		repoCommitCount = i
	} else {
		return err, false
	}
	if i, err := strconv.ParseInt(stdoutNewRepoCommitCount, 10, 64); err == nil {
		newRepoCommitCount = i
	} else {
		return err, false
	}
	if repoCommitCount > newRepoCommitCount {
		// 大概率用户已经删库跑路了
		return nil, false
	} else if repoCommitCount == newRepoCommitCount {
		// noting to happen
		return nil, true
	} else {
		//compare commit id
		gitNewRepoLastCommitIdArgs := []string{"log", "-1", fmt.Sprintf("--skip=%d", newRepoCommitCount-newRepoCommitCount), "--format=\"%H\""}
		stdoutNewRepoCommitId, _, err := getGitCommandStdoutStderr(ctx, m, gitNewRepoLastCommitIdArgs, newRepoPath)
		if err != nil {
			return err, false
		}
		gitRepoLastCommitIdArgs := []string{"log", "--format=\"%H\"", "-n", "1"}
		stdoutRepoCommitId, _, err := getGitCommandStdoutStderr(ctx, m, gitRepoLastCommitIdArgs, repoPath)
		if err != nil {
			return err, false
		}
		if stdoutNewRepoCommitId != stdoutRepoCommitId {
			return errors.New(fmt.Sprintf("Old repo commit id: %s not match new repo id: %s", stdoutRepoCommitId, stdoutNewRepoCommitId)), false
		} else {
			return nil, true
		}
	}
	return errors.New("Unknow error!"), false
}
