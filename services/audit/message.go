// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"fmt"
	"strings"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/modules/log"
)

// Reserved placeholders, filled from the event itself rather than from metadata.
const (
	placeholderScope = "scope"
	placeholderActor = "actor"
)

// renderMessage fills the action's template from the event's scope, actor and
// metadata. A missing template or an unresolved placeholder is logged and
// rendered as the bare key: audit recording must never fail the request that
// triggered it, and a partial message is more useful than none.
func renderMessage(action audit_model.Action, actor, scope audit_model.EntityRef, metadata map[string]any) string {
	tmpl, ok := audit_model.MessageTemplate(action)
	if !ok {
		log.Error("audit: no message template for action %q", action)
		return string(action)
	}

	var sb strings.Builder
	rest := tmpl
	for {
		start := strings.IndexByte(rest, '{')
		if start < 0 {
			break
		}
		end := strings.IndexByte(rest[start:], '}')
		if end < 0 {
			break
		}
		end += start

		key := rest[start+1 : end]
		sb.WriteString(rest[:start])
		sb.WriteString(resolvePlaceholder(action, key, actor, scope, metadata))
		rest = rest[end+1:]
	}
	sb.WriteString(rest)

	return sb.String()
}

func resolvePlaceholder(action audit_model.Action, key string, actor, scope audit_model.EntityRef, metadata map[string]any) string {
	switch key {
	case placeholderScope:
		return scope.DisplayName()
	case placeholderActor:
		return actor.DisplayName()
	}
	if v, ok := metadata[key]; ok {
		return fmt.Sprint(v)
	}
	log.Error("audit: action %q has no metadata for placeholder %q", action, key)
	return key
}
