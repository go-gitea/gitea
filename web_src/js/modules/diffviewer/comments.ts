import {performFetchActionRequest} from '../fetch-action.ts';
import {fetchOk} from './stream.ts';
import {trString} from '../i18n.ts';
import {animateOnce, createElementFromAttrs, createElementFromHTML, toggleElem} from '../../utils/dom.ts';
import {html, htmlEscape, htmlRaw} from '../../utils/html.ts';
import {formatBytes, parseDom} from '../../utils.ts';
import {formatDatetimeISO} from '../../utils/time.ts';
import {svg, svgRaw, type SvgName} from '../../svg.ts';
import type {DiffSide} from './parse.ts';
import {diffTreeFiles, diffTreeStore, type DiffTreeCommentFile, type DiffTreeCommentThread} from '../diff-file.ts';
import {initComboMarkdownEditor, type ComboMarkdownEditor} from '../../features/comp/ComboMarkdownEditor.ts';
import {confirmModal} from '../../features/comp/ConfirmModal.ts';
import {appendQuote, EventCommentContentSaved, extractSelectedMarkdown} from '../../features/repo-issue-edit.ts';
import {showContentHistoryMenuIfEdited} from '../../features/repo-issue-content.ts';
import {showErrorToast} from '../toast.ts';
import {errorMessage} from '../errors.ts';
import type {DiffFileEntry} from './types.ts';
import type {DiffViewer} from './viewer.ts';

// matches diffViewerComment in routers/web/repo/pull_files_comments.go
type ViewerComment = {
  id: number;
  path: string;
  side: DiffSide;
  line: number;
  poster: {name: string; displayName: string; homeLink?: string; avatar: string};
  originalAuthor?: string;
  body: string;
  bodyHtml: string;
  contentVersion: number;
  created: number;
  outdated: boolean;
  pending: boolean;
  reviewId: number;
  resolved: boolean;
  resolvedBy: string;
  canEdit: boolean;
  canBlockUser: boolean;
  canBlockOrg: boolean;
  reactions: Array<{content: string; emojiHtml: string; count: number; users: string; moreCount: number; hasReacted: boolean}>;
  attachments: Array<{name: string; size: number; downloadUrl: string; isImage: boolean; inContent: boolean}>;
};

type CommentsData = {
  isSigned: boolean;
  canComment: boolean;
  canResolve: boolean;
  canReferenceIssue: boolean;
  hasPendingReview: boolean;
  pendingCount: number;
  issueHtmlUrl: string;
  repoLink: string;
  orgBlockLink?: string;
  originalUrl?: string;
  originalHostname?: string;
  migrationIcon?: SvgName;
  reactions: Array<{content: string; emojiHtml: string}>;
  comments: ViewerComment[];
};

type LineTarget = {path: string; side: DiffSide; line: number};

type Thread = {target: LineTarget; comments: ViewerComment[]};

const targetKey = (target: LineTarget) => `${target.path}|${target.side}|${target.line}`;

export class DiffComments {
  host: DiffViewer;
  data: CommentsData = {
    isSigned: false, canComment: false, canResolve: false, canReferenceIssue: false, hasPendingReview: false, pendingCount: 0,
    issueHtmlUrl: '', repoLink: '', reactions: [], comments: [],
  };
  byPath = new Map<string, ViewerComment[]>();
  byId = new Map<number, ViewerComment>();
  drafts = new Map<string, {target: LineTarget; el: HTMLElement}>();
  replyForms = new Map<string, {form: HTMLElement; editorPromise: Promise<ComboMarkdownEditor>}>();
  expandedThreads = new Set<number>();

  constructor(host: DiffViewer) {
    this.host = host;
    // the issue page's edit handler only updates the comment's element
    document.addEventListener(EventCommentContentSaved, () => this.reload());
  }

  async load(): Promise<void> {
    this.data = await (await fetchOk(this.host.config.commentsUrl!)).json();
    this.byPath = Map.groupBy(this.data.comments, (comment) => comment.path);
    this.byId = new Map(this.data.comments.map((comment) => [comment.id, comment]));
    this.publishSummary();
  }

  // the threads of every file for the comments tab, in the order of the file tree
  publishSummary(): void {
    const store = diffTreeStore();
    const result: DiffTreeCommentFile[] = [];
    for (const file of diffTreeFiles(store)) {
      const comments = this.commentsAt(file.FullName, file.OldFullName);
      if (!comments.length) continue;
      const threads = Map.groupBy(comments, (comment) => `${comment.side}|${comment.line}`);
      const summaries = Array.from(threads.values(), (thread): DiffTreeCommentThread => ({
        id: thread[0].id,
        line: thread[0].line,
        posterName: thread[0].originalAuthor || thread[0].poster.displayName,
        avatarUrl: thread[0].originalAuthor ? '' : thread[0].poster.avatar,
        excerpt: parseDom(thread[0].bodyHtml, 'text/html').body.textContent.split('\n').find((line) => line.trim())?.trim() ?? '',
        count: thread.length,
        resolved: thread[0].resolved,
        outdated: thread[0].outdated,
        pending: thread.some((comment) => comment.pending),
      }));
      result.push({name: file.FullName, threads: summaries.sort((a, b) => a.line - b.line)});
    }
    store.commentFiles = result;
  }

  async reload(): Promise<void> {
    try {
      await this.load();
    } catch (error) {
      showErrorToast(errorMessage(error));
      return;
    }
    for (const entry of this.host.entries) this.renderAnnotations(entry);
    this.updatePendingCounter();
  }

  updatePendingCounter(): void {
    const counter = document.querySelector('.review-box-button .review-comments-counter');
    if (!counter) return;
    counter.setAttribute('data-pending-comment-number', String(this.data.pendingCount));
    counter.textContent = String(this.data.pendingCount);
  }

  // comments keep the name a file had before it was renamed
  commentsAt(name: string, oldName: string): ViewerComment[] {
    const comments = this.byPath.get(name) ?? [];
    return oldName && oldName !== name ? [...comments, ...this.byPath.get(oldName) ?? []] : comments;
  }

  commentsFor(entry: DiffFileEntry): ViewerComment[] {
    return this.commentsAt(entry.file.name, entry.file.oldName);
  }

  // comments on lines outside the diff go to line 0
  threadsOf(entry: DiffFileEntry): Map<string, Thread> {
    const threads = new Map<string, Thread>();
    for (const comment of this.commentsFor(entry)) {
      if (this.host.hiddenGap(entry, comment.side, comment.line)) continue;
      const inDiff = this.host.rowIndex(entry, comment.side, comment.line) >= 0;
      const target = {path: entry.file.name, side: comment.side, line: inDiff ? comment.line : 0};
      const key = targetKey(target);
      const thread = threads.get(key);
      if (thread) thread.comments.push(comment);
      else threads.set(key, {target, comments: [comment]});
    }
    return threads;
  }

  threadOf(comment: ViewerComment): Thread | undefined {
    const entry = this.host.findEntry(comment.path);
    return entry ? [...this.threadsOf(entry).values()].find((thread) => thread.comments.includes(comment)) : undefined;
  }

  // the first comment of every thread, in the order they are shown
  orderedThreads(): ViewerComment[] {
    const result: ViewerComment[] = [];
    for (const entry of this.host.visibleEntries) {
      const threads = [...this.threadsOf(entry).values()].map(({target, comments}) => {
        return {row: target.line ? this.host.rowIndex(entry, target.side, target.line) : -1, first: comments[0]};
      });
      threads.sort((a, b) => a.row - b.row);
      result.push(...threads.map((thread) => thread.first));
    }
    return result;
  }

  hasThreadAt(entry: DiffFileEntry, side: DiffSide, line: number): boolean {
    return this.threadsOf(entry).has(targetKey({path: entry.file.name, side, line}));
  }

  // inserts the threads and open composers below their lines, in the split view each side has its own cell
  // previous is the replaced diff of a file rendered again
  renderAnnotations(entry: DiffFileEntry, previous: Element | null = null): void {
    // the diff of LFS objects can't be commented, the comments are about the pointer
    const body = entry.lfsState === null ? entry.element?.querySelector('.code-diff > table > tbody') : null;
    if (!body) return;
    for (const row of body.querySelectorAll(':scope > tr.add-comment')) row.remove();
    for (const button of body.querySelectorAll('.add-code-comment.tw-invisible')) button.classList.remove('tw-invisible');
    const threads = this.threadsOf(entry);
    const drafts = [...this.drafts.values()].filter(({target}) => target.path === entry.file.name);
    if (!threads.size && !drafts.length) return;
    // comments being edited are kept, the editor of the edit handler belongs to their element
    const editing = new Map<number, HTMLElement>();
    for (const zone of (previous ?? body).querySelectorAll('.edit-content-zone:not(.tw-hidden)')) {
      const commentEl = zone.closest<HTMLElement>('.comment')!;
      editing.set(Number(commentEl.getAttribute('data-comment-id')), commentEl);
    }

    const rows = new Map<string, HTMLElement>();
    for (const row of body.children as HTMLCollectionOf<HTMLElement>) {
      for (const side of ['old', 'new'] as const) {
        const line = row.getAttribute(`data-${side}`);
        if (line) rows.set(`${side}|${line}`, row);
      }
    }
    const groups = new Map<HTMLElement | null, Record<DiffSide, HTMLElement[]>>();
    const add = (side: DiffSide, line: number, content: HTMLElement) => {
      const anchor = rows.get(`${side}|${line}`) ?? null;
      let group = groups.get(anchor);
      if (!group) {
        group = {old: [], new: []};
        groups.set(anchor, group);
      }
      group[side].push(content);
      return anchor;
    };
    for (const [key, {target, comments}] of threads) {
      const anchor = add(target.side, target.line, this.buildConversation(key, comments, editing));
      // a line with a conversation gets replies instead of a new thread
      anchor?.querySelector(`.add-code-comment[data-side="${CSS.escape(target.side)}"]`)?.classList.add('tw-invisible');
    }
    for (const {target, el} of drafts) add(target.side, target.line, el);
    for (const [anchor, group] of groups) {
      const row = createElementFromAttrs('tr', {class: 'add-comment'});
      if (this.host.config.isSplit) {
        row.append(
          createElementFromAttrs('td', {class: 'add-comment-left', colspan: 4}, ...group.old),
          createElementFromAttrs('td', {class: 'add-comment-right', colspan: 4}, ...group.new),
        );
      } else {
        row.append(createElementFromAttrs('td', {class: 'add-comment-left add-comment-right', colspan: 5}, ...group.old, ...group.new));
      }
      if (anchor) anchor.after(row);
      else body.prepend(row);
    }
  }

  buildConversation(key: string, comments: ViewerComment[], editing: Map<number, HTMLElement>): HTMLElement {
    const {locale} = this.host;
    const first = comments[0];
    const navButton = (direction: -1 | 1) => {
      const button = createElementFromAttrs('button', {type: 'button', class: 'ui icon tiny basic button'},
        createElementFromHTML(svg(direction < 0 ? 'octicon-arrow-up' : 'octicon-arrow-down', 12)),
        ` ${direction < 0 ? locale.previous : locale.next}`);
      button.addEventListener('click', () => this.navigateFrom(first, direction));
      return button;
    };
    const actions = this.buildThreadActions(key, first);
    actions.prepend(createElementFromAttrs('div', {class: 'ui buttons'}, navButton(-1), navButton(1)));
    const cloud = createElementFromAttrs('div', {class: 'field comment-code-cloud'},
      createElementFromAttrs('div', {class: 'comment-list'},
        createElementFromAttrs('div', {class: 'ui comments'}, ...comments.map((comment) => editing.get(comment.id) ?? this.buildComment(comment))),
      ),
      actions,
    );
    const replyForm = this.replyForms.get(key);
    if (replyForm) cloud.append(replyForm.form);
    const holder = createElementFromAttrs('div', {class: 'diff-conversation', 'data-comment-id': first.id});
    if (first.resolved) holder.append(this.buildResolvedPlaceholder(first, cloud));
    holder.append(cloud);
    return holder;
  }

  buildResolvedPlaceholder(first: ViewerComment, cloud: HTMLElement): HTMLElement {
    const {locale} = this.host;
    const info = createElementFromAttrs('div', {class: 'flex-text-block tw-flex-wrap tw-text-text-light'},
      createElementFromHTML(svg('octicon-check')),
      createElementFromAttrs('b', null, first.resolvedBy),
      ` ${locale.resolvedBy}`,
    );
    if (first.outdated) info.append(createElementFromAttrs('span', {class: 'ui label basic small tw-ml-2'}, locale.outdated));
    const toggle = createElementFromAttrs('button', {type: 'button', class: 'btn tiny'});
    const update = () => {
      const expanded = this.expandedThreads.has(first.id);
      toggleElem(cloud, expanded);
      toggle.replaceChildren(createElementFromHTML(svg(expanded ? 'octicon-fold' : 'octicon-unfold', 16, 'tw-mr-2')), expanded ? locale.hideResolved : locale.showResolved);
    };
    toggle.addEventListener('click', () => {
      if (!this.expandedThreads.delete(first.id)) this.expandedThreads.add(first.id);
      update();
    });
    update();
    return createElementFromAttrs('div', {class: 'resolved-placeholder'}, info, createElementFromAttrs('div', {class: 'flex-text-block'}, toggle));
  }

  buildComment(comment: ViewerComment): HTMLElement {
    const {locale} = this.host;
    const {data} = this;
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
    if (data.canComment && data.reactions.length) headerRight.append(this.buildReactionPicker(comment));
    headerRight.append(this.buildContextMenu(comment));
    const markup = el.querySelector<HTMLElement>('.render-content')!;
    if (comment.canEdit) markup.setAttribute('data-can-edit', 'true');
    if (comment.bodyHtml) markup.innerHTML = comment.bodyHtml;
    else markup.append(createElementFromAttrs('span', {class: 'no-content'}, locale.noContent));
    el.querySelector('.raw-content')!.textContent = comment.body;
    if (comment.attachments.length) el.querySelector('.comment-body')!.append(this.buildAttachments(comment));
    if (comment.reactions.length) el.querySelector('.comment-container')!.append(this.buildReactions(comment));
    showContentHistoryMenuIfEdited(el, comment.id);
    return el;
  }

  // mirrors templates/repo/issue/view_content/attachments.tmpl
  buildAttachments(comment: ViewerComment): HTMLElement {
    const {locale} = this.host;
    const container = createElementFromAttrs('div', {class: 'dropzone-attachments'}, createElementFromAttrs('div', {class: 'divider'}));
    for (const att of comment.attachments) {
      container.append(createElementFromHTML(html`
        <div class="tw-flex">
          <div class="tw-flex-1 tw-p-2">
            <a target="_blank" href="${att.downloadUrl}" title="${trString(locale.openAttachment, att.name)}">
              ${htmlRaw(svg(att.isImage ? 'octicon-file' : 'octicon-desktop-download'))}
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
  buildReactionPicker(comment: ViewerComment): HTMLElement {
    const picker = createElementFromHTML<HTMLElement>(html`
      <div class="item action ui dropdown jump pointing top right select-reaction" aria-label="${this.host.locale.reactions}">
        <a class="muted">${htmlRaw(svg('octicon-smiley'))}</a>
        <div class="menu">${htmlRaw(this.data.reactions.map((reaction) => html`
          <a class="item emoji" data-tooltip-content="${reaction.content}" aria-label="${reaction.content}" data-reaction-content="${reaction.content}">${htmlRaw(reaction.emojiHtml)}</a>
        `).join(''))}
        </div>
      </div>
    `);
    picker.addEventListener('click', (e) => {
      const item = (e.target as Element).closest<HTMLElement>('[data-reaction-content]');
      if (item) this.toggleReaction(item, comment, item.getAttribute('data-reaction-content')!);
    });
    return picker;
  }

  // mirrors templates/repo/issue/view_content/reactions.tmpl
  buildReactions(comment: ViewerComment): HTMLElement {
    const {locale} = this.host;
    const container = createElementFromAttrs('div', {class: 'bottom-reactions', role: 'group', 'aria-label': locale.reactions});
    for (const reaction of comment.reactions) {
      const title = reaction.users + (reaction.moreCount > 0 ? ` ${trString(locale.reactionsMore, reaction.moreCount)}` : '');
      const button = createElementFromHTML<HTMLElement>(html`
        <a role="button" class="ui label basic${reaction.hasReacted ? ' primary' : ''}${this.data.isSigned ? '' : ' disabled'}"
          data-tooltip-content title="${title}" aria-label="${reaction.content}: ${title}" data-tooltip-placement="bottom-start">
          <span class="reaction">${htmlRaw(reaction.emojiHtml)}</span>
          <span class="reaction-count">${reaction.count}</span>
        </a>
      `);
      if (this.data.isSigned) button.addEventListener('click', () => this.toggleReaction(button, comment, reaction.content));
      container.append(button);
    }
    if (this.data.canComment && this.data.reactions.length) container.append(this.buildReactionPicker(comment));
    return container;
  }

  async toggleReaction(el: HTMLElement, comment: ViewerComment, content: string): Promise<void> {
    const hasReacted = comment.reactions.some((reaction) => reaction.content === content && reaction.hasReacted);
    const url = `${this.data.repoLink}/comments/${comment.id}/reactions/${hasReacted ? 'unreact' : 'react'}`;
    if (await performFetchActionRequest(el, {method: 'POST', url, data: new URLSearchParams({content})})) await this.reload();
  }

  // mirrors templates/repo/issue/view_content/context_menu.tmpl
  buildContextMenu(comment: ViewerComment): HTMLElement {
    const {locale} = this.host;
    const {data} = this;
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
      if (action === 'quote') this.quoteReply(comment, menu);
      else if (action === 'delete') this.deleteComment(item!, comment);
    });
    return menu;
  }

  async quoteReply(comment: ViewerComment, menu: HTMLElement): Promise<void> {
    const quoted = extractSelectedMarkdown(menu.closest('.comment')!.querySelector<HTMLElement>('.render-content')!) || comment.body;
    const thread = this.threadOf(comment);
    if (!thread) return;
    appendQuote(await this.openReply(targetKey(thread.target), thread.comments[0], menu.closest('.comment-code-cloud')!), quoted);
  }

  async deleteComment(el: HTMLElement, comment: ViewerComment): Promise<void> {
    if (!await confirmModal({content: this.host.locale.deleteConfirm, confirmButtonColor: 'red'})) return;
    if (await performFetchActionRequest(el, {method: 'POST', url: `${this.data.repoLink}/comments/${comment.id}/delete`})) await this.reload();
  }

  buildThreadActions(key: string, first: ViewerComment): HTMLElement {
    const {locale} = this.host;
    const actions = createElementFromAttrs('div', {class: 'flex-text-block tw-mt-2 tw-flex-wrap tw-justify-end'});
    if (this.data.canResolve && first.reviewId && !first.pending) {
      const resolve = createElementFromAttrs('button', {type: 'button', class: 'ui icon tiny basic button'}, first.resolved ? locale.unresolve : locale.resolve);
      resolve.addEventListener('click', () => this.resolveThread(resolve, first));
      actions.append(resolve);
    }
    if (this.data.canComment) {
      const reply = createElementFromAttrs('button', {type: 'button', class: 'ui primary icon tiny button diff-viewer-reply'}, createElementFromHTML(svg('octicon-reply', 12)), locale.reply);
      toggleElem(reply, !this.replyForms.has(key)); // hidden while the reply form is open
      reply.addEventListener('click', () => this.openReply(key, first, actions.closest('.comment-code-cloud')!));
      actions.append(reply);
    }
    return actions;
  }

  // replies are posted to the line of the first comment, a thread of a line outside the diff is keyed by line 0
  openReply(key: string, first: ViewerComment, cloud: Element): Promise<ComboMarkdownEditor> {
    let reply = this.replyForms.get(key);
    if (!reply) {
      const target = {path: first.path, side: first.side, line: first.line};
      const composer = this.buildComposer([{label: this.host.locale.reply, singleReview: true}], () => {
        this.replyForms.delete(key);
        toggleElem(composer.form.closest('.comment-code-cloud')!.querySelector('.diff-viewer-reply')!, true);
        composer.form.remove();
      }, (content, files, singleReview) => this.submitComment(composer.form, key, target, content, files, singleReview, first.reviewId));
      this.replyForms.set(key, reply = composer);
      cloud.append(reply.form);
      toggleElem(cloud.querySelector('.diff-viewer-reply')!, false);
    }
    return reply.editorPromise;
  }

  async resolveThread(el: HTMLElement, comment: ViewerComment): Promise<void> {
    const data = new URLSearchParams({origin: 'diff', action: comment.resolved ? 'UnResolve' : 'Resolve', comment_id: String(comment.id)});
    if (await performFetchActionRequest(el, {method: 'POST', url: `${this.data.repoLink}/issues/resolve_conversation`, data})) await this.reload();
  }

  openDraft(target: LineTarget): void {
    const key = targetKey(target);
    const entry = this.host.findEntry(target.path);
    if (!this.data.canComment || this.drafts.has(key) || !entry || this.hasThreadAt(entry, target.side, target.line)) return;
    const {locale} = this.host;
    const buttons = this.data.hasPendingReview ?
      [{label: locale.addReviewComment, singleReview: false}] :
      [{label: locale.startReview, singleReview: false}, {label: locale.singleComment, singleReview: true}];
    const draft = createElementFromAttrs('div', {class: 'comment-code-cloud diff-viewer-draft'});
    const composer = this.buildComposer(buttons, () => {
      this.drafts.delete(key);
      draft.closest('tr')?.remove();
    }, (content, files, singleReview) => this.submitComment(composer.form, key, target, content, files, singleReview));
    draft.append(composer.form);
    this.drafts.set(key, {target, el: draft});
    this.renderAnnotations(entry);
  }

  buildComposer(buttons: Array<{label: string; singleReview: boolean}>, onCancel: () => void, onSubmit: (content: string, files: string[], singleReview: boolean) => Promise<void>) {
    const template = document.querySelector<HTMLTemplateElement>('#diff-viewer-editor-template')!;
    const fields = template.content.cloneNode(true) as DocumentFragment;
    const editorEl = fields.querySelector<HTMLElement>('.combo-markdown-editor')!;
    const submitButtons = buttons.map((button, idx) => createElementFromAttrs('button', {
      type: 'submit', class: `ui submit tiny button ${idx === 0 ? 'primary' : 'basic'}`, 'data-single-review': String(button.singleReview),
    }, button.label));
    const cancel = createElementFromAttrs('button', {type: 'button', class: 'ui submit tiny basic button'}, this.host.locale.cancel);
    const form = createElementFromAttrs<HTMLFormElement>('form', {class: 'ui form'}, fields,
      createElementFromAttrs('div', {class: 'field footer'}, createElementFromAttrs('div', {class: 'flex-text-block tw-justify-end'}, ...submitButtons, cancel)),
    );
    // the editor's dropzone is looked up in the form, so it can only be initialized once the form exists
    const editorPromise = (async () => {
      const editor = await initComboMarkdownEditor(editorEl);
      editor.focus();
      return editor;
    })();
    cancel.addEventListener('click', onCancel);
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      const editor = await editorPromise;
      const content = editor.value().trim();
      if (!content || form.classList.contains('is-loading')) return;
      const singleReview = (e.submitter ?? submitButtons[0]).getAttribute('data-single-review') === 'true';
      form.classList.add('is-loading');
      try {
        await onSubmit(content, editor.dropzoneGetFiles() ?? [], singleReview);
      } finally {
        form.classList.remove('is-loading');
      }
    });
    return {form, editorPromise};
  }

  // the composer of the key is dropped once its comment is posted, the comments are reloaded
  async submitComment(form: HTMLElement, key: string, target: LineTarget, content: string, files: string[], singleReview: boolean, replyReviewId = 0): Promise<void> {
    const body = new URLSearchParams({
      origin: 'diff',
      content,
      side: target.side === 'new' ? 'proposed' : 'previous',
      line: String(target.line),
      path: target.path,
      latest_commit_id: this.host.config.headCommitId!,
    });
    if (singleReview) body.set('single_review', 'true');
    if (replyReviewId) body.set('reply', String(replyReviewId));
    for (const file of files) body.append('files', file);
    if (!await performFetchActionRequest(form, {method: 'POST', url: `${this.host.config.issueLink}/files/reviews/comments`, data: body})) return;
    this.drafts.delete(key);
    this.replyForms.delete(key);
    await this.reload();
    const reviewBox = document.querySelector('.review-box-button');
    if (!singleReview && reviewBox) animateOnce(reviewBox, 'pulse-1p5-200');
  }

  async navigateFrom(comment: ViewerComment, direction: -1 | 1): Promise<void> {
    const threads = this.orderedThreads();
    const index = threads.findIndex((first) => first.id === comment.id);
    if (index < 0 || threads.length < 2) return;
    await this.host.scrollToComment(threads[(index + direction + threads.length) % threads.length].id);
  }
}
