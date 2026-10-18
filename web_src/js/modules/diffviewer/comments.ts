import {performFetchActionRequest} from '../fetch-action.ts';
import {fetchOk} from './stream.ts';
import {animateOnce, createElementFromAttrs, createElementFromHTML, toggleElem} from '../../utils/dom.ts';
import {parseDom} from '../../utils.ts';
import {svg} from '../../svg.ts';
import type {DiffSide} from './parse.ts';
import type {Gap} from './expand.ts';
import {diffTreeFiles, diffTreeStore, type DiffTreeCommentFile, type DiffTreeCommentThread} from '../diff-file.ts';
import {initComboMarkdownEditor, type ComboMarkdownEditor} from '../../features/comp/ComboMarkdownEditor.ts';
import {confirmModal} from '../../features/comp/ConfirmModal.ts';
import {appendQuote, EventCommentContentSaved, extractSelectedMarkdown} from '../../features/repo-issue-edit.ts';
import {showErrorToast} from '../toast.ts';
import {errorMessage} from '../errors.ts';
import {buildCommentElement} from './comment-element.ts';
import type {DiffFile} from './entry.ts';
import type {CommentsData, DiffViewerConfig, DiffViewerLocale, ViewerComment} from './types.ts';

// what the comments need from the viewer
export type CommentsHost = {
  config: DiffViewerConfig;
  locale: DiffViewerLocale;
  entries: DiffFile[];
  visibleEntries: DiffFile[];
  findEntry(path: string): DiffFile | undefined;
  hiddenGap(entry: DiffFile, side: DiffSide, line: number): Gap | null;
  scrollToComment(commentId: number): Promise<void>;
};

type LineTarget = {path: string; side: DiffSide; line: number};

type Thread = {target: LineTarget; comments: ViewerComment[]};

type Composer = {form: HTMLFormElement; editorPromise: Promise<ComboMarkdownEditor>};

const targetKey = (target: LineTarget) => `${target.path}|${target.side}|${target.line}`;

// the code comments of a pull request: their threads below the lines, the composers of new comments and replies
export class DiffComments {
  host: CommentsHost;
  data: CommentsData = {
    isSigned: false, canComment: false, canResolve: false, canReferenceIssue: false, hasPendingReview: false, pendingCount: 0,
    issueHtmlUrl: '', repoLink: '', reactions: [], comments: [],
  };
  byPath = new Map<string, ViewerComment[]>();
  byId = new Map<number, ViewerComment>();
  drafts = new Map<string, {target: LineTarget; el: HTMLElement}>();
  replyForms = new Map<string, Composer>();
  expandedThreads = new Set<number>(); // resolved threads which were unfolded

  constructor(host: CommentsHost) {
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

  commentsFor(entry: DiffFile): ViewerComment[] {
    return this.commentsAt(entry.file.name, entry.file.oldName);
  }

  // the threads of the shown lines, comments on lines outside the diff go to line 0, comments in hidden gaps wait for their expansion
  threadsOf(entry: DiffFile): Map<string, Thread> {
    const threads = new Map<string, Thread>();
    for (const comment of this.commentsFor(entry)) {
      if (this.host.hiddenGap(entry, comment.side, comment.line)) continue;
      const inDiff = entry.rowIndex(comment.side, comment.line) >= 0;
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
        return {row: target.line ? entry.rowIndex(target.side, target.line) : -1, first: comments[0]};
      });
      threads.sort((a, b) => a.row - b.row);
      result.push(...threads.map((thread) => thread.first));
    }
    return result;
  }

  hasThreadAt(entry: DiffFile, side: DiffSide, line: number): boolean {
    return this.threadsOf(entry).has(targetKey({path: entry.file.name, side, line}));
  }

  // inserts the threads and open composers below their lines, in the split view each side has its own cell
  // previous is the replaced diff of a file rendered again
  renderAnnotations(entry: DiffFile, previous: Element | null = null): void {
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
    // the annotations below a row, grouped by side, the ones of line 0 go above the first row
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
    const commentContext = {
      data: this.data,
      locale,
      onToggleReaction: (el: HTMLElement, comment: ViewerComment, content: string) => this.toggleReaction(el, comment, content),
      onQuoteReply: (comment: ViewerComment, menu: HTMLElement) => this.quoteReply(comment, menu),
      onDelete: (el: HTMLElement, comment: ViewerComment) => this.deleteComment(el, comment),
    };
    const cloud = createElementFromAttrs('div', {class: 'field comment-code-cloud'},
      createElementFromAttrs('div', {class: 'comment-list'},
        createElementFromAttrs('div', {class: 'ui comments'}, ...comments.map((comment) => editing.get(comment.id) ?? buildCommentElement(comment, commentContext))),
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

  async toggleReaction(el: HTMLElement, comment: ViewerComment, content: string): Promise<void> {
    const hasReacted = comment.reactions.some((reaction) => reaction.content === content && reaction.hasReacted);
    const url = `${this.data.repoLink}/comments/${comment.id}/reactions/${hasReacted ? 'unreact' : 'react'}`;
    if (await performFetchActionRequest(el, {method: 'POST', url, data: new URLSearchParams({content})})) await this.reload();
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

  // opens the composer of a new thread below a line
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

  buildComposer(buttons: Array<{label: string; singleReview: boolean}>, onCancel: () => void, onSubmit: (content: string, files: string[], singleReview: boolean) => Promise<void>): Composer {
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
