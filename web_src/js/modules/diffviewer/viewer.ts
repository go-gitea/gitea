import {Virtualizer, measureElement, observeWindowOffset, observeWindowRect, windowScroll} from '@tanstack/virtual-core';
import {watch} from 'vue';
import {POST} from '../fetch.ts';
import {trN, trString} from '../i18n.ts';
import {parseHunks, splitLines, type DiffSide} from './parse.ts';
import {diffTreeFiles, diffTreeStore, diffTreeStoreSetViewed, filteredFileNames, type DiffTreeEntry} from '../diff-file.ts';
import {createElementFromAttrs, isElemVisible, onInputDebounce, toggleElem} from '../../utils/dom.ts';
import {sleep} from '../../utils.ts';
import {html, htmlRaw} from '../../utils/html.ts';
import {svg, type SvgName} from '../../svg.ts';
import {showErrorToast} from '../toast.ts';
import {errorMessage} from '../errors.ts';
import {fetchOk, streamDiff, type DiffStreamFile} from './stream.ts';
import {highlightLines, loadHighlighter, type Segment} from './highlight.ts';
import {escapeButton, highlightHunkLines, hunksLineCount, linesToHtml, pairChangedLines, renderSplitRows, renderUnifiedRows, splitColgroup, unifiedColgroup} from './render.ts';
import {buildHunks} from './linediff.ts';
import {detectEscapes, loadEscapeTables, type EscapeTables, type LineEscapes} from './escape.ts';
import {imageDiffHtml, renderCsvView, renderedKind} from './rendered.ts';
import {expandDirections, expandGap, gapOfLine, gapsOf, type ExpandDirection, type Gap} from './expand.ts';
import {applyFolded, buildFileBox, fileUrl, submoduleNotice} from './filebox.ts';
import {DiffComments} from './comments.ts';
import type {DiffFileEntry, DiffViewerConfig, DiffViewerLocale} from './types.ts';

const lineHeight = 20;
const headerHeight = 42;
const noticeHeight = 45;
const stickyTopHeight = 44; // matches .diff-file-box scroll-margin-top

function treeEntryToStreamFile(item: DiffTreeEntry): DiffStreamFile {
  return {name: item.FullName, oldName: item.OldFullName || item.FullName, nameHash: item.NameHash, type: item.DiffStatus, tabSize: 4};
}

export class DiffViewer {
  root: HTMLElement;
  config: DiffViewerConfig;
  locale: DiffViewerLocale;
  entries: DiffFileEntry[] = [];
  entriesByName = new Map<string, DiffFileEntry>();
  entriesByElement = new WeakMap<HTMLElement, DiffFileEntry>();
  visibleEntries: DiffFileEntry[] = [];
  mounted = new Set<DiffFileEntry>();
  list: HTMLElement;
  virtualizer: Virtualizer<Window, HTMLElement> | null = null;
  comments: DiffComments | null = null;
  escapeTables: EscapeTables | null = null;
  streamState: 'loading' | 'complete' | 'truncated' | 'failed' = 'loading'; // unless complete, the missing files can be loaded one by one
  mountQueued = false;
  visibleUpdateQueued = false;
  hashPending = true; // the file of the URL hash may still be streaming

  constructor(root: HTMLElement, config: DiffViewerConfig) {
    this.root = root;
    this.config = config;
    this.locale = JSON.parse(root.getAttribute('data-locale')!);
    this.list = createElementFromAttrs('div', {class: 'tw-relative'});
    this.list.addEventListener('click', (e) => this.onListClick(e));
    if (config.commentsUrl) this.comments = new DiffComments(this);
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
        if (this.virtualizer) this.queueVisibleEntriesUpdate();
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
      if (this.entries.length) this.virtualizer ??= this.createVirtualizer();
      this.updateVisibleEntries();
    }
    if (this.streamState === 'truncated') {
      const loaded = this.entries.filter((entry) => !entry.missing).length;
      this.root.prepend(createElementFromAttrs('div', {class: 'ui warning message'}, trString(this.locale.tooManyFiles, loaded, this.entries.length)));
    }
    this.updateScrollMargin(); // the messages above the list move it
    const store = diffTreeStore();
    watch(() => store.filenameFilterQuery, onInputDebounce(() => this.updateVisibleEntries()));
    watch(() => [store.activeExtensions, store.activeStatuses, store.commentedOnly && store.commentFiles], () => this.updateVisibleEntries(), {deep: true});
    this.hashPending = false;
    window.addEventListener('hashchange', () => this.scrollToHash());
    // a link to the current hash doesn't change it, but its target may be scrolled out of the rendered files
    document.addEventListener('click', (e) => {
      if ((e.target as Element).closest('a[href^="#"]')?.getAttribute('href') === window.location.hash) this.scrollToHash();
    });
    const observer = new ResizeObserver(() => this.updateScrollMargin());
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
    this.virtualizer ??= this.createVirtualizer(); // the stream may have failed meanwhile and created it
    this.scrollToHash();
    this.queueMount();
  }

  addEntry(file: DiffStreamFile, missing: boolean): DiffFileEntry {
    const entry: DiffFileEntry = {
      file, missing,
      hunks: parseHunks(file.hunks ?? ''),
      folded: Boolean(diffTreeStore().fullNameMap[file.name]?.IsViewed || file.isGenerated),
      element: null, highlighter: undefined, renderedLines: null, linePairs: null, rowIndex: null, rowCount: null, gaps: null, content: null, contentPromise: null, contentSegments: null, lfsState: null,
    };
    this.entries.push(entry);
    this.entriesByName.set(file.name, entry);
    const treeItem = diffTreeStore().fullNameMap[file.name];
    if (treeItem && !missing) {
      treeItem.Additions = treeItem.Deletions = 0;
      for (const hunk of entry.hunks) {
        for (const line of hunk.lines) {
          if (line.type === 'add') treeItem.Additions++;
          else if (line.type === 'del') treeItem.Deletions++;
        }
      }
    }
    if (entry.hunks.length) this.loadEntryHighlighter(entry);
    return entry;
  }

  findEntry(path: string): DiffFileEntry | undefined {
    return this.entriesByName.get(path) ?? this.entries.find((entry) => entry.file.oldName === path);
  }

  // files without changes left after ignoring whitespace aren't streamed, neither are the ones beyond the size limit
  addMissingEntries(): void {
    for (const item of diffTreeFiles(diffTreeStore())) {
      if (!this.entriesByName.has(item.FullName)) this.addEntry(treeEntryToStreamFile(item), true);
    }
  }

  async loadEntryHighlighter(entry: DiffFileEntry): Promise<void> {
    if (this.config.disableHighlight) {
      entry.highlighter = null;
      return;
    }
    entry.highlighter = await loadHighlighter(entry.file.name, entry.file.language);
    if (entry.highlighter && entry.element && entry.renderedLines) {
      // the file was rendered before its grammar loaded
      entry.renderedLines = null;
      entry.contentSegments = null;
      this.renderBody(entry);
    }
  }

  // the content of every line in hunk order, highlighted and with the changed words of paired lines marked
  renderLines(entry: DiffFileEntry): NonNullable<DiffFileEntry['renderedLines']> {
    if (entry.renderedLines) return entry.renderedLines;
    const lines = entry.hunks.flatMap((hunk) => hunk.lines);
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
      html: linesToHtml(lines, segments, this.linePairs(entry), escapes),
      escapeButtons: escapes.map((escape) => escapeButton(escape, this.locale)),
    };
    return entry.renderedLines;
  }

  resetRendering(entry: DiffFileEntry): void {
    entry.renderedLines = null;
    entry.linePairs = null;
    entry.rowIndex = null;
    entry.rowCount = null;
    entry.gaps = null;
  }

  // the files of the commits can be loaded for text diffs when the repo has file pages, the wiki doesn't
  canLoadContent(entry: DiffFileEntry): boolean {
    const {file} = entry;
    return Boolean(this.config.repoLink) && !file.isBinary && !file.lfs && !file.tooLarge && !entry.missing && entry.hunks.length > 0;
  }

  isExpandable(entry: DiffFileEntry): boolean {
    return this.canLoadContent(entry) && entry.file.type !== 'added' && entry.file.type !== 'deleted';
  }

  gaps(entry: DiffFileEntry): Gap[] {
    entry.gaps ??= this.isExpandable(entry) ? gapsOf(entry.hunks, entry.content?.new.length ?? null) : [];
    return entry.gaps;
  }

  // the gap a line is hidden in, comments on hidden lines show up when their gap gets expanded
  hiddenGap(entry: DiffFileEntry, side: DiffSide, line: number): Gap | null {
    return gapOfLine(this.gaps(entry), side, line);
  }

  // the expand buttons of the gaps, with the number of comments hidden in them
  expanderButtons(entry: DiffFileEntry): Map<number, string> {
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
        <button type="button" class="code-expander-button" data-expand="${direction}">${htmlRaw(svg(icons[direction]))}</button>
      `).join(''))}</div>`);
    }
    return buttons;
  }

  // like the legacy viewer, small files are highlighted as a whole so that multi-line constructs around the hunks are known
  async highlightWithContent(entry: DiffFileEntry): Promise<void> {
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

  loadContent(entry: DiffFileEntry): Promise<NonNullable<DiffFileEntry['content']>> {
    entry.contentPromise ??= this.fetchContent(entry);
    return entry.contentPromise;
  }

  async fetchContent(entry: DiffFileEntry): Promise<NonNullable<DiffFileEntry['content']>> {
    const {file} = entry;
    const fetchLines = async (url: string) => {
      const response = await fetchOk(url);
      if (this.config.maxFileSize >= 0 && Number(response.headers.get('content-length')) > this.config.maxFileSize) {
        await response.body?.cancel();
        throw new Error(this.locale.fileSuppressed);
      }
      return splitLines(await response.text());
    };
    let oldLines: string[] | null;
    let newLines: string[];
    try {
      [oldLines, newLines] = await Promise.all([
        this.config.beforeCommitId && file.type !== 'added' ? fetchLines(fileUrl(this.config, 'raw', 'old', file.oldName)) : null,
        file.type === 'deleted' ? [] : fetchLines(fileUrl(this.config, 'raw', 'new', file.name)),
      ]);
    } catch (error) {
      entry.contentPromise = null; // a later expansion tries again
      throw error;
    }
    entry.content = {old: oldLines, new: newLines};
    entry.contentSegments = null;
    entry.gaps = null; // the tail gap ends at the known line count now
    return entry.content;
  }

  async expand(entry: DiffFileEntry, gapIndex: number, direction: ExpandDirection): Promise<void> {
    const content = await this.loadContent(entry);
    const gap = this.gaps(entry).find((candidate) => candidate.index === gapIndex);
    if (gap) expandGap(entry.hunks, gap, direction, content.new);
    this.resetRendering(entry);
    if (entry.element) this.renderBody(entry);
  }

  lineEscapes(text: string): LineEscapes | null {
    if (!this.escapeTables) return null; // disabled, or the diff failed to load before they did
    return detectEscapes(text, this.escapeTables, (char, confusable) => trString(this.locale.ambiguousCharacter, char, confusable));
  }

  async loadEscapeTables(): Promise<void> {
    this.escapeTables = await loadEscapeTables(document.documentElement.lang);
  }

  linePairs(entry: DiffFileEntry): Int32Array {
    entry.linePairs ??= pairChangedLines(entry.hunks);
    return entry.linePairs;
  }

  rowIndex(entry: DiffFileEntry, side: DiffSide, line: number): number {
    if (!entry.rowIndex) {
      entry.rowIndex = {old: new Map(), new: new Map()};
      let row = 0;
      for (const hunk of entry.hunks) {
        row++;
        for (const diffLine of hunk.lines) {
          if (diffLine.oldNum) entry.rowIndex.old.set(diffLine.oldNum, row);
          if (diffLine.newNum) entry.rowIndex.new.set(diffLine.newNum, row);
          row++;
        }
      }
    }
    return entry.rowIndex[side].get(line) ?? -1;
  }

  estimateHeight(entry: DiffFileEntry): number {
    if (entry.folded) return headerHeight;
    if (this.bodyNotice(entry) !== null) return headerHeight + noticeHeight;
    return headerHeight + this.rowCount(entry) * lineHeight;
  }

  // the virtualizer estimates all unmeasured files again whenever a file arrives, so the count is cached
  rowCount(entry: DiffFileEntry): number {
    if (entry.rowCount === null) {
      let rows = hunksLineCount(entry.hunks);
      if (this.config.isSplit) {
        for (const pair of this.linePairs(entry)) if (pair !== -1) rows -= 0.5;
      }
      entry.rowCount = rows;
    }
    return entry.rowCount;
  }

  // returns the notice shown instead of the lines, or null if the lines are shown
  bodyNotice(entry: DiffFileEntry): string | null {
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

  renderBody(entry: DiffFileEntry): void {
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

  buildSource(entry: DiffFileEntry): HTMLElement {
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
        `${splitColgroup}<tbody>${renderSplitRows(hunks, lineHtml, this.linePairs(entry), opts)}</tbody>` :
        `${unifiedColgroup}<tbody>${renderUnifiedRows(hunks, lineHtml, opts)}</tbody>`;
      source.classList.add('lezer-highlight');
      if (entry.highlighter?.language) source.setAttribute('data-language', entry.highlighter.language);
      source.innerHTML = `<table class="chroma">${rows}</table>`;
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
  lfsNotice(entry: DiffFileEntry): string | null {
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
  async loadLfs(entry: DiffFileEntry): Promise<void> {
    entry.lfsState = 'loading';
    const {lfs, oldName, name} = entry.file;
    const urls = [lfs!.old?.exists ? fileUrl(this.config, 'media', 'old', oldName) : null, lfs!.new?.exists ? fileUrl(this.config, 'media', 'new', name) : null];
    const read = async (url: string | null, headers?: HeadersInit) => (url ? new Uint8Array(await (await fetchOk(url, headers)).arrayBuffer()) : new Uint8Array());
    try {
      // like git, content with a NUL byte in its first 8000 bytes is binary, only text is loaded completely
      const starts = await Promise.all(urls.map((url) => read(url, {Range: 'bytes=0-7999'})));
      if (starts.some((data) => data.subarray(0, 8000).includes(0))) {
        entry.lfsState = 'binary';
      } else {
        const [oldData, newData] = await Promise.all(urls.map(async (url, idx) => (starts[idx].length < 8000 ? starts[idx] : read(url))));
        const decoder = new TextDecoder();
        entry.hunks = buildHunks(splitLines(decoder.decode(oldData)), splitLines(decoder.decode(newData)));
        this.resetRendering(entry);
        entry.lfsState = 'diff';
      }
    } catch (error) {
      console.error(error);
      entry.lfsState = 'failed';
    }
    if (entry.element) this.renderBody(entry);
  }

  async loadFullFile(entry: DiffFileEntry, button: HTMLElement): Promise<void> {
    button.classList.add('is-loading');
    const url = new URL(this.config.streamUrl, window.location.href);
    url.searchParams.append('files', entry.file.name);
    if (entry.file.oldName !== entry.file.name) url.searchParams.append('files', entry.file.oldName);
    url.searchParams.set('full', 'true');
    const files: DiffStreamFile[] = [];
    try {
      await streamDiff(url.href, (file) => { files.push(file) });
    } catch (error) {
      showErrorToast(errorMessage(error));
      return;
    } finally {
      button.classList.remove('is-loading');
    }
    const loaded = files.find((file) => file.name === entry.file.name);
    // a file beyond the total size limit can't be loaded
    if (!loaded || loaded.tooLarge) {
      button.remove();
      showErrorToast(this.locale.fileSuppressed);
      return;
    }
    entry.file = loaded;
    entry.missing = false;
    entry.hunks = parseHunks(entry.file.hunks ?? '');
    this.resetRendering(entry);
    if (entry.highlighter === undefined) await this.loadEntryHighlighter(entry);
    this.rebuildElement(entry);
  }

  // the header shows the file's data too, so the element is built again when it changes
  rebuildElement(entry: DiffFileEntry): void {
    entry.element?.remove();
    this.mounted.delete(entry);
    entry.element = null;
    this.queueMount();
  }

  buildFileElement(entry: DiffFileEntry): HTMLElement {
    const box = buildFileBox(entry, this.config, this.locale, renderedKind(entry, this.config) !== null);
    entry.element = box;
    this.entriesByElement.set(box, entry);
    applyFolded(entry, entry.folded);
    box.querySelector('.fold-file')!.addEventListener('click', () => this.setFolded(entry, !entry.folded));
    box.querySelector<HTMLInputElement>('.viewed-file-form input')?.addEventListener('change', (e) => {
      this.setViewed(entry, (e.target as HTMLInputElement).checked);
    });
    this.renderBody(entry);
    return box;
  }

  setFolded(entry: DiffFileEntry, folded: boolean): void {
    applyFolded(entry, folded);
    if (folded && entry.element && entry.element.getBoundingClientRect().top < 0) {
      const index = this.visibleEntries.indexOf(entry);
      if (index >= 0) this.virtualizer?.scrollToIndex(index, {align: 'start'});
    }
  }

  setAllFolded(folded: boolean): void {
    for (const entry of this.entries) applyFolded(entry, folded);
    this.virtualizer?.measure();
  }

  async setViewed(entry: DiffFileEntry, viewed: boolean): Promise<void> {
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
    this.virtualizer?.scrollToIndex(index, {align: 'start'});
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
    const entry = this.entriesByElement.get(button.closest<HTMLElement>('.diff-file-box')!)!;
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
    this.comments.openDraft({path: this.entriesByElement.get(box)!.file.name, side, line: Number(row.getAttribute(`data-${side}`))});
  }

  createVirtualizer(): Virtualizer<Window, HTMLElement> {
    this.root.append(this.list);
    const instance = new Virtualizer<Window, HTMLElement>({
      count: this.visibleEntries.length,
      getScrollElement: () => window,
      estimateSize: (index) => this.estimateHeight(this.visibleEntries[index]),
      getItemKey: (index) => this.visibleEntries[index].file.name,
      observeElementRect: observeWindowRect,
      observeElementOffset: observeWindowOffset,
      scrollToFn: windowScroll,
      scrollMargin: this.list.getBoundingClientRect().top + window.scrollY,
      scrollPaddingStart: stickyTopHeight,
      // the synchronous measurement on mount would force a layout per file, the ResizeObserver delivers the size after the regular one
      measureElement: (el, entry, instance) => {
        if (entry) return measureElement(el, entry, instance);
        return instance.itemSizeCache.get(instance.options.getItemKey(instance.indexFromElement(el))) ?? this.estimateHeight(this.entriesByElement.get(el)!);
      },
      overscan: 3,
      gap: 8,
      onChange: () => this.queueMount(),
    });
    instance._didMount();
    instance._willUpdate();
    return instance;
  }

  updateScrollMargin(): void {
    if (!this.virtualizer) return;
    this.virtualizer.setOptions({...this.virtualizer.options, scrollMargin: this.list.getBoundingClientRect().top + window.scrollY});
    this.queueMount();
  }

  queueMount(): void {
    if (this.mountQueued) return;
    this.mountQueued = true;
    queueMicrotask(() => {
      this.mountQueued = false;
      this.mountEntries();
    });
  }

  mountEntries(): void {
    const virtualizer = this.virtualizer;
    if (!virtualizer) return;
    const items = virtualizer.getVirtualItems();
    this.list.style.height = `${virtualizer.getTotalSize()}px`;
    const wanted = new Set(items.map((item) => this.visibleEntries[item.index]));
    for (const entry of this.mounted) {
      if (!wanted.has(entry)) {
        entry.element!.remove();
        this.mounted.delete(entry);
      }
    }
    for (const item of items) {
      const entry = this.visibleEntries[item.index];
      const el = entry.element ?? this.buildFileElement(entry);
      el.setAttribute('data-index', String(item.index));
      el.style.top = `${item.start - virtualizer.options.scrollMargin}px`;
      if (!this.mounted.has(entry)) {
        this.list.append(el);
        this.mounted.add(entry);
        virtualizer.measureElement(el);
      }
    }
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
    this.visibleEntries = names ? this.entries.filter((entry) => names.has(entry.file.name)) : this.entries;
    toggleElem('#diff-no-matches', this.entries.length > 0 && this.visibleEntries.length === 0);
    if (!this.virtualizer) return;
    // item sizes are cached by key, so a filter change keeps the measured heights
    this.virtualizer.setOptions({...this.virtualizer.options, count: this.visibleEntries.length});
    this.queueMount();
    if (this.hashPending) this.scrollToHash();
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
    this.virtualizer?.scrollToIndex(index, {align: 'start'});
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
