import {watch} from 'vue';
import {POST} from '../fetch.ts';
import {trN, trString} from '../i18n.ts';
import {parseHunks, type DiffSide} from './parse.ts';
import {diffTreeFiles, diffTreeStore, diffTreeStoreSetViewed, filteredFileNames, type DiffTreeEntry} from '../diff-file.ts';
import {createElementFromAttrs, isElemVisible, onInputDebounce, toggleElem} from '../../utils/dom.ts';
import {sleep} from '../../utils.ts';
import {html, htmlRaw} from '../../utils/html.ts';
import {svgRaw, type SvgName} from '../../svg.ts';
import {showErrorToast} from '../toast.ts';
import {errorMessage} from '../errors.ts';
import {streamDiff, type DiffStreamFile} from './stream.ts';
import {highlightLines, loadHighlighter, type Segment} from './highlight.ts';
import {escapeButton, highlightHunkLines, linesToHtml, renderSplitRows, renderUnifiedRows, splitColgroup, unifiedColgroup} from './render.ts';
import {detectEscapes, loadEscapeTables, type EscapeTables, type LineEscapes} from './escape.ts';
import {imageDiffHtml, renderCsvView, renderedKind} from './rendered.ts';
import {expandDirections, expandGap, gapOfLine, type ExpandDirection, type Gap} from './expand.ts';
import {applyFolded, buildFileBox, submoduleNotice} from './filebox.ts';
import {fetchFileContent, fetchFullFile, fetchLfsHunks} from './content.ts';
import {DiffFile, type FileContent, type RenderedLines} from './entry.ts';
import {VirtualList} from './virtuallist.ts';
import {DiffComments} from './comments.ts';
import type {DiffViewerConfig, DiffViewerLocale} from './types.ts';

const lineHeight = 20;
const headerHeight = 42;
const noticeHeight = 45;

function treeEntryToStreamFile(item: DiffTreeEntry): DiffStreamFile {
  return {name: item.FullName, oldName: item.OldFullName || item.FullName, nameHash: item.NameHash, type: item.DiffStatus, tabSize: 4};
}

// streams the diff into DiffFile entries and shows the ones in view in a VirtualList, each file as one table built by render.ts,
// the files' content is loaded by content.ts for highlighting and expanding, the code comments are handled by DiffComments
export class DiffViewer {
  root: HTMLElement;
  config: DiffViewerConfig;
  locale: DiffViewerLocale;
  entries: DiffFile[] = [];
  entriesByName = new Map<string, DiffFile>();
  list: VirtualList<DiffFile>;
  comments: DiffComments | null = null;
  escapeTables: EscapeTables | null = null;
  streamState: 'loading' | 'complete' | 'truncated' | 'failed' = 'loading'; // unless complete, the missing files can be loaded one by one
  visibleUpdateQueued = false;
  hashPending = true; // the file of the URL hash may still be streaming

  constructor(root: HTMLElement, config: DiffViewerConfig) {
    this.root = root;
    this.config = config;
    this.locale = JSON.parse(root.getAttribute('data-locale')!);
    this.list = new VirtualList<DiffFile>(root, {
      key: (entry) => entry.file.name,
      estimateHeight: (entry) => this.estimateHeight(entry),
      element: (entry) => entry.element ?? this.buildFileElement(entry),
    });
    this.list.element.addEventListener('click', (e) => this.onListClick(e));
    if (config.commentsUrl) this.comments = new DiffComments(this);
  }

  // the entries passing the file tree's filters
  get visibleEntries(): DiffFile[] {
    return this.list.items;
  }

  async init(): Promise<void> {
    const status = this.root.querySelector('.diff-viewer-status')!;
    const escapeTablesPromise = this.config.detectAmbiguous ? this.loadEscapeTables() : null; // tiny, loads while the diff streams
    const commentsPromise = this.loadComments(); // loads while the diff streams
    try {
      let painted: Promise<void> | undefined;
      const end = await streamDiff(this.config.streamUrl, (file) => {
        this.addEntry(file, false);
        painted ??= this.firstPaint(status, file, escapeTablesPromise, commentsPromise);
        if (this.list.started) this.queueVisibleEntriesUpdate();
      });
      await Promise.all([painted, commentsPromise]);
      this.streamState = end.truncated ? 'truncated' : 'complete';
      if (end.failed) throw new Error('The diff stream ended unexpectedly');
    } catch (error) {
      console.error(error); // also when leaving the page while loading
      this.streamState = 'failed';
      this.root.prepend(createElementFromAttrs('div', {class: 'ui error message'}, this.locale.loadFailed));
    } finally {
      status.remove();
      this.addMissingEntries();
      if (this.entries.length) this.list.start();
      this.updateVisibleEntries();
    }
    if (this.streamState === 'truncated') {
      const loaded = this.entries.filter((entry) => !entry.missing).length;
      this.root.prepend(createElementFromAttrs('div', {class: 'ui warning message'}, trString(this.locale.tooManyFiles, loaded, this.entries.length)));
    }
    this.list.updateScrollMargin(); // the messages above the list move it
    const store = diffTreeStore();
    watch(() => store.filenameFilterQuery, onInputDebounce(() => this.updateVisibleEntries()));
    watch(() => [store.activeExtensions, store.activeStatuses, store.commentedOnly && store.commentFiles], () => this.updateVisibleEntries(), {deep: true});
    this.hashPending = false;
    window.addEventListener('hashchange', () => this.scrollToHash());
    // a link to the current hash doesn't change it, but its target may be scrolled out of the rendered files
    document.addEventListener('click', (e) => {
      if ((e.target as Element).closest('a[href^="#"]')?.getAttribute('href') === window.location.hash) this.scrollToHash();
    });
    const observer = new ResizeObserver(() => this.list.updateScrollMargin());
    for (const el of document.querySelectorAll('.diff-detail-box, #diff-viewer-review-box')) observer.observe(el);
  }

  // without the comments, the diff is shown read-only because commenting isn't allowed by default
  async loadComments(): Promise<void> {
    try {
      await this.comments?.load();
    } catch (error) {
      console.error(error);
      showErrorToast(errorMessage(error));
      return;
    }
    for (const entry of this.entries) if (entry.element) this.renderBody(entry); // the files painted before they arrived
  }

  // waits briefly for the first file's grammar and the comments, so the first paint usually has them, and for the rest it needs
  async firstPaint(status: Element, file: DiffStreamFile, escapeTablesPromise: Promise<void> | null, commentsPromise: Promise<void>): Promise<void> {
    const highlighterPromise = this.config.disableHighlight ? null : loadHighlighter(file.name, file.language);
    await Promise.all([Promise.race([Promise.all([highlighterPromise, commentsPromise]), sleep(300)]), escapeTablesPromise]);
    status.remove();
    this.updateVisibleEntries();
    this.list.start(); // the stream may have failed meanwhile and started it
    this.scrollToHash();
    this.list.queueMount();
  }

  addEntry(file: DiffStreamFile, missing: boolean): DiffFile {
    const treeItem = diffTreeStore().fullNameMap[file.name];
    const entry = new DiffFile(file, parseHunks(file.hunks ?? ''), missing, Boolean(treeItem?.IsViewed || file.isGenerated));
    this.entries.push(entry);
    this.entriesByName.set(file.name, entry);
    if (treeItem && !missing) {
      const counts = entry.changeCounts();
      treeItem.Additions = counts.additions;
      treeItem.Deletions = counts.deletions;
    }
    if (entry.hunks.length) this.loadEntryHighlighter(entry);
    return entry;
  }

  findEntry(path: string): DiffFile | undefined {
    return this.entriesByName.get(path) ?? this.entries.find((entry) => entry.file.oldName === path);
  }

  // files without changes left after ignoring whitespace aren't streamed, neither are the ones beyond the size limit
  addMissingEntries(): void {
    for (const item of diffTreeFiles(diffTreeStore())) {
      if (!this.entriesByName.has(item.FullName)) this.addEntry(treeEntryToStreamFile(item), true);
    }
  }

  async loadEntryHighlighter(entry: DiffFile): Promise<void> {
    if (this.config.disableHighlight) {
      entry.highlighter = null;
      return;
    }
    entry.highlighter = await loadHighlighter(entry.file.name, entry.file.language);
    if (entry.highlighter && entry.element && entry.renderedLines) {
      // the file was rendered before its grammar loaded
      entry.invalidateHighlighting();
      this.renderBody(entry);
    }
  }

  // the content of every line in hunk order, highlighted and with the changed words of paired lines marked
  renderLines(entry: DiffFile): RenderedLines {
    if (entry.renderedLines) return entry.renderedLines;
    const {lines} = entry;
    let segments: Segment[][];
    if (entry.content) {
      // with the full file, the lines are highlighted with the context of the whole file
      const highlight = (code: string[]) => highlightLines(entry.highlighter, code);
      entry.contentSegments ??= {old: entry.content.old && highlight(entry.content.old), new: highlight(entry.content.new)};
      const {old: oldSegments, new: newSegments} = entry.contentSegments;
      segments = lines.map((line) => {
        const lineSegments = line.type === 'del' ? oldSegments?.[line.oldNum - 1] : newSegments[line.newNum - 1];
        // the raw file may be decoded differently than the hunks, like a stripped BOM, then the ranges wouldn't match
        return lineSegments?.map(([text]) => text).join('') === line.content ? lineSegments : [[line.content, '']];
      });
    } else {
      segments = highlightHunkLines(lines, entry.highlighter);
    }
    const escapes = lines.map((line) => this.lineEscapes(line.content));
    entry.renderedLines = {
      html: linesToHtml(lines, segments, entry.linePairs(), escapes),
      escapeButtons: escapes.map((escape) => escapeButton(escape, this.locale)),
    };
    return entry.renderedLines;
  }

  // the files of the commits can be loaded for text diffs when the repo has file pages, the wiki doesn't
  canLoadContent(entry: DiffFile): boolean {
    const {file} = entry;
    return Boolean(this.config.repoLink) && !file.isBinary && !file.lfs && !file.tooLarge && !entry.missing && entry.hunks.length > 0;
  }

  isExpandable(entry: DiffFile): boolean {
    return this.canLoadContent(entry) && entry.file.type !== 'added' && entry.file.type !== 'deleted';
  }

  gaps(entry: DiffFile): Gap[] {
    return entry.gaps(this.isExpandable(entry));
  }

  // the gap a line is hidden in, comments on hidden lines show up when their gap gets expanded
  hiddenGap(entry: DiffFile, side: DiffSide, line: number): Gap | null {
    return gapOfLine(this.gaps(entry), side, line);
  }

  // the expand buttons of the gaps, with the number of comments hidden in them
  expanderButtons(entry: DiffFile): Map<number, string> {
    const gaps = this.gaps(entry);
    const hiddenComments = new Map<number, number>();
    for (const comment of this.comments?.commentsFor(entry) ?? []) {
      const gap = gapOfLine(gaps, comment.side, comment.line);
      if (gap) hiddenComments.set(gap.index, (hiddenComments.get(gap.index) ?? 0) + 1);
    }
    const icons: Record<ExpandDirection, SvgName> = {up: 'octicon-fold-up', down: 'octicon-fold-down', all: 'octicon-fold'};
    const buttons = new Map<number, string>();
    for (const gap of gaps) {
      const directions = expandDirections(gap, entry.hunks.length);
      const commentCount = hiddenComments.get(gap.index);
      const badge = commentCount ? html`<span class="code-comment-more" data-tooltip-content="${trN(commentCount, this.locale.numComments1, this.locale.numComments)}">${commentCount}</span>` : '';
      buttons.set(gap.index, html`<div class="code-expander-buttons"${htmlRaw(directions.length === 2 ? ' data-expand-direction="updown"' : '')}>${htmlRaw(badge)}${htmlRaw(directions.map((direction) => html`
        <button type="button" class="code-expander-button" data-expand="${direction}">${svgRaw(icons[direction])}</button>
      `).join(''))}</div>`);
    }
    return buttons;
  }

  // like the legacy viewer, small files are highlighted as a whole so that multi-line constructs around the hunks are known
  async highlightWithContent(entry: DiffFile): Promise<void> {
    if (this.config.disableHighlight || entry.contentPromise || !this.canLoadContent(entry)) return;
    try {
      await this.loadContent(entry);
    } catch {
      return; // the hunks stay highlighted on their own
    }
    if (entry.element) {
      entry.renderedLines = null;
      this.renderBody(entry);
    }
  }

  loadContent(entry: DiffFile): Promise<FileContent> {
    entry.contentPromise ??= this.fetchContent(entry);
    return entry.contentPromise;
  }

  async fetchContent(entry: DiffFile): Promise<FileContent> {
    try {
      entry.setContent(await fetchFileContent(this.config, entry.file, this.locale.fileSuppressed));
    } catch (error) {
      entry.contentPromise = null; // a later expansion tries again
      throw error;
    }
    return entry.content!;
  }

  async expand(entry: DiffFile, gapIndex: number, direction: ExpandDirection): Promise<void> {
    const content = await this.loadContent(entry);
    const gap = this.gaps(entry).find((candidate) => candidate.index === gapIndex);
    if (gap) expandGap(entry.hunks, gap, direction, content.new);
    entry.invalidateRows();
    if (entry.element) this.renderBody(entry);
  }

  lineEscapes(text: string): LineEscapes | null {
    if (!this.escapeTables) return null; // disabled, or the diff failed to load before they did
    return detectEscapes(text, this.escapeTables, (char, confusable) => trString(this.locale.ambiguousCharacter, char, confusable));
  }

  async loadEscapeTables(): Promise<void> {
    this.escapeTables = await loadEscapeTables(document.documentElement.lang);
  }

  estimateHeight(entry: DiffFile): number {
    if (entry.folded) return headerHeight;
    if (this.bodyNotice(entry) !== null) return headerHeight + noticeHeight;
    return headerHeight + entry.rowCount(this.config.isSplit) * lineHeight;
  }

  // returns the notice shown instead of the lines, or null if the lines are shown
  bodyNotice(entry: DiffFile): string | null {
    const {file} = entry;
    const {locale} = this;
    if (entry.missing) return this.streamState === 'complete' ? locale.whitespaceOnly : locale.fileSuppressed;
    if (file.tooLarge) return locale.fileSuppressed;
    if (file.isBinary) return locale.binNotShown;
    if (file.submodule) return '';
    if (entry.hunks.length) return null;
    if (file.oldName !== file.name) return locale.renamedNoChanges;
    if (file.oldMode && file.mode) return locale.modeChanged;
    return locale.whitespaceOnly;
  }

  renderBody(entry: DiffFile): void {
    if (!entry.content) this.highlightWithContent(entry);
    const body = entry.element!.querySelector<HTMLElement>('.diff-file-body')!;
    const oldSource = body.querySelector('.code-diff');
    const source = this.buildSource(entry);
    // a re-render keeps the escaping and the view the user switched to
    if (oldSource) source.classList.toggle('unicode-escaped', oldSource.classList.contains('unicode-escaped'));
    const kind = renderedKind(entry, this.config);
    if (!kind) {
      body.replaceChildren(source);
    } else {
      // the header buttons switch between the source and the rendered view, which is built once
      const {nameHash} = entry.file;
      source.id = `diff-source-${nameHash}`;
      const oldRendered = body.querySelector<HTMLElement>(`#diff-rendered-${nameHash}`);
      const rendered = oldRendered ?? createElementFromAttrs('div', {
        id: `diff-rendered-${nameHash}`, class: `file-body file-code ${this.config.isSplit ? 'code-diff-split' : 'code-diff-unified'} tw-overflow-x-scroll`,
      });
      toggleElem(source, Boolean(oldRendered?.classList.contains('tw-hidden')));
      body.replaceChildren(source, rendered);
      if (!oldRendered && kind === 'image') {
        rendered.innerHTML = imageDiffHtml(entry, this.config, this.locale);
      } else if (!oldRendered) {
        renderCsvView(entry, rendered, async () => {
          const content = await this.loadContent(entry);
          return {old: content.old?.join('\n') ?? null, new: entry.file.type === 'deleted' ? null : content.new.join('\n')};
        }, this.config.csvMaxFileSize, this.locale);
      }
    }
    this.comments?.renderAnnotations(entry, oldSource);
  }

  buildSource(entry: DiffFile): HTMLElement {
    const source = createElementFromAttrs('div', {class: `file-body file-code unicode-escaped code-diff ${this.config.isSplit ? 'code-diff-split' : 'code-diff-unified'}`});
    const notice = entry.file.lfs ? this.lfsNotice(entry) : this.bodyNotice(entry);
    if (notice === null) {
      const readOnly = entry.lfsState === 'diff';
      const {html: lineHtml, escapeButtons} = this.renderLines(entry);
      const opts = {
        addCommentLabel: this.comments?.data.canComment && !readOnly ? this.locale.addLineComment : null,
        expanders: this.expanderButtons(entry),
        escapeButtons,
      };
      const {hunks} = entry;
      const rows = this.config.isSplit ?
        html`${htmlRaw(splitColgroup)}<tbody>${htmlRaw(renderSplitRows(hunks, lineHtml, entry.linePairs(), opts))}</tbody>` :
        html`${htmlRaw(unifiedColgroup)}<tbody>${htmlRaw(renderUnifiedRows(hunks, lineHtml, opts))}</tbody>`;
      source.classList.add('lezer-highlight');
      if (entry.highlighter?.language) source.setAttribute('data-language', entry.highlighter.language);
      source.innerHTML = html`<table class="chroma">${htmlRaw(rows)}</table>`;
      return source;
    }
    if (entry.file.submodule) {
      source.append(submoduleNotice(entry.file, this.locale));
      return source;
    }
    const noticeEl = createElementFromAttrs('div', {class: 'tw-p-3'}, notice);
    if (entry.lfsState === 'tooLarge' || (entry.missing ? this.streamState !== 'complete' : entry.file.tooLarge)) {
      const load = createElementFromAttrs('button', {type: 'button', class: 'ui basic tiny button tw-ml-2'}, this.locale.load);
      load.addEventListener('click', () => {
        if (!entry.file.lfs) {
          this.loadFullFile(entry, load);
          return;
        }
        this.loadLfs(entry);
        this.renderBody(entry);
      });
      noticeEl.append(load);
    }
    source.append(noticeEl);
    return source;
  }

  // LFS objects are diffed by their content, loading it automatically when it's small enough
  lfsNotice(entry: DiffFile): string | null {
    const {locale} = this;
    if (entry.lfsState === null) {
      if (renderedKind(entry, this.config) === 'image') return locale.binNotShown; // compared by the image diff
      const {old: oldObject, new: newObject} = entry.file.lfs!;
      const size = Math.max(oldObject?.exists ? oldObject.size : 0, newObject?.exists ? newObject.size : 0);
      if (this.config.maxFileSize >= 0 && size > this.config.maxFileSize) entry.lfsState = 'tooLarge';
      else this.loadLfs(entry);
    }
    switch (entry.lfsState) {
      case 'loading': return locale.loading;
      case 'tooLarge': return locale.fileSuppressed;
      case 'binary': return locale.binNotShown;
      case 'failed': return locale.loadFailed;
      case 'diff': case null: return null;
    }
  }

  // the caller renders the "loading" state, the result is rendered once it's loaded
  async loadLfs(entry: DiffFile): Promise<void> {
    entry.lfsState = 'loading';
    try {
      const hunks = await fetchLfsHunks(this.config, entry.file);
      if (hunks) {
        entry.hunks = hunks;
        entry.lfsState = 'diff';
      } else {
        entry.lfsState = 'binary';
      }
    } catch (error) {
      console.error(error);
      entry.lfsState = 'failed';
    }
    if (entry.element) this.renderBody(entry);
  }

  async loadFullFile(entry: DiffFile, button: HTMLElement): Promise<void> {
    button.classList.add('is-loading');
    let loaded: DiffStreamFile | null;
    try {
      loaded = await fetchFullFile(this.config, entry.file);
    } catch (error) {
      showErrorToast(errorMessage(error));
      return;
    } finally {
      button.classList.remove('is-loading');
    }
    if (!loaded) {
      button.remove();
      showErrorToast(this.locale.fileSuppressed);
      return;
    }
    entry.file = loaded;
    entry.missing = false;
    entry.hunks = parseHunks(loaded.hunks ?? '');
    if (entry.highlighter === undefined) await this.loadEntryHighlighter(entry);
    // the header shows the file's data too, so the element is built again
    this.list.unmount(entry);
    entry.element = null;
  }

  buildFileElement(entry: DiffFile): HTMLElement {
    const box = buildFileBox(entry, this.config, this.locale, renderedKind(entry, this.config) !== null);
    entry.element = box;
    applyFolded(entry, entry.folded);
    box.querySelector('.fold-file')!.addEventListener('click', () => this.setFolded(entry, !entry.folded));
    box.querySelector<HTMLInputElement>('.viewed-file-form input')?.addEventListener('change', (e) => {
      this.setViewed(entry, (e.target as HTMLInputElement).checked);
    });
    this.renderBody(entry);
    return box;
  }

  setFolded(entry: DiffFile, folded: boolean): void {
    applyFolded(entry, folded);
    if (folded && entry.element && entry.element.getBoundingClientRect().top < 0) {
      const index = this.visibleEntries.indexOf(entry);
      if (index >= 0) this.list.scrollToIndex(index);
    }
  }

  setAllFolded(folded: boolean): void {
    for (const entry of this.entries) applyFolded(entry, folded);
    this.list.measure();
  }

  async setViewed(entry: DiffFile, viewed: boolean): Promise<void> {
    const box = entry.element!;
    box.querySelector('.viewed-file-form')!.classList.toggle('viewed-file-checked-form', viewed);
    box.querySelector('.changed-since-last-review')?.remove();
    this.setFolded(entry, viewed);
    diffTreeStoreSetViewed(diffTreeStore(), entry.file.name, viewed);
    updateViewedFilesSummary(viewed ? 1 : -1);
    try {
      const response = await POST(`${this.config.issueLink}/viewed-files`, {data: {files: {[entry.file.name]: viewed}, headCommitSHA: this.config.headCommitId}});
      if (!response.ok) throw new Error(`Failed to update the viewed state: ${response.status}`);
    } catch (error) {
      showErrorToast(errorMessage(error));
    }
  }

  async scrollToComment(commentId: number): Promise<void> {
    const comment = this.comments?.byId.get(commentId);
    if (!comment) return;
    const entry = this.findEntry(comment.path);
    const index = entry ? this.visibleEntries.indexOf(entry) : -1;
    if (!entry || index < 0) return;
    if (entry.folded) applyFolded(entry, false);
    const gap = this.hiddenGap(entry, comment.side, comment.line);
    if (gap) {
      try {
        await this.expand(entry, gap.index, 'all');
      } catch (error) {
        showErrorToast(errorMessage(error));
      }
    }
    const first = this.comments!.threadOf(comment)?.comments[0];
    if (first?.resolved && !this.comments!.expandedThreads.has(first.id)) {
      this.comments!.expandedThreads.add(first.id);
      this.comments!.renderAnnotations(entry);
    }
    this.list.scrollToIndex(index);
    const anchor = `issuecomment-${commentId}`;
    // waiting at least once also lets a triggering click finish, the issue page clears the hash on clicks outside of its target
    for (let frame = 0; frame < 60; frame++) {
      await new Promise((resolve) => requestAnimationFrame(resolve));
      if (document.querySelector(`#${anchor}`)) break;
    }
    // navigating to the hash lets the browser scroll to the comment and highlight it with ":target"
    if (window.location.hash === `#${anchor}`) document.querySelector(`#${anchor}`)?.scrollIntoView();
    else window.location.assign(`#${anchor}`);
  }

  async expandByButton(button: HTMLElement): Promise<void> {
    const entry = this.list.itemOf(button.closest<HTMLElement>('.diff-file-box')!)!;
    button.classList.add('is-loading');
    try {
      await this.expand(entry, Number(button.closest('tr')!.getAttribute('data-hunk')), button.getAttribute('data-expand') as ExpandDirection);
    } catch (error) {
      button.classList.remove('is-loading');
      showErrorToast(errorMessage(error));
    }
  }

  onListClick(e: MouseEvent): void {
    const target = e.target as HTMLElement;
    const expander = target.closest<HTMLElement>('.code-expander-button');
    if (expander) {
      this.expandByButton(expander);
      return;
    }
    if (!this.comments) return;
    const row = target.closest<HTMLElement>('tr[data-line-type]:not([data-line-type="tag"])');
    const box = target.closest<HTMLElement>('.diff-file-box');
    if (!row || !box) return;
    let side: DiffSide;
    if (target.closest('.add-code-comment, .lines-num-old:not([data-line-num=""])')) {
      side = target.closest('.add-code-comment')?.getAttribute('data-side') as DiffSide ?? 'old';
    } else if (target.closest('.lines-num-new:not([data-line-num=""])')) {
      side = 'new';
    } else {
      return;
    }
    // the line numbers open what the side's button would, there is none for read-only diffs or the old side of expanded lines
    if (!row.querySelector(`.add-code-comment[data-side="${CSS.escape(side)}"]`)) return;
    this.comments.openDraft({path: this.list.itemOf(box)!.file.name, side, line: Number(row.getAttribute(`data-${side}`))});
  }

  queueVisibleEntriesUpdate(): void {
    if (this.visibleUpdateQueued) return;
    this.visibleUpdateQueued = true;
    requestAnimationFrame(() => {
      this.visibleUpdateQueued = false;
      this.updateVisibleEntries();
    });
  }

  updateVisibleEntries(): void {
    const names = filteredFileNames(diffTreeStore());
    this.list.setItems(names ? this.entries.filter((entry) => names.has(entry.file.name)) : this.entries);
    toggleElem('#diff-no-matches', this.entries.length > 0 && this.visibleEntries.length === 0);
    if (this.list.started && this.hashPending) this.scrollToHash();
  }

  scrollToHash(): void {
    const hash = window.location.hash.slice(1);
    const commentId = /^issuecomment-(\d+)$/.exec(hash)?.[1];
    if (commentId) {
      const comment = this.comments?.byId.get(Number(commentId));
      const entry = comment && this.findEntry(comment.path);
      if (!entry || !this.visibleEntries.includes(entry)) return; // its file may still be streaming
      this.hashPending = false;
      // when the comment is shown, the browser has scrolled to it already, otherwise it may be collapsed or not rendered yet
      const commentEl = document.querySelector<HTMLElement>(`#${CSS.escape(hash)}`);
      if (!commentEl || !isElemVisible(commentEl)) this.scrollToComment(Number(commentId));
      return;
    }
    const index = this.visibleEntries.findIndex((entry) => `diff-${entry.file.nameHash}` === hash);
    if (index < 0) return;
    this.hashPending = false;
    const entry = this.visibleEntries[index];
    if (entry.folded) applyFolded(entry, false);
    this.list.scrollToIndex(index);
  }
}

function updateViewedFilesSummary(delta: number): void {
  const prReview = window.config.pageData.prReview;
  if (!prReview) return;
  prReview.numberOfViewedFiles += delta;
  document.querySelector('#viewed-files-summary')?.setAttribute('value', String(prReview.numberOfViewedFiles));
  const label = document.querySelector<HTMLElement>('#viewed-files-summary-label');
  if (label) label.textContent = trString(label.getAttribute('data-text-changed-template')!, prReview.numberOfViewedFiles, prReview.numberOfFiles);
}
