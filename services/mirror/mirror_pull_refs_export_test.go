// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import "time"

// NextBackupRefNameForTest selects a backup name from existing ref names.
func NextBackupRefNameForTest(prefix string, names []string, now time.Time) string {
	refs := make([]mirrorRef, 0, len(names))
	for _, name := range names {
		refs = append(refs, mirrorRef{name: name})
	}
	return nextBackupRefName(prefix, refs, now)
}

// SortBackupRefNamesForTest sorts backup refs from newest to oldest.
func SortBackupRefNamesForTest(prefix string, names []string) []string {
	refs := make([]mirrorRef, 0, len(names))
	for _, name := range names {
		refs = append(refs, mirrorRef{name: name})
	}
	sortBackupRefsNewestFirst(refs, prefix)
	result := make([]string, 0, len(refs))
	for _, ref := range refs {
		result = append(result, ref.name)
	}
	return result
}

// ParseBackupRefForTest returns the original ref name and backup sequence.
func ParseBackupRefForTest(name string) (string, string, int, bool) {
	refType, shortName, _, sequence, ok := parseBackupRef(name)
	return refType, shortName, sequence, ok
}

// FindBlockingMirrorRefsForTest finds local refs blocking incoming refs.
func FindBlockingMirrorRefsForTest(localNames, remoteNames []string, prune bool) ([]string, error) {
	local := make(map[string]mirrorRef, len(localNames))
	remote := make(map[string]mirrorRef, len(remoteNames))
	for _, name := range localNames {
		local[name] = mirrorRef{name: name}
	}
	for _, name := range remoteNames {
		remote[name] = mirrorRef{name: name}
	}
	return findBlockingMirrorRefs(local, remote, prune)
}

// ValidateRemoteRefNameForTest checks reserved mirror namespaces.
func ValidateRemoteRefNameForTest(name string) error {
	return validateRemoteRefName(name)
}
