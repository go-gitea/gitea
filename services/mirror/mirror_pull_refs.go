// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	activities_model "gitea.dev/models/activities"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/log"
	repo_module "gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	feed_service "gitea.dev/services/feed"

	"github.com/google/uuid"
)

const (
	temporaryFetchPrefix        = "refs/mirror-fetch/"
	mirrorBackupBranchPrefix    = git.BranchPrefix + "mirror-backup/branch/"
	mirrorBackupTagPrefix       = git.TagPrefix + "mirror-backup/tag/"
	mirrorBackupTimestampLayout = "20060102-150405"
)

type mirrorRef struct {
	name     string
	objectID string
}

type mirrorRefMutation struct {
	name  string
	oldID string
	newID string
}

type mirrorBackup struct {
	name         string
	originalName string
	objectID     string
	refType      string
	shortName    string
}

type mirrorRefPlan struct {
	backups  []mirrorBackup
	blockers []mirrorRefMutation
	changes  []mirrorRefMutation
}

func needsStagedMirrorSync(ctx context.Context, m *repo_model.Mirror) (bool, error) {
	if m.ForcePushBackup {
		return true, nil
	}
	for _, prefix := range []string{mirrorBackupBranchPrefix, mirrorBackupTagPrefix} {
		refs, err := listRefsWithPrefix(ctx, m.Repo, prefix)
		if err != nil {
			return false, err
		}
		if len(refs) > 0 {
			return true, nil
		}
	}
	return false, nil
}

func listRefsWithPrefix(ctx context.Context, repo gitrepo.RepositoryFacade, prefix string) ([]mirrorRef, error) {
	stdout, _, err := gitcmd.NewCommand("for-each-ref", "--format=%(objectname) %(refname)").
		AddDynamicArguments(prefix).WithRepo(repo).RunStdString(ctx)
	if err != nil {
		return nil, err
	}
	var refs []mirrorRef
	for line := range strings.SplitSeq(stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !strings.HasPrefix(fields[1], prefix) {
			continue
		}
		refs = append(refs, mirrorRef{name: fields[1], objectID: fields[0]})
	}
	return refs, nil
}

func removeRefsWithPrefix(ctx context.Context, repo gitrepo.RepositoryFacade, prefix string) error {
	refs, err := listRefsWithPrefix(ctx, repo, prefix)
	if err != nil {
		return err
	}
	slices.SortFunc(refs, func(a, b mirrorRef) int { return strings.Compare(b.name, a.name) })
	mutations := make([]mirrorRefMutation, 0, len(refs))
	for _, ref := range refs {
		mutations = append(mutations, mirrorRefMutation{name: ref.name, oldID: ref.objectID})
	}
	return runMirrorRefBatch(ctx, repo, mutations)
}

func syncStagedMirrorRefs(ctx context.Context, m *repo_model.Mirror, timeout time.Duration) (err error) {
	remoteName := m.GetRemoteName()
	stagePrefix := temporaryFetchPrefix + uuid.NewString() + "/"
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if cleanupErr := removeRefsWithPrefix(cleanupCtx, m.Repo, stagePrefix); cleanupErr != nil {
			if err == nil {
				err = fmt.Errorf("clean temporary mirror refs: %w", cleanupErr)
				reconcilePartialMirrorState(cleanupCtx, m)
			} else {
				log.Error("SyncMirrors [repo: %-v]: failed to clean temporary refs: %v", m.Repo, cleanupErr)
			}
		}
	}()

	for attempt := range 2 {
		cmd := gitcmd.NewCommand("fetch", "--no-tags", "--no-prune", "--refmap=").
			AddDynamicArguments(remoteName, "+refs/*:"+stagePrefix+"*").
			WithTimeout(timeout).WithRepo(m.Repo)
		_, stderr, fetchErr := cmd.RunStdString(ctx)
		if fetchErr == nil {
			break
		}
		if attempt == 1 || !checkRecoverableSyncError(stderr) {
			return fmt.Errorf("fetch mirror refs: %s", util.SanitizeCredentialURLs(fetchErr.Error()))
		}
		if err := removeRefsWithPrefix(ctx, m.Repo, stagePrefix); err != nil {
			return fmt.Errorf("clear failed temporary fetch: %w", err)
		}
		stagePrefix = temporaryFetchPrefix + uuid.NewString() + "/"
	}

	stagedRefs, err := listRefsWithPrefix(ctx, m.Repo, stagePrefix)
	if err != nil {
		return err
	}
	remoteRefs := make(map[string]mirrorRef, len(stagedRefs))
	for _, ref := range stagedRefs {
		name := "refs/" + strings.TrimPrefix(ref.name, stagePrefix)
		remoteRefs[name] = mirrorRef{name: name, objectID: ref.objectID}
	}
	if err := validateRemoteRefNamespaces(remoteRefs); err != nil {
		return err
	}
	localRefs, err := listRefsWithPrefix(ctx, m.Repo, "refs/")
	if err != nil {
		return err
	}
	plan, err := planMirrorRefChanges(ctx, m, localRefs, remoteRefs)
	if err != nil {
		return err
	}
	return applyMirrorRefPlan(ctx, m, plan)
}

func validateRemoteRefNamespaces(refs map[string]mirrorRef) error {
	for name := range refs {
		if err := validateRemoteRefName(name); err != nil {
			return err
		}
	}
	return nil
}

func validateRemoteRefName(name string) error {
	for _, prefix := range []string{temporaryFetchPrefix, mirrorBackupBranchPrefix, mirrorBackupTagPrefix} {
		root := strings.TrimSuffix(prefix, "/")
		if name == root || strings.HasPrefix(name, prefix) || strings.HasPrefix(root, name+"/") {
			return fmt.Errorf("remote ref %q conflicts with reserved mirror refs", name)
		}
	}
	return nil
}

func findBlockingMirrorRefs(local, remote map[string]mirrorRef, enablePrune bool) ([]string, error) {
	localNames := make([]string, 0, len(local))
	for name := range local {
		localNames = append(localNames, name)
	}
	slices.Sort(localNames)
	blocking := make(map[string]struct{})
	for remoteName := range remote {
		if _, exists := local[remoteName]; exists {
			continue
		}
		for i := 0; i < len(remoteName); i++ {
			if remoteName[i] != '/' {
				continue
			}
			ancestor := remoteName[:i]
			if _, exists := local[ancestor]; !exists {
				continue
			}
			if !enablePrune {
				return nil, fmt.Errorf("remote ref %q conflicts with retained local ref %q", remoteName, ancestor)
			}
			blocking[ancestor] = struct{}{}
		}
		prefix := remoteName + "/"
		for i := sort.SearchStrings(localNames, prefix); i < len(localNames); i++ {
			localName := localNames[i]
			if !strings.HasPrefix(localName, prefix) {
				break
			}
			if !enablePrune {
				return nil, fmt.Errorf("remote ref %q conflicts with retained local ref %q", remoteName, localName)
			}
			blocking[localName] = struct{}{}
		}
	}
	result := make([]string, 0, len(blocking))
	for name := range blocking {
		result = append(result, name)
	}
	slices.Sort(result)
	return result, nil
}

func planMirrorRefChanges(ctx context.Context, m *repo_model.Mirror, localRefs []mirrorRef, remote map[string]mirrorRef) (*mirrorRefPlan, error) {
	local := make(map[string]mirrorRef, len(localRefs))
	backupsByOriginal := make(map[string][]mirrorRef)
	for _, ref := range localRefs {
		if strings.HasPrefix(ref.name, temporaryFetchPrefix) {
			continue
		}
		if strings.HasPrefix(ref.name, mirrorBackupBranchPrefix) || strings.HasPrefix(ref.name, mirrorBackupTagPrefix) {
			if kind, shortName, _, _, ok := parseBackupRef(ref.name); ok {
				backupsByOriginal[kind+":"+shortName] = append(backupsByOriginal[kind+":"+shortName], ref)
			}
			continue
		}
		local[ref.name] = ref
	}
	blocking, err := findBlockingMirrorRefs(local, remote, m.EnablePrune)
	if err != nil {
		return nil, err
	}
	blockingSet := make(map[string]struct{}, len(blocking))
	for _, name := range blocking {
		blockingSet[name] = struct{}{}
	}
	backupBlockers := make(map[string]string)
	for _, refType := range []string{"branch", "tag"} {
		root := strings.TrimSuffix(backupRefPrefix(refType), "/")
		for name := range local {
			if name == root || strings.HasPrefix(root, name+"/") {
				backupBlockers[refType] = name
				break
			}
		}
	}
	plan := &mirrorRefPlan{}
	now := time.Now().UTC()
	for name, oldRef := range local {
		newRef, exists := remote[name]
		if exists && oldRef.objectID == newRef.objectID {
			continue
		}
		if !exists && !m.EnablePrune {
			continue
		}
		if m.ForcePushBackup && (strings.HasPrefix(name, git.BranchPrefix) || strings.HasPrefix(name, git.TagPrefix)) {
			backup := !exists || strings.HasPrefix(name, git.TagPrefix)
			if exists && !backup {
				canFastForward, err := canFastForwardRef(ctx, m.Repo, oldRef.objectID, newRef.objectID)
				if err != nil {
					return nil, err
				}
				backup = !canFastForward
			}
			if backup {
				refType, shortName := mirrorRefTypeAndShortName(name)
				if blocker, exists := backupBlockers[refType]; exists {
					return nil, fmt.Errorf("local ref %q blocks backup namespace", blocker)
				}
				prefix := backupRefPrefix(refType) + shortName + "-"
				plan.backups = append(plan.backups, mirrorBackup{
					name:         nextBackupRefName(prefix, backupsByOriginal[refType+":"+shortName], now),
					originalName: name, objectID: oldRef.objectID, refType: refType, shortName: shortName,
				})
			}
		}
		mutation := mirrorRefMutation{name: name, oldID: oldRef.objectID}
		if exists {
			mutation.newID = newRef.objectID
		}
		if _, isBlocker := blockingSet[name]; isBlocker {
			plan.blockers = append(plan.blockers, mutation)
		} else {
			plan.changes = append(plan.changes, mutation)
		}
	}
	for name, newRef := range remote {
		if _, exists := local[name]; !exists {
			plan.changes = append(plan.changes, mirrorRefMutation{name: name, newID: newRef.objectID})
		}
	}
	slices.SortFunc(plan.backups, func(a, b mirrorBackup) int { return strings.Compare(a.name, b.name) })
	slices.SortFunc(plan.blockers, func(a, b mirrorRefMutation) int { return strings.Compare(a.name, b.name) })
	slices.SortFunc(plan.changes, func(a, b mirrorRefMutation) int { return strings.Compare(a.name, b.name) })
	return plan, nil
}

func mirrorRefTypeAndShortName(name string) (string, string) {
	if shortName, ok := strings.CutPrefix(name, git.TagPrefix); ok {
		return "tag", shortName
	}
	return "branch", strings.TrimPrefix(name, git.BranchPrefix)
}

func canFastForwardRef(ctx context.Context, repo gitrepo.RepositoryFacade, oldObjectID, newObjectID string) (bool, error) {
	_, _, err := gitcmd.NewCommand("merge-base", "--is-ancestor").
		AddDynamicArguments(oldObjectID, newObjectID).WithRepo(repo).RunStdString(ctx)
	if err == nil {
		return true, nil
	}
	if gitcmd.IsErrorExitCode(err, 1) {
		return false, nil
	}
	return false, err
}

func applyMirrorRefPlan(ctx context.Context, m *repo_model.Mirror, plan *mirrorRefPlan) error {
	phaseOne := make([]mirrorRefMutation, 0, len(plan.backups)+len(plan.blockers))
	blocking := make(map[string]struct{}, len(plan.blockers))
	for _, mutation := range plan.blockers {
		blocking[mutation.name] = struct{}{}
	}
	for _, backup := range plan.backups {
		phaseOne = append(phaseOne, mirrorRefMutation{name: backup.name, newID: backup.objectID})
		if _, removed := blocking[backup.originalName]; !removed {
			phaseOne = append(phaseOne, mirrorRefMutation{name: backup.originalName, oldID: backup.objectID, newID: backup.objectID})
		}
	}
	for _, mutation := range plan.blockers {
		phaseOne = append(phaseOne, mirrorRefMutation{name: mutation.name, oldID: mutation.oldID})
	}
	if err := runMirrorRefBatch(ctx, m.Repo, phaseOne); err != nil {
		reconcilePartialMirrorState(ctx, m)
		return fmt.Errorf("create mirror backups and remove conflicting refs: %w", err)
	}
	for _, backup := range plan.backups {
		notifyMirrorBackup(ctx, m, backup)
	}
	phaseTwo := make([]mirrorRefMutation, 0, len(plan.changes)+len(plan.backups))
	phaseTwo = append(phaseTwo, plan.changes...)
	for _, backup := range plan.backups {
		phaseTwo = append(phaseTwo, mirrorRefMutation{name: backup.name, oldID: backup.objectID, newID: backup.objectID})
	}
	if err := runMirrorRefBatch(ctx, m.Repo, phaseTwo); err != nil {
		reconcilePartialMirrorState(ctx, m)
		return fmt.Errorf("apply mirror refs: %w", err)
	}
	for _, backup := range plan.backups {
		if err := pruneBackupRefs(ctx, m, backup.refType, backup.shortName, backup.name); err != nil {
			reconcilePartialMirrorState(ctx, m)
			return fmt.Errorf("prune mirror backups: %w", err)
		}
	}
	return nil
}

func runMirrorRefBatch(ctx context.Context, repo gitrepo.RepositoryFacade, mutations []mirrorRefMutation) error {
	if len(mutations) == 0 {
		return nil
	}
	var input bytes.Buffer
	for _, mutation := range mutations {
		input.WriteString("option no-deref\x00")
		switch {
		case mutation.oldID == "":
			input.WriteString("create " + mutation.name + "\x00" + mutation.newID + "\x00")
		case mutation.newID == "":
			input.WriteString("delete " + mutation.name + "\x00" + mutation.oldID + "\x00")
		case mutation.oldID == mutation.newID:
			input.WriteString("verify " + mutation.name + "\x00" + mutation.oldID + "\x00")
		default:
			input.WriteString("update " + mutation.name + "\x00" + mutation.newID + "\x00" + mutation.oldID + "\x00")
		}
	}
	return gitcmd.NewCommand("update-ref", "--stdin", "-z").WithStdinBytes(input.Bytes()).WithRepo(repo).Run(ctx)
}

func notifyMirrorBackup(ctx context.Context, m *repo_model.Mirror, backup mirrorBackup) {
	notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := feed_service.NotifyWatchers(notifyCtx, &activities_model.Action{
		ActUserID: m.Repo.OwnerID,
		ActUser:   m.Repo.MustOwner(notifyCtx),
		OpType:    activities_model.ActionMirrorSyncBackup,
		RepoID:    m.Repo.ID,
		Repo:      m.Repo,
		IsPrivate: m.Repo.IsPrivate,
		RefName:   backup.name,
		Content:   backup.originalName,
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

func reconcilePartialMirrorState(ctx context.Context, m *repo_model.Mirror) {
	reconcileCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	gitRepo, err := git.OpenRepository(reconcileCtx, m.Repo)
	if err != nil {
		log.Error("SyncMirrors [repo: %-v]: failed to open partially synced repo: %v", m.Repo, err)
		return
	}
	defer gitRepo.Close()
	if _, _, err := repo_module.SyncRepoBranchesWithRepo(reconcileCtx, m.Repo, gitRepo, 0); err != nil {
		log.Error("SyncMirrors [repo: %-v]: failed to reconcile branches: %v", m.Repo, err)
	}
	if _, err := repo_module.SyncReleasesWithTags(reconcileCtx, m.Repo, gitRepo); err != nil {
		log.Error("SyncMirrors [repo: %-v]: failed to reconcile tags: %v", m.Repo, err)
	}
}

func backupRefPrefix(refType string) string {
	if refType == "tag" {
		return mirrorBackupTagPrefix
	}
	return mirrorBackupBranchPrefix
}

func parseBackupRef(name string) (refType, shortName string, timestamp time.Time, sequence int, ok bool) {
	for _, refType = range []string{"branch", "tag"} {
		prefix := backupRefPrefix(refType)
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		body := strings.TrimPrefix(name, prefix)
		if shortName, timestamp, ok = parseBackupBody(body); ok {
			return refType, shortName, timestamp, 1, true
		}
		if base, suffix, found := strings.CutLast(body, "-"); found {
			if n, err := strconv.Atoi(suffix); err == nil && n >= 2 {
				if shortName, timestamp, ok = parseBackupBody(base); ok {
					return refType, shortName, timestamp, n, true
				}
			}
		}
		return "", "", time.Time{}, 0, false
	}
	return "", "", time.Time{}, 0, false
}

func parseBackupBody(body string) (string, time.Time, bool) {
	if len(body) < len(mirrorBackupTimestampLayout)+2 {
		return "", time.Time{}, false
	}
	dash := len(body) - len(mirrorBackupTimestampLayout) - 1
	if dash == 0 || body[dash] != '-' {
		return "", time.Time{}, false
	}
	timestamp, err := time.Parse(mirrorBackupTimestampLayout, body[dash+1:])
	if err != nil {
		return "", time.Time{}, false
	}
	return body[:dash], timestamp, true
}

func isBackupRefForPrefix(refName, prefix string) bool {
	refType, shortName, _, _, ok := parseBackupRef(refName)
	return ok && prefix == backupRefPrefix(refType)+shortName+"-"
}

func nextBackupRefName(prefix string, refs []mirrorRef, now time.Time) string {
	timestamp := now.UTC().Truncate(time.Second)
	maxSequence := 0
	for _, ref := range refs {
		_, _, existingTime, sequence, ok := parseBackupRef(ref.name)
		if !ok || !strings.HasPrefix(ref.name, prefix) {
			continue
		}
		if existingTime.After(timestamp) {
			timestamp = existingTime
			maxSequence = sequence
		} else if existingTime.Equal(timestamp) && sequence > maxSequence {
			maxSequence = sequence
		}
	}
	name := prefix + timestamp.Format(mirrorBackupTimestampLayout)
	if maxSequence > 0 {
		name += "-" + strconv.Itoa(maxSequence+1)
	}
	return name
}

func sortBackupRefsNewestFirst(refs []mirrorRef, prefix string) {
	slices.SortFunc(refs, func(a, b mirrorRef) int {
		_, _, aTime, aSequence, aOK := parseBackupRef(a.name)
		_, _, bTime, bSequence, bOK := parseBackupRef(b.name)
		if aOK && bOK && strings.HasPrefix(a.name, prefix) && strings.HasPrefix(b.name, prefix) {
			if !aTime.Equal(bTime) {
				if aTime.After(bTime) {
					return -1
				}
				return 1
			}
			if aSequence != bSequence {
				return bSequence - aSequence
			}
		}
		return strings.Compare(b.name, a.name)
	})
}

func pruneBackupRefs(ctx context.Context, m *repo_model.Mirror, refType, shortName, newestName string) error {
	prefix := backupRefPrefix(refType) + shortName + "-"
	refs, err := listRefsWithPrefix(ctx, m.Repo, backupRefPrefix(refType))
	if err != nil {
		return err
	}
	var matching []mirrorRef
	for _, ref := range refs {
		if isBackupRefForPrefix(ref.name, prefix) {
			matching = append(matching, ref)
		}
	}
	limit := setting.Mirror.DefaultForcePushBackupLimit
	if len(matching) <= limit {
		return nil
	}
	sortBackupRefsNewestFirst(matching, prefix)
	kept := map[string]struct{}{newestName: {}}
	var deletes []mirrorRefMutation
	for _, ref := range matching {
		if ref.name == newestName {
			continue
		}
		if len(kept) < limit {
			kept[ref.name] = struct{}{}
			continue
		}
		deletes = append(deletes, mirrorRefMutation{name: ref.name, oldID: ref.objectID})
	}
	return runMirrorRefBatch(ctx, m.Repo, deletes)
}
