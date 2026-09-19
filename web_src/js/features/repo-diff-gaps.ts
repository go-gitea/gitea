import {effectScope, reactive, watchEffect} from 'vue';
import {svg, type SvgName} from '../svg.ts';
import {html, htmlRaw} from '../utils/html.ts';
import {toggleElem} from '../utils/dom.ts';

// The reactivity below is Vue's, but the rendering is not. Diff rows are built by the Go templates,
// with syntax highlighting, unicode escaping and comment threads the frontend holds no data for, so
// this places server-rendered rows by hand rather than declaring them in a component. Once diff rows
// are rendered from data on the client, placeGapRows and renderGapExpander become that component's
// template and everything here except the gap arithmetic can go.
//
// A "gap" is a stretch of unchanged lines the diff hides behind a section row. The backend renders
// that row with the gap's own line numbers, and everything below works from them, so revealing lines
// never needs the server to say what is left.
//
// Must match gitdiff.BlobExcerptChunkSize.
export const blobExcerptChunkSize = 20;

export type DiffGap = {
  key: string; // identifies the gap for the life of the page, even as it shrinks
  anchor: string;
  lastLeft: number; lastRight: number; // the last line rendered before the gap
  left: number; right: number; // the next line rendered after it, or the file's last line at its end
  leftHunk: number; rightHunk: number; // zero on both sides means the gap runs to the end of the file
  hiddenCommentIds: string[]; // comments the diff does not show while this gap is closed
};

// "original" is the gap as the diff first described it, "current" what is left of it, and "rows"
// what it has put on screen. One effect per gap renders all three, so the arrows, the hidden comment
// count and the revealed lines cannot disagree about how much of the gap is open.
type DiffGapState = {original: DiffGap; current: DiffGap; rows: HTMLElement[]};

const diffGaps = reactive(new Map<string, DiffGapState>());

function gapStoreKey(el: Element, gapKey: string): string {
  return `${el.closest('.diff-file-box')?.id ?? ''}:${gapKey}`;
}

export function parseDiffGap(el: Element): DiffGap {
  const [lastLeft, lastRight, left, right, leftHunk, rightHunk] = el.getAttribute('data-gap')!.split(',').map(Number);
  const hiddenCommentIds = el.getAttribute('data-hidden-comment-ids')!.split(',').filter(Boolean);
  return {key: el.getAttribute('data-gap-key')!, anchor: el.getAttribute('data-gap-anchor')!, lastLeft, lastRight, left, right, leftHunk, rightHunk, hiddenCommentIds};
}

export function getDiffGapState(el: Element, gapKey: string): DiffGapState | undefined {
  return diffGaps.get(gapStoreKey(el, gapKey));
}

function updateDiffGap(el: Element, gapKey: string, changes: Partial<DiffGapState>) {
  const key = gapStoreKey(el, gapKey);
  const state = diffGaps.get(key);
  if (state) diffGaps.set(key, {...state, ...changes});
}

// runs to the end of the file, so nothing follows the gap and "right" is its last line
export function gapReachesFileEnd(gap: DiffGap): boolean {
  return gap.leftHunk <= 0 && gap.rightHunk <= 0;
}

// which arrows a gap deserves, mirroring gitdiff.GetExpandDirection
export function gapExpandDirection(gap: DiffGap): '' | 'up' | 'down' | 'updown' | 'single' {
  if (gap.left - gap.lastLeft <= 1 || gap.right - gap.lastRight <= 1) return '';
  if (gap.lastLeft <= 0 && gap.lastRight <= 0) return 'up';
  if (gap.right - gap.lastRight - 1 > blobExcerptChunkSize && gap.rightHunk > 0) return 'updown';
  if (gapReachesFileEnd(gap)) return 'down';
  return 'single';
}

// what a gap has left once "direction" has revealed one chunk, mirroring the chunking in
// gitdiff.BuildBlobExcerptDiffSection
export function gapAfterExpanding(gap: DiffGap, direction: string): DiffGap {
  const remaining = gap.right - gap.lastRight;
  if (direction === 'up' && remaining > blobExcerptChunkSize) {
    return {...gap, left: gap.left - blobExcerptChunkSize, right: gap.right - blobExcerptChunkSize, leftHunk: gap.leftHunk + blobExcerptChunkSize, rightHunk: gap.rightHunk + blobExcerptChunkSize};
  }
  if (direction === 'down' && remaining > blobExcerptChunkSize) {
    return {...gap, lastLeft: gap.lastLeft + blobExcerptChunkSize, lastRight: gap.lastRight + blobExcerptChunkSize};
  }
  return {...gap, left: 0, right: 0, leftHunk: 0, rightHunk: 0}; // fully revealed
}

export function gapExcerptUrl(baseUrl: string, gap: DiffGap, direction: string): string {
  const url = new URL(baseUrl, window.location.href);
  url.searchParams.set('direction', direction);
  url.searchParams.set('anchor', gap.anchor);
  url.searchParams.set('gap_key', gap.key);
  url.searchParams.set('last_left', String(gap.lastLeft));
  url.searchParams.set('last_right', String(gap.lastRight));
  url.searchParams.set('left', String(gap.left));
  url.searchParams.set('right', String(gap.right));
  url.searchParams.set('left_hunk_size', String(gap.leftHunk));
  url.searchParams.set('right_hunk_size', String(gap.rightHunk));
  return url.href;
}

// one request reveals every gap named here, so a partly expanded file does not re-fetch what it shows
export function gapsExcerptUrl(baseUrl: string, gaps: DiffGap[]): string {
  const url = new URL(baseUrl, window.location.href);
  for (const gap of gaps) {
    url.searchParams.append('gap', [gap.lastLeft, gap.lastRight, gap.left, gap.right, gap.leftHunk, gap.rightHunk].join(','));
  }
  return url.href;
}

// An excerpt response is a fragment of rows that opens with the table's "colgroup", which makes the
// parser tuck them into a "tbody", so take the rows themselves rather than whatever wrapped them.
export function parseTableRows(respText: string): HTMLElement[] {
  const elTemplate = document.createElement('template'); // a template can hold "tr" elements without a table around them
  elTemplate.innerHTML = respText;
  return Array.from(elTemplate.content.querySelectorAll('tr'));
}

// the gaps of a file that still have something to reveal, as the diff first described them
export function pendingDiffGaps(elFileBody: Element): DiffGap[] {
  const gaps = [];
  for (const el of elFileBody.querySelectorAll('.code-expander-buttons[data-gap-key]')) {
    const state = getDiffGapState(el, el.getAttribute('data-gap-key')!);
    if (state && gapExpandDirection(state.current)) gaps.push(state.original);
  }
  return gaps;
}

export function diffFileHasHiddenLines(elFileBody: Element): boolean {
  return pendingDiffGaps(elFileBody).length > 0;
}

// the rows a gap has revealed and what it has left, the only two things an expansion changes
export function revealGapLines(el: Element, gapKey: string, rows: HTMLElement[], current: DiffGap) {
  const state = getDiffGapState(el, gapKey);
  if (state) updateDiffGap(el, gapKey, {current, rows: [...state.rows, ...rows]});
}

export function closeGap(el: Element, gapKey: string) {
  const state = getDiffGapState(el, gapKey);
  if (state) updateDiffGap(el, gapKey, {current: state.original, rows: []});
}

const arrowSvgs: Record<string, SvgName> = {up: 'octicon-fold-up', down: 'octicon-fold-down', single: 'octicon-fold'};

function renderGapExpander(elExpander: Element, gap: DiffGap) {
  const direction = gapExpandDirection(gap);
  elExpander.setAttribute('data-expand-direction', direction);
  const stillHidden = gap.hiddenCommentIds.filter((id) => !document.querySelector(`#issuecomment-${CSS.escape(id)}`));
  const parts = [];
  if (stillHidden.length) {
    parts.push(html`<span class="code-comment-more" data-tooltip-content="${`${stillHidden.length} hidden comment(s)`}">${stillHidden.length}</span>`);
  }
  for (const d of direction === 'updown' ? ['down', 'up'] : (direction ? [direction] : [])) {
    parts.push(html`<button class="code-expander-button" data-global-click="diffExpandHiddenLines" data-gap-direction="${d}">${htmlRaw(svg(arrowSvgs[d]))}</button>`);
  }
  elExpander.setAttribute('data-hidden-comment-ids', `,${stillHidden.join(',')},`);
  elExpander.innerHTML = parts.join('');
}

// a revealed line keeps its conversations in the row that follows it, so they travel together
function conversationsOf(elRow: HTMLElement): HTMLElement | null {
  const el = elRow.nextElementSibling;
  return el?.matches('tr.add-comment') ? el as HTMLElement : null;
}

function placeGapRows(elSectionRow: HTMLElement, state: DiffGapState) {
  const wanted = new Set(state.rows);
  for (const el of elSectionRow.parentElement!.querySelectorAll<HTMLElement>(`tr[data-expand-gap="${CSS.escape(state.original.key)}"]`)) {
    if (wanted.has(el)) continue;
    conversationsOf(el)?.remove();
    el.remove();
  }
  if (!state.rows.length) return;
  let elAt: HTMLElement = elSectionRow;
  for (const elRow of state.rows) {
    elAt.after(elRow);
    elAt = conversationsOf(elRow) ?? elRow;
  }
  // the still hidden part of the gap sits on whichever side the revealed lines were taken from
  if (state.current.lastRight > state.original.lastRight) elAt.after(elSectionRow);
}

// one effect per gap: the arrows, the hidden comment count and the revealed lines all come from the
// same state, so they cannot disagree
export function initDiffGapExpander(el: HTMLElement) {
  const gap = parseDiffGap(el);
  const key = gapStoreKey(el, gap.key);
  if (!diffGaps.has(key)) diffGaps.set(key, {original: gap, current: gap, rows: []});
  const scope = effectScope();
  scope.run(() => watchEffect(() => {
    if (!el.isConnected) return scope.stop(); // the file box was replaced, nothing left to render into
    const state = diffGaps.get(key);
    if (!state) return;
    renderGapExpander(el, state.current);
    const elSectionRow = el.closest<HTMLElement>('tr')!;
    placeGapRows(elSectionRow, state);
    toggleElem(elSectionRow, Boolean(gapExpandDirection(state.current)));
  }));
}
