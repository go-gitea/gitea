import {trString} from '../i18n.ts';
import {createElementFromAttrs, createElementFromHTML} from '../../utils/dom.ts';
import {html, htmlEscape, htmlRaw} from '../../utils/html.ts';
import {formatBytes} from '../../utils.ts';
import {formatDatetimeISO} from '../../utils/time.ts';
import {svg, svgRaw} from '../../svg.ts';
import {showContentHistoryMenuIfEdited} from '../../features/repo-issue-content.ts';
import type {CommentsData, DiffViewerLocale, ViewerComment} from './types.ts';

type CommentContext = {
  data: CommentsData;
  locale: DiffViewerLocale;
  onToggleReaction: (el: HTMLElement, comment: ViewerComment, content: string) => void;
  onQuoteReply: (comment: ViewerComment, menu: HTMLElement) => void;
  onDelete: (el: HTMLElement, comment: ViewerComment) => void;
};

// the element of a code comment, mirroring templates/repo/issue/view_content/comments.tmpl
export function buildCommentElement(comment: ViewerComment, ctx: CommentContext): HTMLElement {
  const {data, locale} = ctx;
  const anchor = `issuecomment-${comment.id}`;
  const timeHtml = html`<relative-time datetime="${formatDatetimeISO(comment.created)}"></relative-time>`;
  const commentedAt = trString(locale.commentedAt, htmlEscape(anchor), timeHtml);
  const posterHref = comment.poster.homeLink ? html` href="${comment.poster.homeLink}"` : '';
  let avatarHtml: string;
  let headerLeftHtml: string;
  if (comment.originalAuthor) {
    avatarHtml = html`<span class="avatar"><img class="ui avatar tw-align-middle" src="${comment.poster.avatar}" alt="" width="20" height="20"></span>`;
    const migratedFrom = data.originalUrl ? html`<span class="text migrate">(${htmlRaw(trString(locale.migratedFrom, htmlEscape(data.originalUrl), htmlEscape(data.originalHostname!)))})</span>` : '';
    headerLeftHtml = html`<span class="tw-text-text tw-font-semibold tw-mr-1">${htmlRaw(data.migrationIcon ? svg(data.migrationIcon) : '')} ${comment.originalAuthor}</span><span class="tw-text-text-light muted-links">${htmlRaw(commentedAt)}</span>${htmlRaw(migratedFrom)}`;
  } else {
    avatarHtml = html`<a class="avatar-with-link"${htmlRaw(posterHref)}><img class="ui avatar tw-align-middle" src="${comment.poster.avatar}" alt="" loading="lazy" width="20" height="20"></a>`;
    headerLeftHtml = html`<span class="tw-text-text-light muted-links"><a${htmlRaw(posterHref)}>${comment.poster.displayName}</a> ${htmlRaw(commentedAt)}</span>`;
  }
  let labelsHtml = '';
  if (comment.outdated) labelsHtml += html`<a href="${data.issueHtmlUrl}#${anchor}" class="ui label basic small" data-tooltip-content="${locale.outdatedDescription}">${locale.outdated}</a>`;
  if (comment.pending) labelsHtml += html`<div class="ui label basic small yellow pending-label" data-tooltip-content="${locale.pendingTooltip}">${locale.pending}</div>`;
  else if (comment.reviewId) labelsHtml += html`<div class="ui label basic small">${locale.review}</div>`;
  const el = createElementFromHTML<HTMLElement>(html`
    <div class="comment" id="${anchor}" data-comment-id="${comment.id}">
      <div class="tw-mt-2 tw-mr-4">${htmlRaw(avatarHtml)}</div>
      <div class="content comment-container">
        <div class="comment-header avatar-content-left-arrow">
          <div class="comment-header-left">${htmlRaw(headerLeftHtml)}</div>
          <div class="comment-header-right">${htmlRaw(labelsHtml)}</div>
        </div>
        <div class="ui attached segment comment-body">
          <div class="render-content markup"></div>
          <div id="${anchor}-raw" class="raw-content tw-hidden"></div>
          <div class="edit-content-zone tw-hidden" data-update-url="${data.repoLink}/comments/${comment.id}" data-content-version="${comment.contentVersion}"
            data-context="${data.repoLink}" data-attachment-url="${data.repoLink}/comments/${comment.id}/attachments"></div>
        </div>
      </div>
    </div>
  `);
  const headerRight = el.querySelector('.comment-header-right')!;
  if (data.canComment && data.reactions.length) headerRight.append(buildReactionPicker(comment, ctx));
  headerRight.append(buildContextMenu(comment, ctx));
  const markup = el.querySelector<HTMLElement>('.render-content')!;
  if (comment.canEdit) markup.setAttribute('data-can-edit', 'true');
  if (comment.bodyHtml) markup.innerHTML = comment.bodyHtml; // rendered and sanitized by the server
  else markup.append(createElementFromAttrs('span', {class: 'no-content'}, locale.noContent));
  el.querySelector('.raw-content')!.textContent = comment.body;
  if (comment.attachments.length) el.querySelector('.comment-body')!.append(buildAttachments(comment, locale));
  if (comment.reactions.length) el.querySelector('.comment-container')!.append(buildReactions(comment, ctx));
  showContentHistoryMenuIfEdited(el, comment.id);
  return el;
}

// mirrors templates/repo/issue/view_content/attachments.tmpl
function buildAttachments(comment: ViewerComment, locale: DiffViewerLocale): HTMLElement {
  const container = createElementFromAttrs('div', {class: 'dropzone-attachments'}, createElementFromAttrs('div', {class: 'divider'}));
  for (const att of comment.attachments) {
    container.append(createElementFromHTML(html`
      <div class="tw-flex">
        <div class="tw-flex-1 tw-p-2">
          <a target="_blank" href="${att.downloadUrl}" title="${trString(locale.openAttachment, att.name)}">
            ${svgRaw(att.isImage ? 'octicon-file' : 'octicon-desktop-download')}
            <span><strong>${att.name}</strong></span>
          </a>
        </div>
        <div class="flex-text-block tw-p-2"><span class="ui tw-text-text-light">${formatBytes(att.size)}</span></div>
      </div>
    `));
  }
  const thumbnails = comment.attachments.filter((att) => att.isImage && !att.inContent);
  if (thumbnails.length) {
    container.append(createElementFromAttrs('div', {class: 'divider'}), createElementFromHTML(html`<div class="ui small thumbnails">${htmlRaw(thumbnails.map((att) => html`
      <a target="_blank" href="${att.downloadUrl}"><img loading="lazy" alt="${att.name}" src="${att.downloadUrl}" title="${trString(locale.openAttachment, att.name)}"></a>
    `).join(''))}</div>`));
  }
  return container;
}

// mirrors templates/repo/issue/view_content/add_reaction.tmpl
function buildReactionPicker(comment: ViewerComment, ctx: CommentContext): HTMLElement {
  const picker = createElementFromHTML<HTMLElement>(html`
    <div class="item action ui dropdown jump pointing top right select-reaction" aria-label="${ctx.locale.reactions}">
      <a class="muted">${svgRaw('octicon-smiley')}</a>
      <div class="menu">${htmlRaw(ctx.data.reactions.map((reaction) => html`
        <a class="item emoji" data-tooltip-content="${reaction.content}" aria-label="${reaction.content}" data-reaction-content="${reaction.content}">${htmlRaw(reaction.emojiHtml)}</a>
      `).join(''))}
      </div>
    </div>
  `);
  picker.addEventListener('click', (e) => {
    const item = (e.target as Element).closest<HTMLElement>('[data-reaction-content]');
    if (item) ctx.onToggleReaction(item, comment, item.getAttribute('data-reaction-content')!);
  });
  return picker;
}

// mirrors templates/repo/issue/view_content/reactions.tmpl
function buildReactions(comment: ViewerComment, ctx: CommentContext): HTMLElement {
  const {data, locale} = ctx;
  const container = createElementFromAttrs('div', {class: 'bottom-reactions', role: 'group', 'aria-label': locale.reactions});
  for (const reaction of comment.reactions) {
    const title = reaction.users + (reaction.moreCount > 0 ? ` ${trString(locale.reactionsMore, reaction.moreCount)}` : '');
    const button = createElementFromHTML<HTMLElement>(html`
      <a role="button" class="ui label basic${reaction.hasReacted ? ' primary' : ''}${data.isSigned ? '' : ' disabled'}"
        data-tooltip-content title="${title}" aria-label="${reaction.content}: ${title}" data-tooltip-placement="bottom-start">
        <span class="reaction">${htmlRaw(reaction.emojiHtml)}</span>
        <span class="reaction-count">${reaction.count}</span>
      </a>
    `);
    if (data.isSigned) button.addEventListener('click', () => ctx.onToggleReaction(button, comment, reaction.content));
    container.append(button);
  }
  if (data.canComment && data.reactions.length) container.append(buildReactionPicker(comment, ctx));
  return container;
}

// mirrors templates/repo/issue/view_content/context_menu.tmpl
function buildContextMenu(comment: ViewerComment, ctx: CommentContext): HTMLElement {
  const {data, locale} = ctx;
  const anchor = `issuecomment-${comment.id}`;
  const referenceUrl = `${data.issueHtmlUrl}/files#${anchor}`;
  let items = html`
    <div class="item js-aria-clickable" data-clipboard-text="${referenceUrl}">${locale.copyLink}</div>
    <div class="item js-aria-clickable" data-clipboard-target="#${anchor}-raw">${locale.copySource}</div>
  `;
  if (data.canComment) {
    items += html`<div class="item js-aria-clickable" data-diff-viewer-action="quote" data-shortcut-keys="r" data-shortcut-handler="quoteSelectedReply">${locale.quoteReply}</div>`;
    if (data.canReferenceIssue) {
      items += html`<div class="item js-aria-clickable reference-issue" data-target="${anchor}-raw" data-modal="#reference-issue-modal" data-poster="${comment.poster.displayName}" data-poster-username="${comment.poster.name}" data-reference="${referenceUrl}">${locale.referenceIssue}</div>`;
    }
    if (comment.canEdit) {
      items += html`
        <div class="divider"></div>
        <div class="item js-aria-clickable" data-global-click="onConvContentEdit">${locale.edit}</div>
        <div class="item js-aria-clickable" data-diff-viewer-action="delete">${locale.delete}</div>
      `;
    }
  }
  if (comment.canBlockUser || comment.canBlockOrg) {
    items += html`<div class="divider"></div>`;
    if (comment.canBlockUser) {
      items += html`<div class="item js-aria-clickable show-modal" data-modal="#block-user-modal" data-modal-modal-blockee="${comment.poster.name}" data-modal-modal-blockee-name="${comment.poster.displayName}" data-modal-modal-form.url="${window.config.appSubUrl}/user/settings/blocked_users">${locale.blockUser}</div>`;
    }
    if (comment.canBlockOrg) {
      items += html`<div class="item js-aria-clickable show-modal" data-modal="#block-user-modal" data-modal-modal-blockee="${comment.poster.name}" data-modal-modal-blockee-name="${comment.poster.displayName}" data-modal-modal-form.url="${data.orgBlockLink!}">${locale.blockOrg}</div>`;
    }
  }
  const menu = createElementFromHTML<HTMLElement>(html`<div class="item action ui dropdown jump pointing top right">${svgRaw('octicon-kebab-horizontal')}<div class="menu">${htmlRaw(items)}</div></div>`);
  menu.addEventListener('click', (e) => {
    const item = (e.target as Element).closest<HTMLElement>('[data-diff-viewer-action]');
    const action = item?.getAttribute('data-diff-viewer-action');
    if (action === 'quote') ctx.onQuoteReply(comment, menu);
    else if (action === 'delete') ctx.onDelete(item!, comment);
  });
  return menu;
}
