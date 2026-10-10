// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
)

// PushRefResult is the outcome of one ref in "git push --porcelain"
type PushRefResult struct {
	Ref      string // remote ref name
	Refspec  string // the refspec to push this ref again
	Summary  string
	Deleted  bool
	UpToDate bool
	Failed   bool
}

// PushRefsOptions options of PushRefsToExternal
type PushRefsOptions struct {
	Remote   string
	Mirror   bool     // push every ref with "--mirror" (and ignore Refspecs)
	Refspecs []string // used when Mirror is false
	Timeout  time.Duration
}

const (
	pushRefspecBatchSize = 500
	// a server side pre-receive hook rejects all refs of a push if one of them is declined,
	// so the failed refs are pushed one by one to find out which ones really fail
	maxPushRetryRefs = 200
)

// PushRefsToExternal force pushes to an external remote and reports the outcome of every ref.
// A rejected ref does not stop the other refs, it is only reported as failed in the results.
func PushRefsToExternal(ctx context.Context, repo RepositoryFacade, opts PushRefsOptions) ([]PushRefResult, error) {
	var results []PushRefResult
	if opts.Mirror {
		res, err := runPushPorcelain(ctx, repo, opts.Remote, nil, true, opts.Timeout)
		if err != nil {
			return res, err
		}
		results = res
	} else {
		for batch := range chunkStrings(opts.Refspecs, pushRefspecBatchSize) {
			res, err := runPushPorcelain(ctx, repo, opts.Remote, batch, false, opts.Timeout)
			results = append(results, res...)
			if err != nil {
				return results, err
			}
		}
	}
	return retryFailedRefs(ctx, repo, opts, results)
}

// retryFailedRefs pushes every failed ref on its own when more than one ref failed
func retryFailedRefs(ctx context.Context, repo RepositoryFacade, opts PushRefsOptions, results []PushRefResult) ([]PushRefResult, error) {
	var failed []PushRefResult
	final := make([]PushRefResult, 0, len(results))
	for _, r := range results {
		if r.Failed {
			failed = append(failed, r)
		} else {
			final = append(final, r)
		}
	}
	if len(failed) <= 1 {
		return results, nil
	}
	for i, f := range failed {
		if i >= maxPushRetryRefs || ctx.Err() != nil {
			final = append(final, failed[i:]...)
			break
		}
		res, err := runPushPorcelain(ctx, repo, opts.Remote, []string{f.Refspec}, false, opts.Timeout)
		if len(res) == 0 || err != nil {
			final = append(final, f)
			continue
		}
		final = append(final, res...)
	}
	return final, nil
}

func chunkStrings(s []string, size int) func(yield func([]string) bool) {
	return func(yield func([]string) bool) {
		for len(s) > 0 {
			n := min(size, len(s))
			if !yield(s[:n]) {
				return
			}
			s = s[n:]
		}
	}
}

func runPushPorcelain(ctx context.Context, repo RepositoryFacade, remote string, refspecs []string, mirror bool, timeout time.Duration) ([]PushRefResult, error) {
	cmd := gitcmd.NewCommand("push", "--porcelain", "-f")
	if mirror {
		cmd.AddArguments("--mirror")
	} else {
		// the push mirror remote is configured with "mirror = true" which forbids explicit refspecs
		cmd.AddConfig("remote."+remote+".mirror", "false")
	}
	cmd.AddDashesAndList(append([]string{remote}, refspecs...)...)
	stdout, stderr, err := cmd.WithDir(gitrepo.RepoLocalPath(repo)).WithTimeout(timeout).RunStdString(ctx)
	results := ParsePushPorcelain(stdout)
	if err == nil {
		return results, nil
	}
	for _, r := range results {
		if r.Failed {
			// per-ref failures are reported through the results
			return results, nil
		}
	}
	return results, fmt.Errorf("push failed: %w - %s", err, stderr)
}

// ParsePushPorcelain parses the stdout of "git push --porcelain", lines are "<flag>\t<from>:<to>\t<summary> (<reason>)"
func ParsePushPorcelain(stdout string) (results []PushRefResult) {
	for line := range strings.SplitSeq(stdout, "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 || len(parts[0]) != 1 || !strings.Contains(" +-*=!", parts[0]) {
			continue
		}
		from, to, _ := strings.Cut(parts[1], ":")
		refspec := ":" + to
		if from != "" {
			refspec = "+" + parts[1]
		}
		results = append(results, PushRefResult{
			Ref:      to,
			Refspec:  refspec,
			Summary:  strings.TrimSpace(parts[2]),
			Deleted:  parts[0] == "-",
			UpToDate: parts[0] == "=",
			Failed:   parts[0] == "!",
		})
	}
	return results
}

// ListRefNames lists the full ref names with the given prefixes of a local repository
func ListRefNames(ctx context.Context, repo RepositoryFacade, prefixes ...string) ([]string, error) {
	stdout, _, err := gitcmd.NewCommand("for-each-ref", "--format=%(refname)").
		AddDynamicArguments(prefixes...).WithRepo(repo).RunStdString(ctx)
	if err != nil {
		return nil, err
	}
	return strings.Fields(stdout), nil
}

// ListRemoteRefNames lists the branch and tag ref names of a remote configured in the repository
func ListRemoteRefNames(ctx context.Context, repo RepositoryFacade, remote string, timeout time.Duration) ([]string, error) {
	stdout, _, err := gitcmd.NewCommand("ls-remote", "--heads", "--tags").
		AddDashesAndList(remote).WithRepo(repo).WithTimeout(timeout).RunStdString(ctx)
	if err != nil {
		return nil, err
	}
	var refs []string
	for line := range strings.SplitSeq(stdout, "\n") {
		_, ref, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if ok && !strings.HasSuffix(ref, "^{}") {
			refs = append(refs, ref)
		}
	}
	return refs, nil
}
