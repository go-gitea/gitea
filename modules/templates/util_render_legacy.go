// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"
	"html/template"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/translation"
)

func renderEmojiLegacy(ctx context.Context, text string) template.HTML {
	panicIfDevOrTesting()
	return NewRenderUtils(reqctx.FromContext(ctx)).RenderEmoji(text)
}

func renderLabelLegacy(ctx context.Context, locale translation.Locale, label *issues_model.Label) template.HTML {
	panicIfDevOrTesting()
	return NewRenderUtils(reqctx.FromContext(ctx)).RenderLabel(label)
}

func renderLabelsLegacy(ctx context.Context, locale translation.Locale, labels []*issues_model.Label, repoLink string, issue *issues_model.Issue) template.HTML {
	panicIfDevOrTesting()
	return NewRenderUtils(reqctx.FromContext(ctx)).RenderLabels(labels, repoLink, issue)
}

func renderMarkdownToHtmlLegacy(ctx context.Context, input string) template.HTML { //nolint:revive
	panicIfDevOrTesting()
	return NewRenderUtils(reqctx.FromContext(ctx)).MarkdownToHtml(input)
}

func renderCommitMessageLegacy(ctx context.Context, msg string, metas map[string]string) template.HTML {
	panicIfDevOrTesting()
	return NewRenderUtils(reqctx.FromContext(ctx)).RenderCommitMessage(msg, metas)
}

func renderCommitMessageLinkSubjectLegacy(ctx context.Context, msg, urlDefault string, metas map[string]string) template.HTML {
	panicIfDevOrTesting()
	return NewRenderUtils(reqctx.FromContext(ctx)).RenderCommitMessageLinkSubject(msg, urlDefault, metas)
}

func renderIssueTitleLegacy(ctx context.Context, text string, metas map[string]string) template.HTML {
	panicIfDevOrTesting()
	return NewRenderUtils(reqctx.FromContext(ctx)).RenderIssueTitle(text, metas)
}

func renderCommitBodyLegacy(ctx context.Context, msg string, metas map[string]string) template.HTML {
	panicIfDevOrTesting()
	return NewRenderUtils(reqctx.FromContext(ctx)).RenderCommitBody(msg, metas)
}
