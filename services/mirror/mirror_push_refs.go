// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"fmt"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
)

const maxHistoryFailedRefs = 50

// buildPushRefspecs returns the refspecs to push (and delete) according to the config.
// Remote refs which are not selected by the config are never touched.
func buildPushRefspecs(cfg *repo_model.PushMirrorConfig, localRefs, remoteRefs []string) (refspecs []string) {
	local := make(map[string]struct{}, len(localRefs))
	for _, ref := range localRefs {
		local[ref] = struct{}{}
		if branch, ok := strings.CutPrefix(ref, git.BranchPrefix); ok {
			if cfg.MatchBranch(branch) {
				refspecs = append(refspecs, "+"+ref+":"+ref)
			}
		} else if strings.HasPrefix(ref, git.TagPrefix) && !cfg.NoPushTags {
			refspecs = append(refspecs, "+"+ref+":"+ref)
		}
	}
	for _, ref := range remoteRefs {
		if _, ok := local[ref]; ok {
			continue
		}
		if branch, ok := strings.CutPrefix(ref, git.BranchPrefix); ok {
			if !cfg.KeepRemoteBranches && cfg.MatchBranch(branch) {
				refspecs = append(refspecs, ":"+ref)
			}
		} else if strings.HasPrefix(ref, git.TagPrefix) && !cfg.NoPushTags && !cfg.KeepRemoteTags {
			refspecs = append(refspecs, ":"+ref)
		}
	}
	return refspecs
}

func needRemoteRefs(cfg *repo_model.PushMirrorConfig) bool {
	return !cfg.KeepRemoteBranches || (!cfg.NoPushTags && !cfg.KeepRemoteTags)
}

// addPushResults folds the git results into the result of the whole sync
func addPushResults(res *repo_model.PushMirrorResult, results []git.PushRefResult, refPrefix string) {
	for _, r := range results {
		switch {
		case r.Failed:
			res.FailedTotal++
			if len(res.Failed) < maxHistoryFailedRefs {
				res.Failed = append(res.Failed, repo_model.PushMirrorRefError{Ref: refPrefix + r.Ref, Reason: r.Summary})
			}
		case r.Deleted:
			res.Deleted++
		case r.UpToDate:
			res.UpToDate++
		default:
			res.Pushed++
		}
	}
}

func pushResultStatus(res *repo_model.PushMirrorResult, err error) repo_model.PushMirrorStatus {
	switch {
	case err == nil:
		return repo_model.PushMirrorStatusSuccess
	case res.FailedTotal > 0 && res.Error == "" && res.Pushed+res.Deleted+res.UpToDate > 0:
		return repo_model.PushMirrorStatusPartial
	default:
		return repo_model.PushMirrorStatusFailed
	}
}

// failedRefsError summarizes the failed refs of a sync
func failedRefsError(res *repo_model.PushMirrorResult) error {
	if res.FailedTotal == 0 {
		return nil
	}
	msgs := make([]string, 0, len(res.Failed))
	for _, f := range res.Failed {
		msgs = append(msgs, f.Ref+": "+f.Reason)
		if len(msgs) == 5 {
			break
		}
	}
	more := ""
	if res.FailedTotal > len(msgs) {
		more = fmt.Sprintf(" (and %d more)", res.FailedTotal-len(msgs))
	}
	return fmt.Errorf("%d refs failed to push: %s%s", res.FailedTotal, strings.Join(msgs, "; "), more)
}
